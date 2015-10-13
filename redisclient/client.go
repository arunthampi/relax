package redisclient

import (
	"fmt"
	"net/url"
	"os"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/gopkg.in/redis.v3"
)

var redisClient *redis.Client

func Client() *redis.Client {
	if redisClient == nil {
		password := ""
		host := ""

		redisUrl := os.Getenv("REDIS_URL")

		if redisUrl != "" {
			url, err := url.Parse(redisUrl)
			if err == nil {
				host = url.Host
				if url.User != nil {
					password, _ = url.User.Password()
				}
			}
		} else {
			host = os.Getenv("REDIS_HOST")
			password = os.Getenv("REDIS_PASSWORD")
		}

		fmt.Printf("host: %s userInfo: %s\n", host, password)
		redisClient = redis.NewClient(&redis.Options{
			Addr:     host,
			Password: password,
			DB:       0,
		})
	}

	return redisClient
}
