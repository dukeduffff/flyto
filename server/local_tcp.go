package server

import (
	"bytes"
	"github.com/dukeduffff/flyto/common"
	"github.com/dukeduffff/flyto/utils"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

var portTcpServerMap = &utils.Map[string, *LocalTcpServer]{}

type LocalTcpServer struct {
	ServerPort        string
	listener          net.Listener
	ForwardClientList []*ForwardTcpClient
	lock              sync.RWMutex
	initialized       bool
	common.Component
}

func NewLocalTcpServer(clientId string, clientInfo *ClientInfo, sw *YamuxSessionWrapper) (*LocalTcpServer, error) {
	clientPort, err := strconv.Atoi(clientInfo.GetClientPort())
	if err != nil {
		return nil, err
	}
	f := &ForwardTcpClient{
		ClientId:    clientId,
		ClientIp:    clientInfo.GetClientIp(),
		ClientPort:  clientPort,
		NetworkType: common.TCP,
		session:     sw,
	}
	serverPort := clientInfo.GetServerPort()
	tcpServer, ok := portTcpServerMap.Get(serverPort)
	if !ok {
		tcpServer = &LocalTcpServer{
			ServerPort: clientInfo.GetServerPort(),
		}
		if ok := portTcpServerMap.Put(serverPort, tcpServer); !ok {
			// 此时可能是另一个协程已经创建了这个端口的LocalTcpServer
			tcpServer, _ = portTcpServerMap.Get(serverPort)
		}
	}
	// 关闭session
	go func(tcpServer *LocalTcpServer) {
		<-sw.session.CloseChan()
		for _, client := range tcpServer.ForwardClientList {
			if client.session.id != sw.id {
				continue
			}
			tcpServer.RemoveForwardClient(client)
		}
	}(tcpServer)
	tcpServer.AddForwardClient(f)
	return tcpServer, nil
}

func (s *LocalTcpServer) Start() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.initialized {
		return nil
	}
	listener, err := net.Listen("tcp", ":"+s.ServerPort)
	if err != nil {
		return err
	}
	s.listener = listener
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				listener.Close()
				log.Printf("Error accepting connection: %v", err)
				if conn != nil {
					conn.Close()
				}
				break
			}
			go s.HandleTcpConnection(conn)
		}
	}()
	s.initialized = true
	return nil
}

func (s *LocalTcpServer) HandleTcpConnection(conn net.Conn) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(s.ForwardClientList))
	client := s.ForwardClientList[index]
	client.HandleTcpConnection(conn)
}

func (s *LocalTcpServer) AddForwardClient(client *ForwardTcpClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.ForwardClientList = append(s.ForwardClientList, client)
}

func (s *LocalTcpServer) RemoveForwardClient(client *ForwardTcpClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for i, c := range s.ForwardClientList {
		log.Printf("Removing client %s from server %s\n", c.ClientId, s.ServerPort)
		if c.session.id == client.session.id {
			s.ForwardClientList = append(s.ForwardClientList[:i], s.ForwardClientList[i+1:]...)
			break
		}
	}
	if len(s.ForwardClientList) == 0 {
		s.Close()
	}
}

func (s *LocalTcpServer) Close() error {
	portTcpServerMap.Remove(s.ServerPort)
	if err := s.listener.Close(); err != nil {
		return err
	}
	return nil
}

type ForwardTcpClient struct {
	ClientId    string
	ClientIp    string
	ClientPort  int
	NetworkType string // tcp, udp
	session     *YamuxSessionWrapper
}

func (f *ForwardTcpClient) GetHeader() *bytes.Buffer {
	buf := &bytes.Buffer{}
	/**
	VER: 0x05
	CMD: 0x01 (CONNECT)
	RSV: 0x00
	ATYP: 0x01=IPv4, 0x03=域名, 0x04=IPv6
	DST.ADDR: 目标地址
	DST.PORT: 目标端口(大端)
	*/
	buf.Write([]byte{0x06, 0x01, 0x00, 0x01}) // VER, CMD, RSV, ATYP=domain
	buf.WriteByte(byte(len(f.ClientIp)))      // 域名长度
	buf.WriteString(f.ClientIp)               // 域名内容
	buf.WriteByte(byte(f.ClientPort >> 8))    // 端口高字节
	buf.WriteByte(byte(f.ClientPort & 0xff))  // 端口低字节
	return buf
}

func (f *ForwardTcpClient) HandleTcpConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("捕获panic:", r)
			// 可以做清理、日志、重试等处理
		}
	}()
	session := f.session.session
	stream, err := session.OpenStream()
	if err != nil {
		session.Close()
		conn.Close()
		return
	}
	stream.Write(f.GetHeader().Bytes())
	prConn, pwConn := io.Pipe()
	prStream, pwStream := io.Pipe()

	var once sync.Once
	closeAll := func() {
		conn.Close()
		stream.Close()
		prConn.Close()
		pwConn.Close()
		prStream.Close()
		pwStream.Close()
	}
	// server write to client
	go func() {
		io.Copy(pwConn, conn)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(stream, prConn)
		once.Do(closeAll)
	}()
	// read from client
	go func() {
		io.Copy(pwStream, stream)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(conn, prStream)
		once.Do(closeAll)
	}()
}
