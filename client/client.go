package client

import (
	"context"
	"fmt"
	"github.com/dukeduffff/flyto/common"
	"github.com/dukeduffff/flyto/server"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Client struct {
	c *common.Config
}

func (c *Client) Start() error {
	for {
		conn, err := net.Dial("tcp", c.c.RemoteHostAndPort)
		if err != nil {
			return fmt.Errorf("dial error: %w", err)
		}
		conn, err = common.NewCipherConn(conn, c.c.Key)
		if err != nil {
			return fmt.Errorf("cipher conn error: %w", err)
		}
		session, err := yamux.Client(conn, nil)
		if err != nil {
			return fmt.Errorf("yamux client error: %w", err)
		}
		c.doStart(session)
		log.Printf("last connect to %s lost, waiting for 5 seconds before reconnecting...\n", c.c.RemoteHostAndPort)
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (c *Client) doStart(session *yamux.Session) error {
	stream, err := session.OpenStream()
	// 1.use grpc register info
	dialOption := grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return stream, nil
	})
	credentialsOption := grpc.WithTransportCredentials(insecure.NewCredentials())
	clientConn, err := grpc.NewClient("127.0.0.1", dialOption, credentialsOption)
	client := server.NewFlyToServiceClient(clientConn)
	infos := make([]*server.ClientInfo, 0)
	for _, addrStr := range c.c.LocalHostAndPort {
		split := strings.Split(addrStr, ":")
		if len(split) != 3 && len(split) != 4 {
			return fmt.Errorf("invalid local host and port: %s", addrStr)
		}
		tmpNetworkType := server.NetworkType_NETWORK_TYPE_TCP
		begin := 0
		if len(split) == 4 {
			if strings.HasPrefix(split[begin+0], "udp") {
				tmpNetworkType = server.NetworkType_NETWORK_TYPE_UDP
			}
			begin += 1
		}
		tmpClientIp := strings.ReplaceAll(split[begin+0], "//", "")
		tmpClientPort := split[begin+1]
		tmpServerPort := split[begin+2]
		infos = append(infos, &server.ClientInfo{ServerPort: tmpServerPort, ClientIp: tmpClientIp, ClientPort: tmpClientPort, NetworkType: tmpNetworkType})
	}

	request := &server.RegisterRequest{ClientId: "default", CipherKey: c.c.Key, ClientInfos: infos}
	response, err := client.Register(context.Background(), request)
	if err != nil || !response.GetStatus() {
		return fmt.Errorf("register error: %w, status=%v", err, response.GetStatus())
	}
	// ping
	go func() {
		for {
			pingRequest := &server.PingRequest{ClientId: "default"}
			client.Ping(context.Background(), pingRequest)
			time.Sleep(time.Second * 30)
		}
	}()
	// 2. accept new stream
	c.handleSession(session)
	return nil
}

func (c *Client) handleSession(session *yamux.Session) {
	for {
		stream, err := session.Accept()
		if err != nil {
			log.Println("accept error:", err)
			break
		}
		go func() {
			err = c.handleStream(stream)
			if err != nil {
				log.Println("handle stream error:", err)
			}
		}()
	}
}

func (c *Client) handleStream(stream net.Conn) error {
	ver, _, networkType, atyp, domain, port, err := parseSocksRequestFrom(stream)
	if err != nil {
		return fmt.Errorf("parse socks request error: %w", err)
	}
	if ver != 0x06 {
		return fmt.Errorf("invalid version: %d", ver)
	}
	if atyp != 0x01 {
		return fmt.Errorf("invalid type: %d, currently only support 0x01", atyp)
	}
	//log.Printf("domain: %s, port: %d\n", domain, port)
	if networkType == 0x00 {
		return c.handleTcp(domain, port, stream)
	} else if networkType == 0x01 {
		return c.handleUdp(domain, port, stream)
	}
	return nil
}

func (c *Client) handleTcp(domain string, port uint16, stream net.Conn) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", domain, port))
	if err != nil {
		return err
	}
	readStream, writeConn := io.Pipe()
	readConn, writeStream := io.Pipe()
	var once sync.Once
	closeAll := func() {
		conn.Close()
		stream.Close()
		readStream.Close()
		writeConn.Close()
		readConn.Close()
		writeStream.Close()
	}
	go func() {
		io.Copy(writeConn, stream)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(conn, readStream)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(writeStream, conn)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(stream, readConn)
		once.Do(closeAll)
	}()
	return nil
}

func parseSocksRequestFrom(r io.Reader) (ver, cmd, rsv, atyp byte, domain string, port uint16, err error) {
	header := make([]byte, 5)
	if _, err = io.ReadFull(r, header); err != nil {
		return
	}
	ver = header[0]
	cmd = header[1]
	rsv = header[2]
	atyp = header[3]
	domainLen := int(header[4])
	domainBytes := make([]byte, domainLen)
	if _, err = io.ReadFull(r, domainBytes); err != nil {
		return
	}
	domain = string(domainBytes)

	portBytes := make([]byte, 2)
	if _, err = io.ReadFull(r, portBytes); err != nil {
		return
	}
	port = uint16(portBytes[0])<<8 | uint16(portBytes[1])
	return
}

func (c *Client) handleUdp(domain string, port uint16, stream net.Conn) error {
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", domain, port))
	if err != nil {
		return err
	}
	readStream, writeConn := io.Pipe()
	readConn, writeStream := io.Pipe()
	var once sync.Once
	closeAll := func() {
		conn.Close()
		stream.Close()
		readStream.Close()
		writeConn.Close()
		readConn.Close()
		writeStream.Close()
	}
	go func() {
		io.Copy(writeConn, stream)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(conn, readStream)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(writeStream, conn)
		once.Do(closeAll)
	}()
	go func() {
		io.Copy(stream, readConn)
		once.Do(closeAll)
	}()
	return nil
}

func NewClient(config *common.Config) *Client {
	return &Client{
		c: config,
	}
}
