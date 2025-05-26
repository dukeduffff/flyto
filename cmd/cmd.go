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
}

func ParseCmd() *Cmd {
	cmd := &Cmd{}
	pflag.StringVarP(&cmd.Mode, "mode", "m", "", "服务类型, client or server")
	pflag.StringVarP(&cmd.ServerPort, "serverPort", "s", "", "server端监听端口")
	pflag.StringVarP(&cmd.RemoteHostAndPort, "remoteHostAndPort", "r", "", "client端配置的远程服务端口")
	pflag.StringVarP(&cmd.LocalHostAndPort, "localHostAndPort", "l", "", "client端配置的本地服务端口")
	pflag.StringVarP(&cmd.Key, "key", "k", "", "加密密钥,为空则不使用加密 生成方式: openssl rand -base64 16")
	pflag.Parse()
	return cmd
}
