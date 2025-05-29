package main

import (
	"github.com/dukeduffff/flyto/client"
	"github.com/dukeduffff/flyto/cmd"
	"github.com/dukeduffff/flyto/common"
	"github.com/dukeduffff/flyto/server"
	"log"
)

func main() {
	cmd := cmd.ParseCmd()
	isServer, err := common.IsServer(cmd.Mode)
	if err != nil {
		log.Fatalf("config error, %v", err)
		return
	}
	if !isServer {
		clientConfig, err := common.NewClientConfig(cmd)
		if err != nil {
			log.Fatalf("config error, %v", err)
			return
		}
		client := client.NewClient(clientConfig)
		if err := client.Start(); err != nil {
			log.Println("client start error", err)
		}
	} else if isServer {
		config, err := common.NewServerConfig(cmd)
		if err != nil {
			log.Fatalf("config error, %v", err)
			return
		}
		server := server.NewServer(config)
		if err := server.Start(); err != nil {
			log.Println("server start error", err)
		}
	}
}
