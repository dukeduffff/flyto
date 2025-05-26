package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"time"
)

type CipherConn struct {
	conn       net.Conn
	gcm        cipher.AEAD
	readBuffer []byte
}

func NewCipherConn(conn net.Conn, cipherKey string) (net.Conn, error) {
	if cipherKey == "" {
		return conn, nil
	}
	key, err := base64.StdEncoding.DecodeString(cipherKey)
	if err != nil {
		return nil, err
	}
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}
	return &CipherConn{
		conn: conn,
		gcm:  gcm,
	}, nil
}

func (c *CipherConn) Read(b []byte) (int, error) {
	// 如果缓冲区为空，先读取并解密新消息
	if len(c.readBuffer) == 0 {
		if err := c.readNextMsg(); err != nil {
			return 0, err
		}
	}
	n := copy(b, c.readBuffer)
	c.readBuffer = c.readBuffer[n:]
	return n, nil
}

func (c *CipherConn) readNextMsg() error {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, lenBuf); err != nil {
		return err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	// 再读整个nonce+ciphertext
	encBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(c.conn, encBuf); err != nil {
		return err
	}
	// nonce
	nonceSize := c.gcm.NonceSize()
	nonce, ciphertext := encBuf[:nonceSize], encBuf[nonceSize:]
	// 解密
	plain, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}
	c.readBuffer = plain
	return nil
}

func (c *CipherConn) Write(b []byte) (int, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, err
	}
	ciphertext := c.gcm.Seal(nil, nonce, b, nil)          // 密文含tag，不含nonce
	sendBuf := make([]byte, 4+len(nonce)+len(ciphertext)) // 4 字节长度头
	totalLen := len(nonce) + len(ciphertext)
	binary.BigEndian.PutUint32(sendBuf[:4], uint32(totalLen))
	copy(sendBuf[4:4+len(nonce)], nonce)
	copy(sendBuf[4+len(nonce):], ciphertext)
	// 通过底层连接发送
	_, err := c.conn.Write(sendBuf)
	if err != nil {
		return 0, err
	}
	// 返回写入明文长度（可选）或密文长度
	return len(b), nil
}

func (c *CipherConn) Close() error {
	return c.conn.Close()
}

func (c *CipherConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *CipherConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *CipherConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *CipherConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *CipherConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
