package main

import (
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

	hcServer := &healthcheck.HealthCheckServer{}
	go hcServer.Start("0.0.0.0", uint16(portInt))

	slack.InitClients()
}
