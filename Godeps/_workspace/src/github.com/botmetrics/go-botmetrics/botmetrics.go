package botmetrics

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type BotmetricsClient struct {
	ApiKey string
	BotId  string
}

var (
	ErrClientNotConfigured = fmt.Errorf("Set the environment variables BOTMETRICS_API_KEY and BOTMETRICS_BOT_ID or call NewBotmetricsClient(\"api_key\", \"bot_id\")")
)

func NewBotmetricsClient(args ...string) (*BotmetricsClient, error) {
	switch len(args) {
	case 0:
		if os.Getenv("BOTMETRICS_API_KEY") == "" || os.Getenv("BOTMETRICS_BOT_ID") == "" {
			return nil, ErrClientNotConfigured
		}

		return &BotmetricsClient{ApiKey: os.Getenv("BOTMETRICS_API_KEY"), BotId: os.Getenv("BOTMETRICS_BOT_ID")}, nil

	case 1:
		if os.Getenv("BOTMETRICS_BOT_ID") == "" {
			return nil, ErrClientNotConfigured
		}

		return &BotmetricsClient{ApiKey: args[0], BotId: os.Getenv("BOTMETRICS_API_ID")}, nil

	case 2:
		return &BotmetricsClient{ApiKey: args[0], BotId: args[1]}, nil

	default:
		return nil, ErrClientNotConfigured
	}
}

func (b *BotmetricsClient) RegisterBot(token string, createdAt int64) bool {
	params := url.Values{
		"format":          []string{"json"},
		"instance[token]": []string{token},
	}
	host := os.Getenv("BOTMETRICS_API_HOST")
	if host == "" {
		host = "https://www.getbotmetrics.com"
	}
	url := fmt.Sprintf("%s/bots/%s/instances", host, b.BotId)

	if createdAt > 0 {
		params["instance[created_at]"] = []string{strconv.Itoa(int(createdAt))}
	}

	client := &http.Client{}
	r, _ := http.NewRequest("POST", url, strings.NewReader(params.Encode()))
	r.Header.Add("Authorization", b.ApiKey)
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(params.Encode())))

	resp, err := client.Do(r)

	if err != nil {
		return false
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 201 {
			return true
		}
	}

	return false
}
