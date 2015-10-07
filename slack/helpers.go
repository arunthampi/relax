package slack

import (
	"fmt"
	"os"
	"regexp"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/gopkg.in/redis.v3"
)

// isMessageForBot is a utility method that is used to check whether a message is intended
// for the bot managed by Relax. This checks if an @-mention exists in the message
// with the bot user id
func isMessageForBot(msg *Message, botUserId string) bool {
	re := regexp.MustCompile(fmt.Sprintf("<@(%s)>", botUserId))
	isMessageForBot := false

	if matches := re.FindStringSubmatch(msg.Text); matches != nil {
		if len(matches) == 2 && matches[1] == botUserId {
			isMessageForBot = true
		}
	}
	return isMessageForBot
}

// shouldSendEvent is a utility function that determines whether a message should be sent
// back to the user. When relax is run in "high-availabilty" mode, (i.e. multiple instances of
// Relax are running), we need to make sure that the same event is not sent more than once
// back to the user. We use Redis to make sure to ensure the "send-only-once" requirement.
func shouldSendEvent(event *Event) bool {
	key := fmt.Sprintf("bot_message:%s:%s", event.ChannelUid, event.EventTimestamp)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	boolCmd := redisClient.HSetNX(os.Getenv("REDIS_MUTEX_KEY"), key, "ok")

	if boolCmd == nil {
		return true
	} else {
		return boolCmd.Val()
	}
}
