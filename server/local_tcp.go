package server

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/hashicorp/yamux"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
)

type PortListenMap struct {
	portListenMap map[string]net.Listener
	lock          sync.RWMutex
}

var portListenMap = &PortListenMap{portListenMap: make(map[string]net.Listener), lock: sync.RWMutex{}}

func (p *PortListenMap) Get(port string) (net.Listener, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if listener, ok := p.portListenMap[port]; ok {
		return listener, nil
	}
	return nil, nil
}

func (p *PortListenMap) Put(port string, listener net.Listener) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.portListenMap[port]; ok {
		log.Printf("Port %s already exists, closing old listener", port)
		p.portListenMap[port].Close()
	}
	p.portListenMap[port] = listener
}

func (p *PortListenMap) Remove(port string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if listener, ok := p.portListenMap[port]; ok {
		listener.Close()
		delete(p.portListenMap, port)
	}
}

type LocalTcpServer struct {
	ClientId   string
	ServerPort string
	ClientIp   string
	ClientPort int
	listener   net.Listener
	session    *yamux.Session
}

func NewLocalServer(clientId string, clientInfo *ClientInfo, session *yamux.Session) (*LocalTcpServer, error) {
	port, err := strconv.Atoi(clientInfo.ClientPort)
	if err != nil {
		return nil, err
	}
	return &LocalTcpServer{
		ClientId:   clientId,
		ServerPort: clientInfo.ServerPort,
		ClientIp:   clientInfo.ClientIp,
		ClientPort: port,
		session:    session,
	}, nil
}

func (s *LocalTcpServer) Start() error {
	listener, err := portListenMap.Get(s.ServerPort)
	if err != nil {
		log.Printf("Error starting local tcp server: %v", err)
		return err
	}
	if listener == nil {
		listener, err = net.Listen("tcp", ":"+s.ServerPort)
		if err != nil {
			return err
		}
		portListenMap.Put(s.ServerPort, listener)
	}
	s.listener = listener
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting connection: %v", err)
				if errors.Is(err, net.ErrClosed) {
					break
				}
				continue
			}
			go s.HandleTcpConnection(conn)
		}
	}()
	return nil
}

func (s *LocalTcpServer) Close() error {
	if err := s.listener.Close(); err != nil {
		return err
	}
	return nil
}

func (s *LocalTcpServer) GetHeader() *bytes.Buffer {
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
	buf.WriteByte(byte(len(s.ClientIp)))      // 域名长度
	buf.WriteString(s.ClientIp)               // 域名内容
	buf.WriteByte(byte(s.ClientPort >> 8))    // 端口高字节
	buf.WriteByte(byte(s.ClientPort & 0xff))  // 端口低字节
	return buf
}

func (s *LocalTcpServer) HandleTcpConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("捕获panic:", r)
			// 可以做清理、日志、重试等处理
		}
	}()
	stream, err := s.session.OpenStream()
	if err != nil {
		s.session.Close()
		portListenMap.Remove(s.ServerPort)
		return
	}
	stream.Write(s.GetHeader().Bytes())
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
