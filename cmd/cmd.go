package cmd

import (
	"github.com/spf13/pflag"
)

type Cmd struct {
	Mode              string
	ServerPort        string
	RemoteHostAndPort string
	LocalHostAndPort  string
}

func ParseCmd() *Cmd {
	cmd := &Cmd{}
	pflag.StringVarP(&cmd.Mode, "mode", "m", "", "服务类型, client or server")
	pflag.StringVarP(&cmd.ServerPort, "serverPort", "s", "", "server端监听端口")
	pflag.StringVarP(&cmd.RemoteHostAndPort, "remoteHostAndPort", "r", "", "client端配置的远程服务端口")
	pflag.StringVarP(&cmd.LocalHostAndPort, "localHostAndPort", "l", "", "client端配置的本地服务端口")
	pflag.Parse()
	return cmd
}
