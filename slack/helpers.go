package slack

import (
	"fmt"
	"os"
	"regexp"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/gopkg.in/redis.v3"
)

func IsMessageForBot(msg *Message, botUserId string) bool {
	re := regexp.MustCompile(fmt.Sprintf("<@(%s)>", botUserId))
	isMessageForBot := false

	if matches := re.FindStringSubmatch(msg.Text); matches != nil {
		if len(matches) == 2 && matches[1] == botUserId {
			isMessageForBot = true
		}
	}
	return isMessageForBot
}

func shouldSendToBot(msg *Message) bool {
	key := fmt.Sprintf("bot_message:%s:%s", msg.Channel.Id, msg.Timestamp)

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
