package slack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/cenkalti/backoff"
	"github.com/zerobotlabs/relax/redisclient"
	"github.com/zerobotlabs/relax/utils"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/gorilla/websocket"
	"github.com/zerobotlabs/relax/Godeps/_workspace/src/gopkg.in/redis.v3"
)

// This data structure holds all clients that are connected to Slack's RealTime API
// keyed by Team ID
var Clients = map[string]*Client{}

func init() {
	utils.SetupLogging()
}

// NewClient initializes a new client that connects to Slack's RealTime API
// The client is initialized by JSON that looks like this:
// {"token":"xoxo_token","team_id":"team_id","provider":"slack"}
// where token is the token used by the bot, team_id is the ID for the team
// and provider is "slack"
func NewClient(initJSON string) (*Client, error) {
	var err error
	var c Client

	if err = json.Unmarshal([]byte(initJSON), &c); err == nil {
	} else {
		return &c, err
	}

	c.heartBeatsMutex = &sync.Mutex{}
	return &c, nil
}

// InitClients initializes all clients which are present in Redis on boot.
// This looks for all hashes in the key RELAX_BOTS_KEY and calls NewClient for each
// hash present in the redis key. It also starts a loop to listen to REDIS_BOTS_PUBSUB
// when new clients need to be started. It listens to Redis on pubsub instead of a queue
// because there can be multiple instances of Relax running and they all need to start
// a Slack client.
func InitClients() {
	redisClient := redisclient.Client()

	resultCmd := redisClient.HGetAll(os.Getenv("RELAX_BOTS_KEY"))
	result := resultCmd.Val()

	for i := 0; i < len(result); i += 2 {
		teamId := result[i]
		val := result[i+1]

		c, err := NewClient(val)

		if err != nil {
			log.WithFields(log.Fields{
				"team":  teamId,
				"error": err,
			}).Error("starting slack client")
		} else {
			go c.LoginAndStart()
		}
	}

	go startReadFromRedisPubSubLoop()
}

func (c *Client) ResetHeartBeatsMissed() {
	c.heartBeatsMutex.Lock()
	c.heartBeatsMissed = 0
	c.heartBeatsMutex.Unlock()
}

func (c *Client) IncrementHeartBeatsMissed() {
	c.heartBeatsMutex.Lock()
	c.heartBeatsMissed = c.heartBeatsMissed + 1
	c.heartBeatsMutex.Unlock()
}

// Login calls the "rtm.start" Slack API and gets a bunch of information such as
// the websocket URL to connect to, users and channel information for the team and so on
func (c *Client) Login() error {
	contents, err := c.callSlack("rtm.start", map[string][]string{}, 200)
	var metadata Metadata

	if err != nil {
		log.WithFields(log.Fields{
			"team":  c.TeamId,
			"error": err,
		}).Error("calling rtm.start")

		return err
	} else {
		if err = json.Unmarshal([]byte(contents), &metadata); err == nil {
			metadata.Channels = map[string]Channel{}
			metadata.Users = map[string]User{}

			for _, im := range metadata.ImsList {
				metadata.Channels[im.Id] = Channel{Id: im.Id, Created: im.Created, CreatorId: im.CreatorId, Name: "direct", Im: true}
			}
			for _, c := range metadata.ChannelsList {
				c.Im = false
				metadata.Channels[c.Id] = c
			}
			for _, g := range metadata.GroupsList {
				g.Im = false
				metadata.Channels[g.Id] = g
			}
			for _, u := range metadata.UsersList {
				metadata.Users[u.Id] = u
			}

			c.data = &metadata
			return nil
		} else {
			log.WithFields(log.Fields{
				"team":  c.TeamId,
				"error": err,
			}).Error("parsing JSON response from rtm.start")

			return err
		}
	}
}

// Start starts a websocket connection to Slack's servers and starts listening for messages
// If it detects an "invalid_auth" or "inactive_account" message, it means that the token provided by the user
// has expired or is incorrect and so it sends a "disable_bot" event back to the user
// so that they can take remedial action.
func (c *Client) Start() error {
	// Make connection to redis now
	c.redisClient = redisclient.Client()

	if c.data.Ok == true {
		conn, _, err := websocket.DefaultDialer.Dial(c.data.Url, http.Header{})
		if err != nil {
			log.WithFields(log.Fields{
				"team":  c.TeamId,
				"error": err,
			}).Error("dialing connection to Slack websocket")

			return err
		} else {
			c.conn = conn
		}

		c.pingTicker = time.NewTicker(time.Millisecond * 5000)
		go c.startReadFromSlackLoop()
		go c.startPingPump()
	} else {
		// Bot has been disabled by the user,
		// so we need to mark it as disabled
		if c.data.Error == "invalid_auth" ||
			c.data.Error == "account_inactive" {
			var msg Message
			msg.User = User{}
			msg.Channel = Channel{}

			err := c.sendEvent("disable_bot", &msg, "", "", "")
			if err != nil {
				return err
			}
		}

		log.WithFields(log.Fields{
			"team":  c.TeamId,
			"error": c.data.Error,
		}).Error("starting slack client")

		return fmt.Errorf(fmt.Sprintf("error connecting to slack websocket server: %s", c.data.Error))
	}

	// This serves no real purpose other than to let tests know that a certain client has been initialized
	val := c.redisClient.HSet(os.Getenv("RELAX_MUTEX_KEY"), fmt.Sprintf("bot-%s-started", c.TeamId), fmt.Sprintf("%d", time.Now().Nanosecond()))
	if val == nil || val.Val() != true {
		log.WithFields(log.Fields{
			"team":  c.TeamId,
			"error": fmt.Sprintf("could not set bot-%s-started in RELAX_MUTEX_KEY", c.TeamId),
		}).Error("starting slack client")
	}

	Clients[c.TeamId] = c

	return nil
}

// LoginAndStarts is a simple helper function that calls login and start successively
// so that it can be invoked in a goroutine (as is done in InitClients)
func (c *Client) LoginAndStart() error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1500 * time.Millisecond
	err := backoff.Retry(c.Login, b)

	if err == nil {
		err = c.Start()
	} else {
		log.WithFields(log.Fields{
			"team":  c.TeamId,
			"error": err,
		}).Error("logging in for slack client")
	}

	return err
}

func (c *Client) startPingPump() {
	for _ = range c.pingTicker.C {
		m := Message{
			Id:             c.TeamId,
			Type:           "ping",
			EventTimestamp: fmt.Sprintf("%d", time.Now().UnixNano()),
		}

		json, err := json.Marshal(m)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("error marshaling ping message")
		} else {
			c.IncrementHeartBeatsMissed()
			if c.heartBeatsMissed >= 3 {
				c.ResetHeartBeatsMissed()
				c.pingTicker.Stop()
				c.Stop()
				break
			}
			c.conn.WriteMessage(websocket.TextMessage, []byte(json))
		}
	}

	go c.LoginAndStart()
}

// Stop closes the websocket connection to Slack's websocket servers
func (c *Client) Stop() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// callSlack is a utility method that makes HTTP API calls to Slack
func (c *Client) callSlack(method string, params url.Values, expectedStatusCode int) (string, error) {
	params.Set("token", c.Token)
	method = "/api/" + method

	host := os.Getenv("SLACK_HOST")
	if host == "" {
		host = "https://api.slack.com"
	}

	return c.callAPI(host, method, params, expectedStatusCode)
}

// sendEvent is a utility function that wraps event data in an Event struct
// and sends them back to the user via Redis.
func (c *Client) sendEvent(responseType string, msg *Message, text string, timestamp string, eventTimestamp string) error {
	// If the eventTimestamp blank, then set a timestamp to the currentTime (this typically means)
	// that it is the responsibility of the client to make sure that events are handled idempotently
	if eventTimestamp == "" {
		eventTimestamp = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	event := &Event{
		Type:           responseType,
		UserUid:        msg.User.Id,
		ChannelUid:     msg.Channel.Id,
		TeamUid:        c.TeamId,
		Im:             msg.Channel.Im,
		Text:           text,
		RelaxBotUid:    c.data.Self.Id,
		Timestamp:      timestamp,
		EventTimestamp: eventTimestamp,
		Provider:       "slack",
	}

	eventJson, err := json.Marshal(event)

	if err != nil {
		return err
	} else {
		// When relax is run in "high-availabilty" mode, (i.e. multiple instances of
		// Relax are running), we need to make sure that the same event is not sent more than once
		// back to the user. We use Redis to make sure to ensure the "send-only-once" requirement.
		send := func(tx *redis.Multi, event *Event) error {
			shouldSend := true
			key := fmt.Sprintf("bot_message:%s:%s", event.ChannelUid, event.EventTimestamp)
			boolCmd := tx.HSetNX(os.Getenv("RELAX_MUTEX_KEY"), key, "ok")

			log.WithFields(log.Fields{
				"team":      c.TeamId,
				"timestamp": event.EventTimestamp,
				"channel":   event.ChannelUid,
			}).Debug("sending event back to client")

			if boolCmd != nil {
				shouldSend = boolCmd.Val()
			}

			if shouldSend {
				intCmd := tx.RPush(os.Getenv("RELAX_EVENTS_QUEUE"), string(eventJson))
				if intCmd == nil || intCmd.Val() != 1 {
					return fmt.Errorf("Unexpected error while pushing to RELAX_EVENTS_QUEUE")
				}
			} else {
				log.WithFields(log.Fields{
					"team":      c.TeamId,
					"timestamp": event.EventTimestamp,
					"channel":   event.ChannelUid,
				}).Debug("ignoring, not sending event back to client")
			}

			return nil
		}

		tx := c.redisClient.Multi()
		defer tx.Close()
		send(tx, event)
	}

	return nil
}

// startReadFromRedisPubSubLoop is the method invoked by InitClients that listens for new
// clients that need to be started via a Redis Pubsub channel.
func startReadFromRedisPubSubLoop() {
	redisClient := redisclient.Client()

	pubsub := redisClient.PubSub()
	defer pubsub.Close()

	pubsubChannel := os.Getenv("RELAX_BOTS_PUBSUB")
	err := pubsub.Subscribe(pubsubChannel)

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error subscribing to redis pubsub")
		return
	}

	for {
		msgi, err := pubsub.ReceiveTimeout(100 * time.Millisecond)
		if err != nil {
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				continue
			}
		} else {
			switch msg := msgi.(type) {
			case *redis.Message:
				if msg.Channel != pubsubChannel {
					break
				}

				payload := msg.Payload
				var cmd Command

				if err := json.Unmarshal([]byte(payload), &cmd); err != nil {
					break
				}

				switch cmd.Type {
				case "message":
					shouldSend := true

					if cmd.TeamId == "" {
						break
					}
					// if Id is not present, then populate an ID
					if cmd.Id == "" {
						cmd.Id = fmt.Sprintf("%d", time.Now().Nanosecond())
					}
					c := Clients[cmd.TeamId]

					if c != nil && c.conn != nil {
						key := fmt.Sprintf("send_slack_message:%s", cmd.Id)
						boolCmd := redisClient.HSetNX(os.Getenv("RELAX_MUTEX_KEY"), key, "ok")

						if boolCmd != nil {
							shouldSend = boolCmd.Val()
						}

						if shouldSend {
							c.conn.WriteMessage(websocket.TextMessage, []byte(cmd.Payload))
							log.WithFields(log.Fields{
								"team":       cmd.TeamId,
								"command_id": cmd.Id,
							}).Debug("sent message to slack")
						} else {
							log.WithFields(log.Fields{
								"team":       cmd.TeamId,
								"command_id": cmd.Id,
							}).Debug("ignoring, not sending message to slack")
						}
					}

				case "team_added":
					if cmd.TeamId == "" {
						break
					}
					result := redisClient.HGet(os.Getenv("RELAX_BOTS_KEY"), cmd.TeamId)
					if result == nil {
						break
					}
					val := result.Val()
					c := Clients[cmd.TeamId]

					if c != nil {
						err := c.Stop()
						if err != nil {
							log.WithFields(log.Fields{
								"team":  cmd.TeamId,
								"error": err,
							}).Error("closing websocket connection")
						}
					}

					c, err := NewClient(val)
					if err == nil {
						c.LoginAndStart()
					} else {
						log.WithFields(log.Fields{
							"team":  cmd.TeamId,
							"error": err,
						}).Error("starting client")
					}
				}
			}
		}
	}
}

// startReadFromSlackLoop is the method invoked by Start() that listens to Slack's
// websocket connection and handles messages accordingly.
func (c *Client) startReadFromSlackLoop() {
	for {
		messageType, msg, err := c.conn.ReadMessage()
		if err == nil {
			if messageType == websocket.TextMessage {
				var message Message
				if err = json.Unmarshal(msg, &message); err == nil {
					c.handleMessage(&message)
				} else {
					log.WithFields(log.Fields{
						"team":  c.TeamId,
						"error": err,
					}).Error("recognizing message from Slack")
				}
			}
		} else {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				log.WithFields(log.Fields{
					"team":  c.TeamId,
					"error": err,
				}).Error("error reading message from slack websocket connection, timeout")
			} else {
				// Reconnect the client
				log.WithFields(log.Fields{
					"team":  c.TeamId,
					"error": err,
				}).Error("error reading message from slack websocket connection")

				return
			}
		}
	}
}

// handleMessage is a utility method that handles the different Slack events that
// are generated by the RealTime API. For each event that is handled here, a response
// event is sent back to the user via a redis queue
func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {

	case "pong":
		if msg.ReplyTo == c.TeamId {
			c.ResetHeartBeatsMissed()
		}
	case "message":
		userId := msg.UserId()
		channelId := msg.ChannelId()

		switch msg.Subtype {
		case "message_deleted":
			msg.User = c.data.Users[userId]
			msg.Channel = c.data.Channels[channelId]

			c.sendEvent("message_deleted", msg, msg.Text, msg.DeletedTimestamp, msg.Timestamp)

		case "message_changed":
			embeddedMessage := msg.EmbeddedMessage()

			if embeddedMessage != nil {
				userId = embeddedMessage.UserId()
				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]
				c.sendEvent("message_edited", msg, embeddedMessage.Text, embeddedMessage.Timestamp, msg.Timestamp)
			}

		// simple message
		case "":
			// Ignore Messages sent from the bot itself
			if userId != c.data.Self.Id {
				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]

				log.WithFields(log.Fields{
					"userId":      userId,
					"channelId":   channelId,
					"msg.User":    msg.User,
					"msg.Channel": msg.Channel,
				}).Info("Sending message_new for this user and channel")

				c.sendEvent("message_new", msg, msg.Text, msg.Timestamp, msg.Timestamp)
			}
		}

	case "reaction_added":
		embeddedItem := msg.EmbeddedItem()
		if embeddedItem != nil {
			channelId := embeddedItem.ChannelId()
			userId := msg.UserId()

			msg.User = c.data.Users[userId]
			msg.Channel = c.data.Channels[channelId]

			c.sendEvent("reaction_added", msg, msg.Reaction, embeddedItem.Timestamp, msg.EventTimestamp)
		}

	case "reaction_removed":
		embeddedItem := msg.EmbeddedItem()
		if embeddedItem != nil {
			channelId := embeddedItem.ChannelId()
			userId := msg.UserId()

			msg.User = c.data.Users[userId]
			msg.Channel = c.data.Channels[channelId]

			c.sendEvent("reaction_removed", msg, msg.Reaction, embeddedItem.Timestamp, msg.EventTimestamp)
		}

	case "team_join":
		if err := json.Unmarshal(msg.RawUser, &msg.User); err == nil {
			c.data.Users[msg.User.Id] = msg.User
			c.sendEvent("team_joined", msg, "", "", "")
		}

	case "im_created":
		if err := json.Unmarshal(msg.RawChannel, &msg.Channel); err == nil {
			msg.Channel.Im = true
			c.data.Channels[msg.Channel.Id] = msg.Channel
			msg.User = c.data.Users[msg.UserId()]

			c.sendEvent("im_created", msg, "", "", "")
		}

	case "group_joined":
		fallthrough
	case "channel_joined":
		var channel Channel

		err := json.Unmarshal(msg.RawChannel, &channel)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("error parsing channel from channel_joined")
		} else {
			channel.Im = false
			c.data.Channels[channel.Id] = channel
			msg.Channel = channel
			// Don't send channel joined messages for upto a minute
			timestamp := fmt.Sprintf("channel-joined-%d-%s", (time.Now().Unix()/60)*60, channel.Id)
			c.sendEvent("channel_joined", msg, "", timestamp, timestamp)
		}
	}
}

// callAPI is a utility method that is invoked by callSlack and is used to make
// HTTP calls to REST API endpoints
func (c *Client) callAPI(h string, method string, params url.Values, expectedStatusCode int) (string, error) {
	u, err := url.ParseRequestURI(h)
	if err != nil {
		return "", err
	}

	u.Path = method
	urlStr := fmt.Sprintf("%v", u)

	client := &http.Client{}
	r, _ := http.NewRequest("POST", urlStr, strings.NewReader(params.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(params.Encode())))

	resp, err := client.Do(r)

	if err != nil {
		return "", err
	} else {
		defer resp.Body.Close()
		contents, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return "", err
		} else {
			if resp.StatusCode != expectedStatusCode {
				return "", fmt.Errorf("Expected Status Code: %d, Got: %d\n", expectedStatusCode, resp.StatusCode)
			} else {
				return string(contents), err
			}
		}
	}
}
