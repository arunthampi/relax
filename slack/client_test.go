package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/gorilla/websocket"
	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/onsi/gomega"
	"gopkg.in/redis.v3"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "client")
}

func newTestServer(jsonResponse string, statusCode int) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, jsonResponse)
	}))

	return server
}

func setRedisQueueWebEnv() {
	now := time.Now()
	os.Setenv("REDIS_QUEUE_WEB", fmt.Sprintf("redis_key_queue_web_%d", now.Nanosecond()))
}

func newWSServer(wsResponse string) *httptest.Server {
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}

		var upgrader websocket.Upgrader

		ws, err := upgrader.Upgrade(w, r, http.Header{})
		if err != nil {
			return
		}
		defer ws.Close()

		err = ws.WriteMessage(websocket.TextMessage, []byte(wsResponse))
		if err != nil {
			panic(err)
		}
	}))

	return wsServer
}

func newRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
}

// Stolen from: github.com/gorilla/websocket
func makeWsProto(s string) string {
	return "ws" + strings.TrimPrefix(s, "http")
}

var _ = Describe("Client", func() {
	BeforeEach(func() {
		os.Setenv("REDIS_HOST", "localhost:6379")
		rc := newRedisClient()
		rc.FlushDb()
	})

	Describe("NewClient", func() {
		Context("with valid params", func() {
			It("should initialize a new client with no error", func() {
				client, err := NewClient("TDEADBEEF", "{\"bot_token\":\"xoxo_deadbeef\"}")

				Expect(err).To(BeNil())
				Expect(client.TeamId).To(Equal("TDEADBEEF"))
				Expect(client.SlackToken).To(Equal("xoxo_deadbeef"))
			})
		})

		Context("without valid params", func() {
			It("should initialize a new client with an error", func() {
				client, err := NewClient("TDEADBEEF", "")

				Expect(err).ToNot(BeNil())
				Expect(client).To(BeNil())
			})
		})
	})

	Describe("Login", func() {
		var server *httptest.Server
		var existingSlackHost string
		var client *Client
		var err error

		Context("unsuccessful login", func() {
			BeforeEach(func() {
				server = newTestServer("{\"ok\": false}", 500)
				existingSlackHost = os.Getenv("SLACK_HOST")
				os.Setenv("SLACK_HOST", server.URL)

				client, err = NewClient("TDEADBEEF", "{\"bot_token\":\"xoxo_deadbeef\"}")
				Expect(err).To(BeNil())
			})

			AfterEach(func() {
				os.Setenv("SLACK_HOST", existingSlackHost)
			})

			It("shouldn't set the metadata", func() {
				err := client.Login()
				Expect(err).ToNot(BeNil())
				Expect(client.data).To(BeNil())
			})
		})

		Context("successful login", func() {
			BeforeEach(func() {
				server = newTestServer(`
{
    "ok": true,
    "url": "wss://ms9.slack-msgs.com/websocket/7I5yBpcvk",

    "self": {
        "id": "U023BECGF",
        "name": "bot",
        "created": 1402463766,
        "manual_presence": "active"
    },
    "team": {
        "id": "T024BE7LD",
        "name": "Example Team",
        "email_domain": "",
        "domain": "example",
        "msg_edit_window_mins": -1,
        "over_storage_limit": false,
        "plan": "std"
    },
    "users": [
        {
            "id": "U023BECGF",
            "name": "bobby",
            "deleted": false,
            "color": "9f69e7",
            "profile": {
                "first_name": "Bobby",
                "last_name": "Tables",
                "real_name": "Bobby Tables",
                "email": "bobby@slack.com",
                "skype": "my-skype-name",
                "phone": "+1 (123) 456 7890",
                "image_24": "https://...",
                "image_32": "https://...",
                "image_48": "https://...",
                "image_72": "https://...",
                "image_192": "https://..."
            },
            "is_admin": true,
            "is_owner": true,
            "has_2fa": false,
            "has_files": true
        },
        {
            "id": "U023BECGG",
            "name": "johnny",
            "deleted": true,
            "color": "9f69e7",
            "profile": {
                "first_name": "Johnny",
                "last_name": "Tables",
                "real_name": "Johnny Tables",
                "email": "johnny@slack.com",
                "skype": "my-skype-name",
                "phone": "+1 (123) 456 7890",
                "image_24": "https://...",
                "image_32": "https://...",
                "image_48": "https://...",
                "image_72": "https://...",
                "image_192": "https://..."
            },
            "is_admin": true,
            "is_owner": true,
            "has_2fa": false,
            "has_files": true
        }
    ],
    "channels": [
        {
            "id": "C024BE91L",
            "name": "fun",
            "created": 1360782804,
            "creator": "U024BE7LH",
            "is_archived": false,
            "is_member": false,
            "num_members": 6,
            "topic": {
                "value": "Fun times",
                "creator": "U024BE7LV",
                "last_set": 1369677212
            },
            "purpose": {
                "value": "This channel is for fun",
                "creator": "U024BE7LH",
                "last_set": 1360782804
            }
        },
        {
            "id": "C024BE91M",
            "name": "sad",
            "created": 1360782804,
            "creator": "U024BE7LH",
            "is_archived": false,
            "is_member": false,
            "num_members": 6,
            "topic": {
                "value": "Sad times",
                "creator": "U024BE7LV",
                "last_set": 1369677212
            },
            "purpose": {
                "value": "This channel is for sads",
                "creator": "U024BE7LH",
                "last_set": 1360782804
            }
        }
    ],
    "ims": [
        {
           "id": "D024BFF1M",
           "is_im": true,
           "user": "USLACKBOT",
           "created": 1372105335,
           "is_user_deleted": false
        },
        {
           "id": "D024BE7RE",
           "is_im": true,
           "user": "U024BE7LH",
           "created": 1356250715,
           "is_user_deleted": false
        }
    ]
}
`, 200)
				existingSlackHost = os.Getenv("SLACK_HOST")
				os.Setenv("SLACK_HOST", server.URL)
				client, err = NewClient("TDEADBEEF", "{\"bot_token\":\"xoxo_deadbeef\"}")
				Expect(err).To(BeNil())
			})

			AfterEach(func() {
				os.Setenv("SLACK_HOST", existingSlackHost)
			})

			It("should set the metadata for the client", func() {
				err := client.Login()

				Expect(err).To(BeNil())
				Expect(client.data).ToNot(BeNil())

				Expect(client.data.Ok).To(BeTrue())
				Expect(client.data.Url).To(Equal("wss://ms9.slack-msgs.com/websocket/7I5yBpcvk"))
				Expect(client.data.Self.Id).To(Equal("U023BECGF"))
				Expect(client.data.Self.Name).To(Equal("bot"))

				Expect(len(client.data.ImsList)).To(Equal(2))
				Expect(client.data.ImsList[0].Id).To(Equal("D024BFF1M"))
				Expect(client.data.ImsList[0].Created).To(Equal(int64(1372105335)))
				Expect(client.data.ImsList[0].CreatorId).To(Equal("USLACKBOT"))

				Expect(client.data.ImsList[1].Id).To(Equal("D024BE7RE"))
				Expect(client.data.ImsList[1].Created).To(Equal(int64(1356250715)))
				Expect(client.data.ImsList[1].CreatorId).To(Equal("U024BE7LH"))

				Expect(len(client.data.UsersList)).To(Equal(2))
				Expect(client.data.UsersList[0].Id).To(Equal("U023BECGF"))
				Expect(client.data.UsersList[0].Name).To(Equal("bobby"))
				Expect(client.data.UsersList[0].Color).To(Equal("9f69e7"))
				Expect(client.data.UsersList[0].IsDeleted).To(BeFalse())

				Expect(client.data.UsersList[1].Id).To(Equal("U023BECGG"))
				Expect(client.data.UsersList[1].Name).To(Equal("johnny"))
				Expect(client.data.UsersList[1].Color).To(Equal("9f69e7"))
				Expect(client.data.UsersList[1].IsDeleted).To(BeTrue())

				Expect(len(client.data.ChannelsList)).To(Equal(2))
				Expect(client.data.ChannelsList[0].Id).To(Equal("C024BE91L"))
				Expect(client.data.ChannelsList[0].Name).To(Equal("fun"))
				Expect(client.data.ChannelsList[0].CreatorId).To(Equal("U024BE7LH"))
				Expect(client.data.ChannelsList[0].Created).To(Equal(int64(1360782804)))

				Expect(client.data.ChannelsList[1].Id).To(Equal("C024BE91M"))
				Expect(client.data.ChannelsList[1].Name).To(Equal("sad"))
				Expect(client.data.ChannelsList[1].CreatorId).To(Equal("U024BE7LH"))
				Expect(client.data.ChannelsList[1].Created).To(Equal(int64(1360782804)))

				Expect(len(client.data.Channels)).To(Equal(4))
				Expect(len(client.data.Users)).To(Equal(2))
			})
		})
	})

	Describe("Start", func() {
		var client *Client
		var err error

		BeforeEach(func() {
			client, err = NewClient("TDEADBEEF", "{\"bot_token\":\"xoxo_deadbeef\"}")
			Expect(err).To(BeNil())
		})

		Context("when the metadata.Ok flag is not true", func() {
			BeforeEach(func() {
				client.data = &Metadata{Ok: false}
				setRedisQueueWebEnv()
			})

			Context("when error is set to invalid_auth", func() {
				BeforeEach(func() {
					client.data.Error = "invalid_auth"
				})

				It("should return an error and send a response via Redis", func() {
					var resp Response
					var cmd Command

					err = client.Start()
					Expect(err).ToNot(BeNil())

					redisClient := newRedisClient()

					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(2))
					err := json.Unmarshal([]byte(result[1]), &resp)

					Expect(err).To(BeNil())
					Expect(resp.Type).To(Equal("disable_bot"))
					err = json.Unmarshal([]byte(resp.Payload), &cmd)
					Expect(err).To(BeNil())

					Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
					Expect(cmd.Provider).To(Equal("slack"))
				})
			})

			Context("when there is another error", func() {
				It("should return an error", func() {
					err = client.Start()
					Expect(err).ToNot(BeNil())
				})
			})
		})

		Context("when the metadata.Ok flag is true", func() {
			BeforeEach(func() {
				wsServer := newWSServer("{\"ok\":true}")
				client.data = &Metadata{Ok: true, Url: makeWsProto(wsServer.URL)}
			})

			It("should not return an error", func() {
				err = client.Start()
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Handling Events from Slack RTM API", func() {
		var client *Client
		var err error
		var redisClient *redis.Client
		var wsServer *httptest.Server

		BeforeEach(func() {
			client, err = NewClient("TDEADBEEF", "{\"bot_token\":\"xoxo_deadbeef\"}")
			Expect(err).To(BeNil())
		})

		Context("message event", func() {
			BeforeEach(func() {
				redisClient = newRedisClient()
				setRedisQueueWebEnv()
			})

			Context("message comes in from Slack when the user/channel don't exist in the map", func() {
				BeforeEach(func() {
					wsServer = newWSServer(`
						{
							"type": "message",
							"channel": "C2147483705",
							"user": "U2147483697",
							"text": "Hello world",
							"ts": "1355517523.000005"
						}
					`)

					client.data = &Metadata{Ok: true, Url: makeWsProto(wsServer.URL)}
					client.Start()
				})

				AfterEach(func() {
					wsServer.Close()
				})

				It("should not send an event to Redis", func() {
					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(0))
				})
			})

			Context("message comes in from Slack and is an IM", func() {
				BeforeEach(func() {
					wsServer = newWSServer(`
						{
							"type": "message",
							"channel": "D2147483705",
							"user": "U2147483697",
							"text": "Hello world",
							"ts": "1355517523.000005"
						}
					`)

					client.data = &Metadata{
						Ok:       true,
						Url:      makeWsProto(wsServer.URL),
						Users:    map[string]User{},
						Channels: map[string]Channel{},
						Self:     User{Id: "UBOTUID"},
					}
					client.data.Channels["D2147483705"] = Channel{Id: "D2147483705", Im: true}
					client.data.Users["U2147483697"] = User{Id: "U2147483697"}
					client.TeamId = "TDEADBEEF"

					client.Start()
				})

				AfterEach(func() {
					wsServer.Close()
				})

				It("should send an event to Redis", func() {
					var resp Response
					var cmd Command

					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(2))
					err := json.Unmarshal([]byte(result[1]), &resp)

					Expect(err).To(BeNil())
					Expect(resp.Type).To(Equal("message_new"))
					err = json.Unmarshal([]byte(resp.Payload), &cmd)
					Expect(err).To(BeNil())

					Expect(cmd.ChannelUid).To(Equal("D2147483705"))
					Expect(cmd.UserUid).To(Equal("U2147483697"))
					Expect(cmd.Text).To(Equal("Hello world"))
					Expect(cmd.Timestamp).To(Equal("1355517523.000005"))
					Expect(cmd.Im).To(BeTrue())
					Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
					Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
					Expect(cmd.Provider).To(Equal("slack"))
				})
			})

			Context("message comes in from Slack and is addressed to the bot (with bot UID at the beginning of the message)", func() {
				BeforeEach(func() {
					wsServer = newWSServer(`
						{
							"type": "message",
							"channel": "C2147483705",
							"user": "U2147483697",
							"text": "<@UBOTUID>: Hello world",
							"ts": "1355517523.000005"
						}
					`)

					client.data = &Metadata{
						Ok:       true,
						Url:      makeWsProto(wsServer.URL),
						Users:    map[string]User{},
						Channels: map[string]Channel{},
						Self:     User{Id: "UBOTUID"},
					}
					client.data.Channels["C2147483705"] = Channel{Id: "C2147483705", Im: false}
					client.data.Users["U2147483697"] = User{Id: "U2147483697"}
					client.TeamId = "TDEADBEEF"

					client.Start()
				})

				AfterEach(func() {
					wsServer.Close()
				})

				It("should send an event to Redis", func() {
					var resp Response
					var cmd Command

					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(2))
					err := json.Unmarshal([]byte(result[1]), &resp)

					Expect(err).To(BeNil())
					Expect(resp.Type).To(Equal("message_new"))
					err = json.Unmarshal([]byte(resp.Payload), &cmd)
					Expect(err).To(BeNil())

					Expect(cmd.ChannelUid).To(Equal("C2147483705"))
					Expect(cmd.UserUid).To(Equal("U2147483697"))
					Expect(cmd.Text).To(Equal("<@UBOTUID>: Hello world"))
					Expect(cmd.Timestamp).To(Equal("1355517523.000005"))
					Expect(cmd.Im).To(BeFalse())
					Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
					Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
					Expect(cmd.Provider).To(Equal("slack"))
				})
			})

			Context("message comes in from Slack and is addressed to the bot (with bot UID at in the middle of the message)", func() {
				BeforeEach(func() {
					wsServer = newWSServer(`
						{
							"type": "message",
							"channel": "C2147483705",
							"user": "U2147483697",
							"text": "Hey <@UBOTUID> Hello world",
							"ts": "1355517523.000005"
						}
					`)

					client.data = &Metadata{
						Ok:       true,
						Url:      makeWsProto(wsServer.URL),
						Users:    map[string]User{},
						Channels: map[string]Channel{},
						Self:     User{Id: "UBOTUID"},
					}
					client.data.Channels["C2147483705"] = Channel{Id: "C2147483705", Im: false}
					client.data.Users["U2147483697"] = User{Id: "U2147483697"}
					client.TeamId = "TDEADBEEF"

					client.Start()
				})

				AfterEach(func() {
					wsServer.Close()
				})

				It("should send an event to Redis", func() {
					var resp Response
					var cmd Command

					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(2))
					err := json.Unmarshal([]byte(result[1]), &resp)

					Expect(err).To(BeNil())
					Expect(resp.Type).To(Equal("message_new"))
					err = json.Unmarshal([]byte(resp.Payload), &cmd)
					Expect(err).To(BeNil())

					Expect(cmd.ChannelUid).To(Equal("C2147483705"))
					Expect(cmd.UserUid).To(Equal("U2147483697"))
					Expect(cmd.Text).To(Equal("Hey <@UBOTUID> Hello world"))
					Expect(cmd.Timestamp).To(Equal("1355517523.000005"))
					Expect(cmd.Im).To(BeFalse())
					Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
					Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
					Expect(cmd.Provider).To(Equal("slack"))
				})
			})

			Context("text message edited", func() {
				BeforeEach(func() {
					wsServer = newWSServer(`
						{
							"type": "message",
							"subtype": "message_changed",
							"hidden": true,
							"channel": "C2147483705",
							"ts": "1358878755.000001",
							"message": {
								"type": "message",
								"user": "U2147483697",
								"text": "Hello, world!",
								"ts": "1355517523.000005",
								"edited": {
									"user": "U2147483697",
									"ts": "1358878755.000001"
								}
							}
						}
					`)

					client.data = &Metadata{
						Ok:       true,
						Url:      makeWsProto(wsServer.URL),
						Users:    map[string]User{},
						Channels: map[string]Channel{},
						Self:     User{Id: "UBOTUID"},
					}
					client.data.Channels["C2147483705"] = Channel{Id: "C2147483705", Im: false}
					client.data.Users["U2147483697"] = User{Id: "U2147483697"}
					client.TeamId = "TDEADBEEF"

					client.Start()
				})

				AfterEach(func() {
					wsServer.Close()
				})

				It("should send a 'message_edited' event to Redis with the timestamp being the one set in message.ts", func() {
					// From the Slack API Docs: https://api.slack.com/events/message/message_changed
					// When clients recieve this message type, they should look for an existing message
					// with the same message.ts in that channel. If they find one the existing message should be replaced with the new one.
					var resp Response
					var cmd Command

					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(2))
					err := json.Unmarshal([]byte(result[1]), &resp)

					Expect(err).To(BeNil())
					Expect(resp.Type).To(Equal("message_edited"))
					err = json.Unmarshal([]byte(resp.Payload), &cmd)
					Expect(err).To(BeNil())

					Expect(cmd.ChannelUid).To(Equal("C2147483705"))
					Expect(cmd.UserUid).To(Equal("U2147483697"))
					Expect(cmd.Text).To(Equal("Hello, world!"))
					Expect(cmd.Timestamp).To(Equal("1355517523.000005"))
					Expect(cmd.Im).To(BeFalse())
					Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
					Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
					Expect(cmd.Provider).To(Equal("slack"))
				})
			})

			Context("text message deleted", func() {
				BeforeEach(func() {
					wsServer = newWSServer(`
						{
							"type": "message",
							"subtype": "message_deleted",
							"hidden": true,
							"channel": "C2147483705",
							"user": "U2147483697",
							"ts": "1358878755.000001",
							"deleted_ts": "1358878749.000002"
						}
					`)

					client.data = &Metadata{
						Ok:       true,
						Url:      makeWsProto(wsServer.URL),
						Users:    map[string]User{},
						Channels: map[string]Channel{},
						Self:     User{Id: "UBOTUID"},
					}
					client.data.Channels["C2147483705"] = Channel{Id: "C2147483705", Im: false}
					client.data.Users["U2147483697"] = User{Id: "U2147483697"}
					client.TeamId = "TDEADBEEF"

					client.Start()
				})

				AfterEach(func() {
					wsServer.Close()
				})

				It("should send a 'message_deleted' event to Redis with the timestamp being the one set in message.deleted_ts", func() {
					// From the Slack API Docs: https://api.slack.com/events/message/message_deleted
					// When clients recieve this message type, they should look for an existing message
					// with the same message.deleted_ts in that channel. If they find one they should delete this message
					var resp Response
					var cmd Command

					resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
					result := resultCmd.Val()

					Expect(len(result)).To(Equal(2))
					err := json.Unmarshal([]byte(result[1]), &resp)

					Expect(err).To(BeNil())
					Expect(resp.Type).To(Equal("message_deleted"))
					err = json.Unmarshal([]byte(resp.Payload), &cmd)
					Expect(err).To(BeNil())

					Expect(cmd.ChannelUid).To(Equal("C2147483705"))
					Expect(cmd.UserUid).To(Equal("U2147483697"))
					Expect(cmd.Text).To(Equal(""))
					Expect(cmd.Timestamp).To(Equal("1358878749.000002"))
					Expect(cmd.Im).To(BeFalse())
					Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
					Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
					Expect(cmd.Provider).To(Equal("slack"))
				})
			})
		})

		Context("reaction added", func() {
			BeforeEach(func() {
				redisClient = newRedisClient()
				setRedisQueueWebEnv()

				wsServer = newWSServer(`
					{
						"type": "reaction_added",
						"user": "U024BE7LH",
						"reaction": "+1",
						"item": {
							"type": "message",
							"channel": "C0304SBLA",
							"ts": "1435766912.000727"
						},
						"event_ts": "1360782804.083113"
					}
				`)

				client.data = &Metadata{
					Ok:       true,
					Url:      makeWsProto(wsServer.URL),
					Users:    map[string]User{},
					Channels: map[string]Channel{},
					Self:     User{Id: "UBOTUID"},
				}
				client.data.Channels["C0304SBLA"] = Channel{Id: "C0304SBLA", Im: false}
				client.data.Users["U024BE7LH"] = User{Id: "U024BE7LH"}
				client.TeamId = "TDEADBEEF"

				client.Start()
			})

			It("should send a 'reaction_added' event to Redis with the timestamp being the one set in item.ts", func() {
				var resp Response
				var cmd Command

				resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
				result := resultCmd.Val()

				Expect(len(result)).To(Equal(2))
				err := json.Unmarshal([]byte(result[1]), &resp)

				Expect(err).To(BeNil())
				Expect(resp.Type).To(Equal("reaction_added"))
				err = json.Unmarshal([]byte(resp.Payload), &cmd)
				Expect(err).To(BeNil())

				Expect(cmd.ChannelUid).To(Equal("C0304SBLA"))
				Expect(cmd.UserUid).To(Equal("U024BE7LH"))
				Expect(cmd.Text).To(Equal("+1"))
				Expect(cmd.Timestamp).To(Equal("1435766912.000727"))
				Expect(cmd.Im).To(BeFalse())
				Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
				Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
				Expect(cmd.Provider).To(Equal("slack"))
			})
		})

		Context("reaction removed", func() {
			BeforeEach(func() {
				redisClient = newRedisClient()
				setRedisQueueWebEnv()

				wsServer = newWSServer(`
					{
						"type": "reaction_removed",
						"user": "U024BE7LH",
						"reaction": "+1",
						"item": {
							"type": "message",
							"channel": "C0304SBLA",
							"ts": "1435766912.000727"
						},
						"event_ts": "1360782804.083113"
					}
				`)

				client.data = &Metadata{
					Ok:       true,
					Url:      makeWsProto(wsServer.URL),
					Users:    map[string]User{},
					Channels: map[string]Channel{},
					Self:     User{Id: "UBOTUID"},
				}
				client.data.Channels["C0304SBLA"] = Channel{Id: "C0304SBLA", Im: false}
				client.data.Users["U024BE7LH"] = User{Id: "U024BE7LH"}
				client.TeamId = "TDEADBEEF"

				client.Start()
			})

			It("should send a 'reaction_removed' event to Redis with the timestamp being the one set in item.ts", func() {
				var resp Response
				var cmd Command

				resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
				result := resultCmd.Val()

				Expect(len(result)).To(Equal(2))
				err := json.Unmarshal([]byte(result[1]), &resp)

				Expect(err).To(BeNil())
				Expect(resp.Type).To(Equal("reaction_removed"))
				err = json.Unmarshal([]byte(resp.Payload), &cmd)
				Expect(err).To(BeNil())

				Expect(cmd.ChannelUid).To(Equal("C0304SBLA"))
				Expect(cmd.UserUid).To(Equal("U024BE7LH"))
				Expect(cmd.Text).To(Equal("+1"))
				Expect(cmd.Timestamp).To(Equal("1435766912.000727"))
				Expect(cmd.Im).To(BeFalse())
				Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
				Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
				Expect(cmd.Provider).To(Equal("slack"))
			})
		})

		Context("team_join", func() {
			BeforeEach(func() {
				redisClient = newRedisClient()
				setRedisQueueWebEnv()

				wsServer = newWSServer(`
				{
					"type": "team_join",
					"user": {
						"id": "U023BECGF",
						"name": "bobby",
						"deleted": false,
						"color": "9f69e7",
						"profile": {
							"first_name": "Bobby",
							"last_name": "Tables",
							"real_name": "Bobby Tables",
							"email": "bobby@slack.com",
							"skype": "my-skype-name",
							"phone": "+1 (123) 456 7890",
							"image_24": "https://...",
							"image_32": "https://...",
							"image_48": "https://...",
							"image_72": "https://...",
							"image_192": "https://..."
						},
						"is_admin": true,
						"is_owner": true,
						"has_2fa": false,
						"has_files": true
					}
				}
			`)

				client.data = &Metadata{
					Ok:       true,
					Url:      makeWsProto(wsServer.URL),
					Users:    map[string]User{},
					Channels: map[string]Channel{},
					Self:     User{Id: "UBOTUID"},
				}
				client.TeamId = "TDEADBEEF"

				client.Start()
			})

			It("should send a 'users_bulk_create' event to Redis with the team ID and the user", func() {
				var resp Response
				var cmd Command

				resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
				result := resultCmd.Val()

				Expect(len(result)).To(Equal(2))
				err := json.Unmarshal([]byte(result[1]), &resp)

				Expect(err).To(BeNil())
				Expect(resp.Type).To(Equal("team_joined"))
				err = json.Unmarshal([]byte(resp.Payload), &cmd)
				Expect(err).To(BeNil())

				Expect(cmd.ChannelUid).To(Equal(""))
				Expect(cmd.UserUid).To(Equal("U023BECGF"))
				Expect(cmd.Text).To(Equal(""))
				Expect(cmd.Timestamp).To(Equal(""))
				Expect(cmd.Im).To(BeFalse())
				Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
				Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
				Expect(cmd.Provider).To(Equal("slack"))

				Expect(client.data.Users["U023BECGF"].Id).To(Equal("U023BECGF"))
				Expect(client.data.Users["U023BECGF"].Name).To(Equal("bobby"))
			})
		})

		Context("im_created", func() {
			BeforeEach(func() {
				redisClient = newRedisClient()
				setRedisQueueWebEnv()

				wsServer = newWSServer(`
					{
						"type": "im_created",
						"user": "U024BE7LH",
						"channel": {
							"id": "D024BE91L",
							"name": "fun",
							"created": 1360782804,
							"creator": "U024BE7LH",
							"is_archived": false,
							"is_member": true
						}
					}
				`)

				client.data = &Metadata{
					Ok:       true,
					Url:      makeWsProto(wsServer.URL),
					Users:    map[string]User{},
					Channels: map[string]Channel{},
					Self:     User{Id: "UBOTUID"},
				}
				client.data.Users["U024BE7LH"] = User{Id: "U024BE7LH"}
				client.TeamId = "TDEADBEEF"

				client.Start()
			})

			It("should send a 'im_created' event to Redis with the team ID and the user", func() {
				var resp Response
				var cmd Command

				resultCmd := redisClient.BLPop(5*time.Second, os.Getenv("REDIS_QUEUE_WEB"))
				result := resultCmd.Val()

				Expect(len(result)).To(Equal(2))
				err := json.Unmarshal([]byte(result[1]), &resp)

				Expect(err).To(BeNil())
				Expect(resp.Type).To(Equal("im_created"))
				err = json.Unmarshal([]byte(resp.Payload), &cmd)
				Expect(err).To(BeNil())

				Expect(cmd.ChannelUid).To(Equal("D024BE91L"))
				Expect(cmd.UserUid).To(Equal("U024BE7LH"))
				Expect(cmd.Text).To(Equal(""))
				Expect(cmd.Timestamp).To(Equal(""))
				Expect(cmd.Im).To(BeTrue())
				Expect(cmd.RelaxBotUid).To(Equal("UBOTUID"))
				Expect(cmd.TeamUid).To(Equal("TDEADBEEF"))
				Expect(cmd.Provider).To(Equal("slack"))

				Expect(client.data.Channels["D024BE91L"].Id).To(Equal("D024BE91L"))
			})

		})
	})
})
