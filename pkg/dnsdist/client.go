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
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	KEY_LEN   = 32
	NONCE_LEN = 24
)

type clientOption func(c *Client) error

type Client struct {
	conn    net.Conn //raw connection to configured Host and Port
	key     [KEY_LEN]byte
	host    net.IP
	port    string
	timeout time.Duration
	retries int
	cNonce  [NONCE_LEN]byte //ClientNonce
	sNonce  [NONCE_LEN]byte //ServerNonce
	wNonce  [NONCE_LEN]byte //WriteNonce
	rNonce  [NONCE_LEN]byte //ReadNonce
}

func NewClient(key string, options ...clientOption) (*Client, error) {
	client := &Client{ // init default values
		host:    net.ParseIP("127.0.0.1"),
		port:    "5199",
		timeout: time.Second * 30,
		retries: 1,
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

	if err := client.generateClientNonce(); err != nil {
		return nil, err
	}

	for _, opt := range options {
		err := opt(client)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func WithHost(host string) clientOption {
	return func(c *Client) error {
		ip := net.ParseIP(host)
		if ip == nil {
			return ErrCouldNotParseAddr
		}
		c.host = ip
		return nil
	}
}

func WithPort(port string) clientOption {
	return func(c *Client) error {
		port = strings.TrimSpace(port)
		if port == "" {
			return ErrCouldNotParseAddr
		}
		// Ensure all characters are digits
		for _, r := range port {
			if r < '0' || r > '9' {
				return ErrCouldNotParseAddr
			}
		}

		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return ErrCouldNotParseAddr
		}

		c.port = port
		return nil
	}
}

func WithTimeout(timeout time.Duration) clientOption {
	return func(c *Client) error {
		c.timeout = timeout
		return nil
	}
}

func WithNumRetriesOnCommandFailure(retries int) clientOption {
	return func(c *Client) error {
		if retries < 0 {
			return ErrNegativeRetryCount
		}
		c.retries = retries
		return nil
	}
}

func (c *Client) generateClientNonce() error {
	bufferNonce := make([]byte, NONCE_LEN)
	_, err := rand.Read(bufferNonce) // initialize client nonce
	if err != nil {
		errors.Join(ErrFatalRandRead, err)
	}
	copy(c.cNonce[0:NONCE_LEN], bufferNonce)

	return nil
}

// will reconnect to the server, by closing existing connection,  generating a new client nonce,
// and then trying to reconnect
func (c *Client) reconnect() error {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}

	if err := c.generateClientNonce(); err != nil {
		return err
	}

	return c.connect()
}

// ensures that we have a connection to the server
func (c *Client) ensureConnected() error {
	if c.conn == nil {
		return c.connect()
	}
	return nil
}

// connect does the handshake to initialize the reading and writing nonce
func (c *Client) connect() error {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", c.host.String(), c.port))
	if err != nil {
		return errors.Join(ErrCouldNotParseAddr, err)
	}

	c.conn, err = net.DialTimeout("tcp", addr.String(), c.timeout)
	if err != nil {
		return errors.Join(ErrWhileCreatingConnection, err)
	}

	_, err = c.conn.Write(c.cNonce[:]) // present client nonce
	if err != nil {
		return errors.Join(ErrCouldNotWrite, err)
	}

	buffer := make([]byte, NONCE_LEN)
	readSize, err := c.conn.Read(buffer) // read server nonce
	if err != nil {
		return errors.Join(ErrCouldNotRead, err)
	}

	if readSize != NONCE_LEN {
		// TODO: hehe data is not complete, how to handle?
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

	resp, err := c.command("") // test handshake
	if err != nil {
		return errors.Join(ErrCouldNotSendCommand, err)
	}

	if resp != "" {
		return fmt.Errorf("%w: got response: %v", ErrHandShakeNotValid, resp)
	}

	return nil
}

func (c *Client) Disconnect() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
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

func (c *Client) command(cmd string) (string, error) {
	if err := c.ensureConnected(); err != nil {
		return "", err
	}

	response, err := c.sendCommand(cmd)

	attempts := 0
	for err != nil && attempts < c.retries {
		attempts++
		if recErr := c.reconnect(); recErr != nil {
			err = errors.Join(err, recErr)
			continue
		}
		response, err = c.sendCommand(cmd)
	}

	if err != nil {
		return "", err
	}

	return response, nil
}

func (c *Client) sendCommand(cmd string) (response string, err error) {
	encoded := c.encrypt(cmd)

	bufferLen := make([]byte, 4)
	binary.BigEndian.PutUint32(bufferLen, uint32(len(encoded)))

	_, err = c.conn.Write(bufferLen) // write the length of the command
	if err != nil {
		return "", errors.Join(ErrCouldNotWrite, err)
	}

	_, err = c.conn.Write(encoded) // write command
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

func (c *Client) AddDomainSpoof(domain string, ips []string) error {
	// addAction(QNameRule('example.com'), SpoofAction({"192.168.1.0","192.168.1.2"}), {name="example.com"})
	cmd := fmt.Sprintf("addAction(QNameRule('%v'), SpoofAction({", domain)

	for _, ip := range ips {
		cmd += fmt.Sprintf("'%v', ", ip)
	}
	idx := strings.LastIndex(cmd, ",")
	if idx == -1 {
		return fmt.Errorf("no trailing comma found in command: %s", cmd)
	}
	cmd = fmt.Sprintf("%v {name='%v'})", cmd[:idx]+"}),", domain)

	return Must(c.command(cmd))
}

func (c *Client) RmDomainSpoof(domain string) error {
	return Must(c.command(fmt.Sprintf("rmRule(%s)", domain)))
}
