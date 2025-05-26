package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

type PortTcpServerMap struct {
	portTcpServerMap map[string]*LocalTcpServer
	lock             sync.RWMutex
}

var portTcpServerMap = &PortTcpServerMap{portTcpServerMap: make(map[string]*LocalTcpServer), lock: sync.RWMutex{}}

func (p *PortTcpServerMap) Get(port string) *LocalTcpServer {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if localTcpServer, ok := p.portTcpServerMap[port]; ok {
		return localTcpServer
	}
	return nil
}

func (p *PortTcpServerMap) Put(port string, tcpServer *LocalTcpServer) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.portTcpServerMap[port]; ok {
		log.Printf("Port %s already exists, closing old listener", port)
		return false
	}
	p.portTcpServerMap[port] = tcpServer
	return true
}

func (p *PortTcpServerMap) Remove(port string) *LocalTcpServer {
	p.lock.Lock()
	defer p.lock.Unlock()
	if tcpServer, ok := p.portTcpServerMap[port]; ok {
		delete(p.portTcpServerMap, port)
		return tcpServer
	}
	return nil
}

type LocalTcpServer struct {
	ServerPort        string
	listener          net.Listener
	ForwardClientList []*ForwardClient
	lock              sync.RWMutex
	initialized       bool
}

func NewLocalServer(clientId string, clientInfo *ClientInfo, sw *YamuxSessionWrapper) (*LocalTcpServer, error) {
	clientPort, err := strconv.Atoi(clientInfo.GetClientPort())
	if err != nil {
		return nil, err
	}
	f := &ForwardClient{
		ClientId:   clientId,
		ClientIp:   clientInfo.GetClientIp(),
		ClientPort: clientPort,
		session:    sw,
	}
	tcpServer := portTcpServerMap.Get(clientInfo.ServerPort)
	if tcpServer == nil {
		tcpServer = &LocalTcpServer{
			ServerPort: clientInfo.GetServerPort(),
		}
		if ok := portTcpServerMap.Put(clientInfo.GetServerPort(), tcpServer); !ok {
			// 此时可能是另一个协程已经创建了这个端口的LocalTcpServer
			tcpServer = portTcpServerMap.Get(clientInfo.GetServerPort())
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

func (s *LocalTcpServer) AddForwardClient(client *ForwardClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.ForwardClientList = append(s.ForwardClientList, client)
}

func (s *LocalTcpServer) RemoveForwardClient(client *ForwardClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for i, c := range s.ForwardClientList {
		log.Printf("Removing client %s from server %s\n", c.ClientId, s.ServerPort)
		if c.session.id == client.session.id {
			s.ForwardClientList = append(s.ForwardClientList[:i], s.ForwardClientList[i+1:]...)
			break
		}
	}
}

func (s *LocalTcpServer) Close() error {
	if err := s.listener.Close(); err != nil {
		return err
	}
	return nil
}

type ForwardClient struct {
	ClientId   string
	ClientIp   string
	ClientPort int
	session    *YamuxSessionWrapper
}

func (f *ForwardClient) GetHeader() *bytes.Buffer {
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

func (f *ForwardClient) HandleTcpConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("捕获panic:", r)
			// 可以做清理、日志、重试等处理
		}
	}()
	session := f.session.session
	stream, err := session.OpenStream()
	if err != nil {
		session.Close()
		conn.Close()
		//if lis := portListenMap.Remove(s.ServerPort); lis != nil {
		//	lis.Close()
		//}
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
