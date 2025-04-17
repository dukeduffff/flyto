package main

import (
	"github.com/dukeduffff/flyto/client"
	"github.com/dukeduffff/flyto/server"
	"testing"
	"time"
)

func TestConnect(m *testing.T) {
	go func() {
		server := server.NewServer(nil)
		server.Start()
	}()
	time.Sleep(1 * time.Second)
	client := client.NewClient()
	client.Start()
}
