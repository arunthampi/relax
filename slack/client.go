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
	"time"

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
	// For some events, such as message_changed, message_deleted, etc.
	// the Timestamp field contains the timestamp of the original message
	// so to make sure only one instance of the event is sent to REDIS_QUEUE_WEB
	// only once, and will be used by the `shouldSendToBot` function
	EventTimestamp string `json:"event_ts"`

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

type Event struct {
	UserUid        string `json:"user_uid"`
	ChannelUid     string `json:"channel_uid"`
	TeamUid        string `json:"team_uid"`
	Im             bool   `json:"im"`
	Text           string `json:"text"`
	RelaxBotUid    string `json:"relax_bot_uid"`
	Timestamp      string `json:"timestamp"`
	Provider       string `json:"provider"`
	EventTimestamp string `json:"event_timestamp"`
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
			var msg Message
			msg.User = User{}
			msg.Channel = Channel{}

			err := c.sendEvent("disable_bot", &msg, "", "", "")
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

func (c *Client) sendEvent(responseType string, msg *Message, text string, timestamp string, eventTimestamp string) error {
	if responseType == "message_new" && !(msg.Channel.Im == true || isMessageForBot(msg, c.data.Self.Id)) {
		return nil
	}

	// If the eventTimestamp blank, then set a timestamp to the currentTime (this typically means)
	// that it is the responsibility of the client to make sure that events are handled idempotently
	if eventTimestamp == "" {
		eventTimestamp = fmt.Sprintf("%d", time.Now().Nanosecond())
	}

	event := &Event{
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
		return nil
	}

	response := &Response{
		Type:    responseType,
		Payload: string(eventJson),
	}

	jsonResponse, err := json.Marshal(response)

	if err != nil {
		return err
	} else {
		if shouldSendEvent(event) == true {
			intCmd := c.redisClient.RPush(os.Getenv("REDIS_QUEUE_WEB"), string(jsonResponse))
			if intCmd == nil || intCmd.Val() != 1 {
				return fmt.Errorf("Unexpected error while pushing to REDIS_QUEUE_WEB")
			}
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
	case "message":
		userId := msg.UserId()
		channelId := msg.ChannelId()

		switch msg.Subtype {
		// message edited
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
			msg.User = c.data.Users[userId]
			msg.Channel = c.data.Channels[channelId]

			c.sendEvent("message_new", msg, msg.Text, msg.Timestamp, msg.Timestamp)
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
