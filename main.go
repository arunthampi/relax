package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/zerobotlabs/relax/healthcheck"
	"github.com/zerobotlabs/relax/slack"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "0"
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	if redisCloudUrl := os.Getenv("REDISCLOUD_URL"); redisCloudUrl != "" {
		os.Setenv("REDIS_URL", redisCloudUrl)
	}

	if os.Getenv("RELAX_BOTS_KEY") == "" || os.Getenv("RELAX_BOTS_PUBSUB") == "" ||
		os.Getenv("RELAX_EVENTS_QUEUE") == "" || os.Getenv("REDIS_URL") == "" ||
		os.Getenv("RELAX_MUTEX_KEY") == "" {
		fmt.Printf("usage: RELAX_MUTEX_KEY=relax_mutex_key RELAX_BOTS_KEY=relax_bots_key RELAX_BOTS_PUBSUB=relax_bots_pubsub RELAX_EVENTS_QUEUE=relax_events_queue REDIS_URL=redis://localhost:6379 relax\n")
		os.Exit(1)
	}

	slack.InitClients()

	hcServer := &healthcheck.HealthCheckServer{}
	hcServer.Start("0.0.0.0", uint16(portInt))
}
