package redisclient

import (
	"net/url"
	"os"
	"sync"
	"time"

	log "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/Sirupsen/logrus"

	"github.com/zerobotlabs/relax/Godeps/_workspace/src/gopkg.in/redis.v3"
)

var redisClient *redis.Client
var mu sync.Mutex

func Client() *redis.Client {
	mu.Lock()
	defer mu.Unlock()

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

		redisClient = redis.NewClient(&redis.Options{
			Addr:     host,
			Password: password,
			DB:       0,
		})

		result, err := redisClient.Ping().Result()

		for err != nil || result != "PONG" {
			log.WithFields(log.Fields{
				"host":   host,
				"err":    err,
				"result": result,
			}).Error("retrying connecting to redis host, failed credentials")

			time.Sleep(5 * time.Second)
			result, err = redisClient.Ping().Result()
		}

		log.WithFields(log.Fields{
			"host":   host,
			"result": result,
		}).Info("connected to redis host")
	}

	return redisClient
}
