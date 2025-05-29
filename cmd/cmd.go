package cmd

import (
	"github.com/spf13/pflag"
)

type Cmd struct {
	Mode              string
	ServerPort        string
	RemoteHostAndPort string
	LocalHostAndPort  string
	Key               string
	ConnectCnt        int
}

func ParseCmd() *Cmd {
	cmd := &Cmd{}
	pflag.StringVarP(&cmd.Mode, "mode", "m", "", "mode, c: client model; s: server model")
	pflag.StringVarP(&cmd.ServerPort, "server_port", "s", "", "server listen port, only used in server mode")
	pflag.StringVarP(&cmd.RemoteHostAndPort, "remote_host", "r", "", "remote server host and port, only used in client mode")
	pflag.StringVarP(&cmd.LocalHostAndPort, "local_host", "l", "", "local host and port, only used in client mode")
	pflag.IntVarP(&cmd.ConnectCnt, "connect_cnt", "c", 1, "number of connections, only used in client mode")
	pflag.StringVarP(&cmd.Key, "key", "k", "", "Encryption key; if left empty, encryption and legitimacy verification will not be used. Generation method: openssl rand -base64 16")
	pflag.Parse()
	return cmd
}
