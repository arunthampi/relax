package slack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/gorilla/websocket"
	"github.com/zerobotlabs/relax/Godeps/_workspace/src/gopkg.in/redis.v3"
)

type Channel struct {
	Id        string `json:"id"`
	Created   int64  `json:"created"`
	Name      string `json:"name"`
	CreatorId string `json:"creator"`

	Im bool
}

type Response struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type Im struct {
	Id        string `json:"id"`
	Created   int64  `json:"created"`
	CreatorId string `json:"user"`
}

type Message struct {
	Type             string `json:"type"`
	Subtype          string `json:"subtype"`
	Text             string `json:"text"`
	Timestamp        string `json:"ts"`
	DeletedTimestamp string `json:"deleted_ts"`
	Reaction         string `json:"reaction"`
	Hidden           bool   `json:"hidden"`

	User    User
	Channel Channel

	RawUser    json.RawMessage `json:"user"`
	RawChannel json.RawMessage `json:"channel"`
	RawMessage json.RawMessage `json:"message"`
	RawItem    json.RawMessage `json:"item"`
}

func (m *Message) UserId() string {
	userId := ""

	userBytes, err := m.RawUser.MarshalJSON()
	if err == nil {
		// since we know it's a string, just remove the extra quotes
		userId = strings.Trim(string(userBytes), "\"")
	}

	return userId
}

func (m *Message) ChannelId() string {
	channelId := ""

	channelBytes, err := m.RawChannel.MarshalJSON()
	if err == nil {
		// since we know it's a string, just remove the extra quotes
		channelId = strings.Trim(string(channelBytes), "\"")
	}

	return channelId
}

func (m *Message) EmbeddedMessage() *Message {
	messageBytes, err := m.RawMessage.MarshalJSON()

	if err == nil {
		var embeddedMessage Message
		err = json.Unmarshal(messageBytes, &embeddedMessage)
		if err == nil {
			return &embeddedMessage
		}
	}

	return nil
}

func (m *Message) EmbeddedItem() *Message {
	messageBytes, err := m.RawItem.MarshalJSON()

	if err == nil {
		var embeddedItem Message
		err = json.Unmarshal(messageBytes, &embeddedItem)
		if err == nil {
			return &embeddedItem
		}
	}

	return nil
}

type Metadata struct {
	Ok           bool      `json:"ok"`
	Self         User      `json:"self"`
	Url          string    `json:"url"`
	ImsList      []Im      `json:"ims"`
	ChannelsList []Channel `json:"channels"`
	UsersList    []User    `json:"users"`
	Error        string    `json:"error"`

	Users    map[string]User
	Channels map[string]Channel
}

type Client struct {
	SlackToken  string `json:"bot_token"`
	TeamId      string
	data        *Metadata
	conn        *websocket.Conn
	redisClient *redis.Client
}

type User struct {
	Id                  string `json:"id"`
	Name                string `json:"name"`
	Color               string `json:"color"`
	Timezone            string `json:"tz"`
	TimezoneDescription string `json:"tz_label"`
	TimezoneOffset      int64  `json:"tz_offset"`
	IsDeleted           bool   `json:"deleted"`
	IsAdmin             bool   `json:"is_admin"`
	IsBot               bool   `json:"is_bot"`
	IsOwner             bool   `json:"is_owner"`
	IsPrimaryOwner      bool   `json:"is_primary_owner"`
	IsRestricted        bool   `json:"is_restricted"`
}

type Command struct {
	UserUid     string `json:"user_uid"`
	ChannelUid  string `json:"channel_uid"`
	TeamUid     string `json:"team_uid"`
	Im          bool   `json:"im"`
	Text        string `json:"text"`
	RelaxBotUid string `json:"relax_bot_uid"`
	Timestamp   string `json:"timestamp"`
	Provider    string `json:"provider"`
}

func NewClient(teamId string, initJSON string) (*Client, error) {
	var c Client
	var err error

	if err = json.Unmarshal([]byte(initJSON), &c); err == nil {
		c.TeamId = teamId
	} else {
		return nil, err
	}

	return &c, nil
}

func (c *Client) Login() error {
	contents, err := c.callSlack("rtm.start", map[string][]string{}, 200)
	var metadata Metadata

	if err != nil {
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
			for _, u := range metadata.UsersList {
				metadata.Users[u.Id] = u
			}

			c.data = &metadata
		} else {
			return err
		}
	}

	return nil
}

func (c *Client) Start() error {
	// Make connection to redis now
	c.redisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	if c.data.Ok == true {
		conn, _, err := websocket.DefaultDialer.Dial(c.data.Url, http.Header{})
		if err != nil {
			return err
		} else {
			c.conn = conn
		}

		go c.startReadLoop()
	} else {
		// Bot has been disabled by the user,
		// so we need to mark it as disabled
		if c.data.Error == "invalid_auth" {
			err := c.sendResponse("disable_bot", "")
			if err != nil {
				return err
			}
		}

		return fmt.Errorf(fmt.Sprintf("error connecting to slack websocket server: %s", c.data.Error))
	}

	return nil
}

func (c *Client) callSlack(method string, params url.Values, expectedStatusCode int) (string, error) {
	params.Set("token", c.SlackToken)
	method = "/api/" + method

	return c.callAPI(os.Getenv("SLACK_HOST"), method, params, expectedStatusCode)
}

func (c *Client) sendResponse(responseType string, payload string) error {
	response := &Response{
		Type:    responseType,
		Payload: payload,
	}

	jsonResponse, err := json.Marshal(response)

	if err != nil {
		return err
	} else {

		intCmd := c.redisClient.RPush(os.Getenv("REDIS_QUEUE_WEB"), string(jsonResponse))
		if intCmd == nil || intCmd.Val() != 1 {
			return fmt.Errorf("Unexpected error while pushing to REDIS_QUEUE_WEB")
		}
	}

	return nil
}

func (c *Client) startReadLoop() {
	for {
		messageType, msg, err := c.conn.ReadMessage()
		if err != nil {
			continue
		}

		if messageType == websocket.TextMessage {
			var message Message

			if err = json.Unmarshal(msg, &message); err == nil {
				c.handleMessage(&message)
			} else {
				fmt.Printf("error recognizing message from Slack, err: %s, error msg: %+v\n", msg, err)
			}
		}
	}
}

func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case "reaction_added":
		if shouldSendToBot(msg) == true {
			embeddedItem := msg.EmbeddedItem()
			if embeddedItem != nil {
				channelId := embeddedItem.ChannelId()
				userId := msg.UserId()

				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]

				command := &Command{
					UserUid:     msg.UserId(),
					ChannelUid:  msg.Channel.Id,
					TeamUid:     c.TeamId,
					Im:          msg.Channel.Im,
					Text:        msg.Reaction,
					RelaxBotUid: c.data.Self.Id,
					Timestamp:   embeddedItem.Timestamp,
					Provider:    "slack",
				}

				commandJson, err := json.Marshal(command)
				if err == nil {
					c.sendResponse("reaction_added", string(commandJson))
				}
			}
		}

	case "reaction_removed":
		if shouldSendToBot(msg) == true {
			embeddedItem := msg.EmbeddedItem()
			if embeddedItem != nil {
				channelId := embeddedItem.ChannelId()
				userId := msg.UserId()

				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]

				command := &Command{
					UserUid:     msg.UserId(),
					ChannelUid:  msg.Channel.Id,
					TeamUid:     c.TeamId,
					Im:          msg.Channel.Im,
					Text:        msg.Reaction,
					RelaxBotUid: c.data.Self.Id,
					Timestamp:   embeddedItem.Timestamp,
					Provider:    "slack",
				}

				commandJson, err := json.Marshal(command)
				if err == nil {
					c.sendResponse("reaction_removed", string(commandJson))
				}
			}
		}

	case "message":
		userId := msg.UserId()
		channelId := msg.ChannelId()

		switch msg.Subtype {
		// message edited
		case "message_deleted":
			if shouldSendToBot(msg) == true {
				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]
				command := &Command{
					UserUid:     msg.UserId(),
					ChannelUid:  msg.Channel.Id,
					TeamUid:     c.TeamId,
					Im:          msg.Channel.Im,
					Text:        msg.Text,
					RelaxBotUid: c.data.Self.Id,
					Timestamp:   msg.DeletedTimestamp,
					Provider:    "slack",
				}

				commandJson, err := json.Marshal(command)
				if err == nil {
					c.sendResponse("message_deleted", string(commandJson))
				}
			}

		case "message_changed":
			if shouldSendToBot(msg) == true {
				embeddedMessage := msg.EmbeddedMessage()
				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]

				if embeddedMessage != nil {
					command := &Command{
						UserUid:     embeddedMessage.UserId(),
						ChannelUid:  msg.Channel.Id,
						TeamUid:     c.TeamId,
						Im:          msg.Channel.Im,
						Text:        embeddedMessage.Text,
						RelaxBotUid: c.data.Self.Id,
						Timestamp:   embeddedMessage.Timestamp,
						Provider:    "slack",
					}

					commandJson, err := json.Marshal(command)
					if err == nil {
						c.sendResponse("message_edited", string(commandJson))
					}
				}
			}
		// simple message
		case "":
			// Ignore Messages sent from the bot itself
			if userId != c.data.Self.Id {
				msg.User = c.data.Users[userId]
				msg.Channel = c.data.Channels[channelId]

				if msg.Channel.Im == true || IsMessageForBot(msg, c.data.Self.Id) {
					if shouldSendToBot(msg) == true {
						command := &Command{
							UserUid:     msg.User.Id,
							ChannelUid:  msg.Channel.Id,
							TeamUid:     c.TeamId,
							Im:          msg.Channel.Im,
							Text:        msg.Text,
							RelaxBotUid: c.data.Self.Id,
							Timestamp:   msg.Timestamp,
							Provider:    "slack",
						}

						commandJson, err := json.Marshal(command)
						if err == nil {
							c.sendResponse("message_new", string(commandJson))
						}
					}
				}
			}
		}
	case "team_join":
		if err := json.Unmarshal(msg.RawUser, &msg.User); err == nil {
			c.data.Users[msg.User.Id] = msg.User
			command := &Command{
				UserUid:     msg.User.Id,
				TeamUid:     c.TeamId,
				RelaxBotUid: c.data.Self.Id,
				Provider:    "slack",
			}
			commandJson, err := json.Marshal(command)
			if err == nil {
				c.sendResponse("team_joined", string(commandJson))
			}
		}
	case "im_created":
		if err := json.Unmarshal(msg.RawChannel, &msg.Channel); err == nil {
			msg.Channel.Im = true
			c.data.Channels[msg.Channel.Id] = msg.Channel
			msg.User = c.data.Users[msg.UserId()]

			command := &Command{
				UserUid:     msg.User.Id,
				ChannelUid:  msg.Channel.Id,
				TeamUid:     c.TeamId,
				Im:          msg.Channel.Im,
				RelaxBotUid: c.data.Self.Id,
				Provider:    "slack",
			}
			commandJson, err := json.Marshal(command)
			if err == nil {
				c.sendResponse("im_created", string(commandJson))
			}
		}
	}
}

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
