package server

import (
	"bytes"
	"github.com/dukeduffff/flyto/common"
	"github.com/dukeduffff/flyto/utils"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

var portUdpServerMap = &utils.Map[string, *LocalUdpServer]{}

type LocalUdpServer struct {
	ServerPort        string
	udpConn           net.PacketConn
	ForwardClientList []*ForwardUdpClient
	lock              sync.RWMutex
	initialized       bool
	CloseChan         chan struct{}
}

func NewLocalUdpServer(clientId string, clientInfo *ClientInfo, sw *YamuxSessionWrapper) (*LocalUdpServer, error) {
	clientPort, err := strconv.Atoi(clientInfo.GetClientPort())
	if err != nil {
		return nil, err
	}
	f := &ForwardUdpClient{
		ClientId:    clientId,
		ClientIp:    clientInfo.GetClientIp(),
		ClientPort:  clientPort,
		NetworkType: common.UDP,
		session:     sw,
	}
	serverPort := clientInfo.GetServerPort()
	udpServer, ok := portUdpServerMap.Get(serverPort)
	if !ok {
		udpServer = &LocalUdpServer{
			ServerPort: clientInfo.GetServerPort(),
			CloseChan:  make(chan struct{}, 1),
		}
		if ok := portUdpServerMap.Put(serverPort, udpServer); !ok {
			// 此时可能是另一个协程已经创建了这个端口的LocalTcpServer
			udpServer, _ = portUdpServerMap.Get(serverPort)
		}
	}
	// 关闭session
	go func(udpServer *LocalUdpServer) {
		<-sw.session.CloseChan()
		for _, client := range udpServer.ForwardClientList {
			if client.session.id != sw.id {
				continue
			}
			udpServer.RemoveForwardClient(client)
		}
	}(udpServer)
	udpServer.AddForwardClient(f)
	return udpServer, nil
}

func (s *LocalUdpServer) Start() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.initialized {
		return nil
	}
	conn, err := net.ListenPacket("udp", ":"+s.ServerPort)
	if err != nil {
		return err
	}
	s.udpConn = conn
	go func() {
		buffer := make([]byte, 65507)
	loop:
		for {
			select {
			case <-s.CloseChan:
				break loop
			default:
			}
			n, addr, err := conn.ReadFrom(buffer)
			if err != nil {
				log.Printf("Error reading from UDP connection: %v\n", err)
				continue
			}
			data := buffer[:n]
			go s.HandleUdpData(conn, data, addr)
		}
	}()
	s.initialized = true
	return nil
}

func (s *LocalUdpServer) HandleUdpData(conn net.PacketConn, data []byte, addr net.Addr) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(s.ForwardClientList))
	client := s.ForwardClientList[index]
	client.HandleUdpConnection(conn, data, addr)
}

func (s *LocalUdpServer) AddForwardClient(client *ForwardUdpClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.ForwardClientList = append(s.ForwardClientList, client)
}

func (s *LocalUdpServer) RemoveForwardClient(client *ForwardUdpClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.ForwardClientList) == 0 {
		return
	}
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

func (s *LocalUdpServer) Close() error {
	s.CloseChan <- struct{}{}
	portUdpServerMap.Remove(s.ServerPort)
	if err := s.udpConn.Close(); err != nil {
		return err
	}
	return nil
}

type ForwardUdpClient struct {
	ClientId    string
	ClientIp    string
	ClientPort  int
	NetworkType string // tcp, udp
	session     *YamuxSessionWrapper
}

func (f *ForwardUdpClient) GetHeader() *bytes.Buffer {
	buf := &bytes.Buffer{}
	/**
	VER: 0x05
	CMD: 0x01 (CONNECT)
	RSV: 0x00
	ATYP: 0x01=IPv4, 0x03=域名, 0x04=IPv6
	DST.ADDR: 目标地址
	DST.PORT: 目标端口(大端)
	*/
	buf.Write([]byte{0x06, 0x01, 0x01, 0x01}) // VER, CMD, RSV, ATYP=domain
	buf.WriteByte(byte(len(f.ClientIp)))      // 域名长度
	buf.WriteString(f.ClientIp)               // 域名内容
	buf.WriteByte(byte(f.ClientPort >> 8))    // 端口高字节
	buf.WriteByte(byte(f.ClientPort & 0xff))  // 端口低字节
	return buf
}

func (f *ForwardUdpClient) HandleUdpConnection(conn net.PacketConn, data []byte, addr net.Addr) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("捕获panic:", r)
			// 可以做清理、日志、重试等处理
		}
	}()
	session := f.session.session
	stream, err := session.OpenStream()
	if err != nil {
		stream.Close()
		return
	}
	err = stream.SetDeadline(time.Now().Add(time.Second * 5))
	if err != nil {
		stream.Close()
		return
	}
	var once sync.Once
	closeAll := func() {
		stream.Close()
	}
	go func() {
		buffer := make([]byte, 65507)
		for {
			n, err2 := stream.Read(buffer)
			if err2 != nil {
				log.Printf("Error reading udp from tcp connection: %v\n", err2)
				break
			}
			conn.WriteTo(buffer[:n], addr)
		}
		once.Do(closeAll)
	}()
	if _, err := stream.Write(f.GetHeader().Bytes()); err != nil {
		once.Do(closeAll)
	}
	if _, err := stream.Write(data); err != nil {
		once.Do(closeAll)
	}
}
