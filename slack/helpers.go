package slack

import (
	"fmt"
	"regexp"
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
