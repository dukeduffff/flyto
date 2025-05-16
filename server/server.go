package server

import (
	"context"
	"fmt"
	"github.com/dukeduffff/flyto/common"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc"
	"log"
	"net"
)

type Server struct {
	c *common.Config
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", ":"+fmt.Sprintf("%v", s.c.ServerPort))
	if err != nil {
		return err
	}
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("Error accepting connection", err)
			continue
		}
		go func() {
			defer conn.Close()
			handleConn(conn)
		}()
	}
}

func NewServer(config *common.Config) *Server {
	return &Server{
		c: config,
	}
}

func handleConn(conn net.Conn) {
	config := yamux.DefaultConfig()
	session, err := yamux.Server(conn, config)
	if err != nil {
		log.Printf("yamux server error: %v", err)
	}
	handleSession(session)
}

func handleSession(session *yamux.Session) {
	srv := grpc.NewServer()
	RegisterFlyToServiceServer(srv, &FlyToServiceServerImpl{session: session})
	srv.Serve(&yamuxListener{session: session})
	// 连接断开后的处理
}

type FlyToServiceServerImpl struct {
	UnimplementedFlyToServiceServer
	session *yamux.Session
}

func (f *FlyToServiceServerImpl) Register(ctx context.Context, request *RegisterRequest) (*RegisterResponse, error) {
	clientId := request.GetClientId()
	infos := request.GetClientInfos()
	log.Println("register client", clientId, infos)
	for _, info := range infos {
		ls, err := NewLocalServer(clientId, info, f.session)
		if err != nil {
			log.Println("register client, create error", err)
			return nil, err
		}
		sessionPortMap.AddSessionPort(f.session, info.GetServerPort())
		if err = ls.Start(); err != nil {
			log.Println("register client, start error", err)
			return nil, err
		}
	}
	resp := &RegisterResponse{}
	resp.Status = true
	return resp, nil
}

func (f *FlyToServiceServerImpl) Ping(ctx context.Context, request *PingRequest) (*PingResponse, error) {
	log.Println("ping, clientId:", request.GetClientId())
	return &PingResponse{}, nil
}

type yamuxListener struct {
	session *yamux.Session
}

func (yl *yamuxListener) Accept() (net.Conn, error) {
	return yl.session.AcceptStream()
}
func (yl *yamuxListener) Close() error   { return yl.session.Close() }
func (yl *yamuxListener) Addr() net.Addr { return yl.session.RemoteAddr() }
