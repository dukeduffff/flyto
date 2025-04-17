package common

import (
	"errors"
	"github.com/dukeduffff/flyto/cmd"
	"strconv"
	"strings"
)

func IsServer(mode string) (bool, error) {
	if mode == "server" || mode == "s" {
		return true, nil
	} else if mode == "client" || mode == "c" {
		return false, nil
	}
	return false, errors.New("无法识别的模式")
}

type Config struct {
	IsServer          bool
	ServerPort        uint16
	RemoteHostAndPort string
	LocalHostAndPort  []string
}

func getServerPort(c *cmd.Cmd) (uint16, error) {
	port, err := strconv.Atoi(c.ServerPort)
	return uint16(port), err
}

func getRemoteHostAndPort(c *cmd.Cmd) (string, error) {
	return c.RemoteHostAndPort, nil
}

func getLocalHostAndPort(c *cmd.Cmd) ([]string, error) {
	addersStr := c.LocalHostAndPort
	if strings.TrimSpace(addersStr) == "" {
		return nil, errors.New("本地地址不能为空")
	}
	split := strings.Split(addersStr, ",")
	addrs := make([]string, 0)
	for _, addrStr := range split {
		addrs = append(addrs, addrStr)
	}
	return addrs, nil
}

func NewServerConfig(cmd *cmd.Cmd) (*Config, error) {
	server, err := IsServer(cmd.Mode)
	if err != nil {
		return nil, err
	}
	serverPort, err := getServerPort(cmd)
	if err != nil {
		return nil, err
	}

	return &Config{
		IsServer:   server,
		ServerPort: serverPort,
	}, nil
}

func NewClientConfig(cmd *cmd.Cmd) (*Config, error) {
	server, err := IsServer(cmd.Mode)
	if err != nil {
		return nil, err
	}
	remoteHostAndPort, err := getRemoteHostAndPort(cmd)
	if err != nil {
		return nil, err
	}
	localHostAndPort, err := getLocalHostAndPort(cmd)
	if err != nil {
		return nil, err
	}
	return &Config{
		IsServer:          server,
		RemoteHostAndPort: remoteHostAndPort,
		LocalHostAndPort:  localHostAndPort,
	}, nil
}
