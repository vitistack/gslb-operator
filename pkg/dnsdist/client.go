/*
This package is based on: https://github.com/dmachard/python-dnsdist-console/blob/master/dnsdist_console/console.py
And: https://github.com/PowerDNS/go-dnsdist-client/blob/master/dnsdist/client.go
Creds to the developers of these projects, as it is the foundation of this package
*/
package dnsdist

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	KEY_LEN   = 32
	NONCE_LEN = 24
)

type Client struct {
	conn    net.Conn //raw connection to configured Host and Port
	key     [KEY_LEN]byte
	Host    string
	Port    string
	timeout time.Duration
	cNonce  [NONCE_LEN]byte //ClientNonce
	sNonce  [NONCE_LEN]byte //ServerNonce
	wNonce  [NONCE_LEN]byte //WriteNonce
	rNonce  [NONCE_LEN]byte //ReadNonce
}

func NewClient(key, host, port string, timeout time.Duration) (*Client, error) {
	client := &Client{
		Host:    host,
		Port:    port,
		timeout: timeout,
	}

	xKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, errors.Join(ErrUnSuccessFullBase64Decode, err)
	}
	xKeyLen := len(xKey)
	if xKeyLen != KEY_LEN {
		return nil, errors.Join(ErrUnSuccessFullBase64Decode, fmt.Errorf("expected key length: %v, but got: %v", KEY_LEN, xKeyLen))
	}
	copy(client.key[0:KEY_LEN], xKey)

	bufferNonce := make([]byte, NONCE_LEN)
	_, err = rand.Read(bufferNonce) // initialize client nonce
	if err != nil {
		errors.Join(ErrFatalRandRead, err)
	}
	copy(client.cNonce[0:NONCE_LEN], bufferNonce)

	return client, client.connect()
}

func (c *Client) connect() error {
	ip := net.ParseIP(c.Host)
	if ip == nil {
		return ErrCouldNotParseAddr
	}
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", ip.String(), c.Port))
	if err != nil {
		return errors.Join(ErrCouldNotParseAddr, err)
	}

	c.conn, err = net.Dial("tcp", addr.String())
	if err != nil {
		return errors.Join(ErrWhileCreatingConnection, err)
	}

	_, err = c.conn.Write(c.cNonce[:])
	if err != nil {
		return errors.Join(ErrCouldNotWrite, err)
	}

	buffer := make([]byte, NONCE_LEN)
	readSize, err := c.conn.Read(buffer)
	if err != nil {
		return errors.Join(ErrCouldNotRead, err)
	}

	if readSize != NONCE_LEN {
		// hehe data is not complete, how to handle this shit?
		return errors.Join(ErrReceivedInvalidNonce, fmt.Errorf("expected length: %v, but got: %v", NONCE_LEN, readSize))
	}
	copy(c.sNonce[:], buffer)

	sNonceLen := len(c.sNonce)
	cNonceLen := len(c.cNonce)
	if sNonceLen != cNonceLen {
		return errors.Join(ErrInvalidNoncePair, fmt.Errorf("length of nonces differ, client: %v, server: %v", cNonceLen, sNonceLen))
	}

	halfNonce := NONCE_LEN / 2
	//initialize reading nonce
	copy(c.rNonce[:halfNonce], c.cNonce[:halfNonce])
	copy(c.rNonce[halfNonce:], c.sNonce[halfNonce:])

	//initialize writing nonce
	copy(c.wNonce[:halfNonce], c.sNonce[:halfNonce])
	copy(c.wNonce[halfNonce:], c.cNonce[halfNonce:])

	resp, err := c.Command("")
	if err != nil {
		return errors.Join(ErrCouldNotSendCommand, err)
	}

	if resp != "" {
		return fmt.Errorf("%w: got: %v", ErrHandShakeNotValid, resp)
	}

	return nil
}

func (c *Client) Disconnect() error {
	return c.conn.Close()
}

func (c *Client) encrypt(cmd string) []byte {
	// Encrypt using secretbox (NaCl)
	cmdBytes := []byte(cmd)
	encrypted := secretbox.Seal(nil, cmdBytes, &c.wNonce, &c.key)

	// Increment write nonce for next message
	incrementNonce(&c.wNonce)

	return encrypted
}

func (c *Client) decrypt(data []byte) (string, bool) {
	// Decrypt using secretbox (NaCl)
	decrypted, ok := secretbox.Open(nil, data, &c.rNonce, &c.key)
	if !ok {
		return "", false
	}

	// Increment read nonce for next message
	incrementNonce(&c.rNonce)

	return string(decrypted), true
}

func (c *Client) Command(cmd string) (response string, err error) {
	encoded := c.encrypt(cmd)

	bufferLen := make([]byte, 4)
	binary.BigEndian.PutUint32(bufferLen, uint32(len(encoded)))

	_, err = c.conn.Write(bufferLen)
	if err != nil {
		return "", errors.Join(ErrCouldNotWrite, err)
	}

	_, err = c.conn.Write(encoded)
	if err != nil {
		return "", errors.Join(ErrCouldNotWrite, err)
	}

	_, err = io.ReadFull(c.conn, bufferLen)
	if err != nil {
		return "", errors.Join(ErrCouldNotRead, err)
	}

	receiveLenBuffer := binary.BigEndian.Uint32(bufferLen)
	receiveBuffer := make([]byte, receiveLenBuffer)
	_, err = io.ReadFull(c.conn, receiveBuffer)
	if err != nil {
		return "", errors.Join(ErrCouldNotRead, err)
	}

	decrypted, ok := c.decrypt(receiveBuffer)
	if !ok {
		return "", fmt.Errorf("%w: got: %v", ErrCouldNotDecrypt, decrypted)
	}

	return decrypted, nil
}

func incrementNonce(nonce *[NONCE_LEN]byte) {
	value := binary.BigEndian.Uint32(nonce[:4])
	value++
	binary.BigEndian.PutUint32(nonce[:4], value)
}
