package tcp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport"
	"golang.org/x/crypto/nacl/secretbox"
)

type tcpTransport struct {
	conn     net.Conn
	key      [transport.KEY_LEN]byte
	hostIP   net.IP
	hostPort string
	timeout  time.Duration
	retries  int
	cNonce   [transport.NONCE_LEN]byte //ClientNonce
	sNonce   [transport.NONCE_LEN]byte //ServerNonce
	wNonce   [transport.NONCE_LEN]byte //WriteNonce
	rNonce   [transport.NONCE_LEN]byte //ReadNonce
}

func NewTCPTransport(key string, opts ...tcpTransportOption) (*tcpTransport, error) {
	tcpTransport := &tcpTransport{ // default transport values
		hostIP:   net.ParseIP("127.0.0.1"),
		hostPort: "5199",
		timeout:  time.Second * 30,
		retries:  1,
	}

	xKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}
	xKeyLen := len(xKey)
	if xKeyLen != transport.KEY_LEN {
		return nil, fmt.Errorf("could not verify key: expected key-length of :%d but got: %d", transport.KEY_LEN, xKeyLen)
	}
	copy(tcpTransport.key[0:transport.KEY_LEN], xKey)

	if err := tcpTransport.generateClientNonce(); err != nil {
		return nil, fmt.Errorf("failed to generate client nonce: %w", err)
	}

	for _, opt := range opts {
		err := opt(tcpTransport)
		if err != nil {
			return nil, fmt.Errorf("invalid options: %w", err)
		}
	}

	return tcpTransport, nil
}

func (t *tcpTransport) generateClientNonce() error {
	bufferNonce := make([]byte, transport.NONCE_LEN)
	_, err := rand.Read(bufferNonce) // initialize client nonce
	if err != nil {
		return err
	}
	copy(t.cNonce[0:transport.NONCE_LEN], bufferNonce)

	return nil
}

// connect does the handshake to initialize the reading and writing nonce
func (t *tcpTransport) connect() error {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:%v", t.hostIP.String(), t.hostPort))
	if err != nil {
		return fmt.Errorf("failed to resolve tcp address: %w", err)
	}

	t.conn, err = net.DialTimeout("tcp", addr.String(), t.timeout)
	if err != nil {
		return fmt.Errorf("failed to establish tcp connection: %w", err)
	}

	_, err = t.conn.Write(t.cNonce[:]) // present client nonce
	if err != nil {
		return fmt.Errorf("failed to present client nonce: %w", err)
	}

	buffer := make([]byte, transport.NONCE_LEN)
	readSize, err := t.conn.Read(buffer) // read server nonce
	if err != nil {
		return fmt.Errorf("failed to read server nonce: %w", err)
	}

	if readSize != transport.NONCE_LEN {
		// TODO: hehe data is not complete, how to handle?
		return fmt.Errorf("invalid server nonce: expected length: %d, but got: %d", transport.NONCE_LEN, readSize)
	}
	copy(t.sNonce[:], buffer)

	sNonceLen := len(t.sNonce)
	cNonceLen := len(t.cNonce)
	if sNonceLen != cNonceLen {
		return fmt.Errorf("invalid nonce client/server pair: length of nonce differ, client: %d, server: %d", cNonceLen, sNonceLen)
	}

	halfNonce := transport.NONCE_LEN / 2
	//initialize reading nonce
	copy(t.rNonce[:halfNonce], t.cNonce[:halfNonce])
	copy(t.rNonce[halfNonce:], t.sNonce[halfNonce:])

	//initialize writing nonce
	copy(t.wNonce[:halfNonce], t.sNonce[:halfNonce])
	copy(t.wNonce[halfNonce:], t.cNonce[halfNonce:])

	resp, err := t.Execute("") // test handshake
	if err != nil {
		return fmt.Errorf("dnsdist controllsocket handshake: failed to execute handshake command: %w", err)
	}

	if resp != "" {
		return fmt.Errorf("dnsdist controllsocket handshake failed: got response: %s", resp)
	}

	return nil
}

func (t *tcpTransport) ensureConnected() error {
	if t.conn == nil {
		return t.connect()
	}
	return nil
}

func (t *tcpTransport) reconnect() error {
	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}

	if err := t.generateClientNonce(); err != nil {
		return err
	}

	return t.connect()
}

func (t *tcpTransport) Execute(cmd string) (string, error) {
	return t.command(cmd)
}

func (t *tcpTransport) command(cmd string) (string, error) {
	if err := t.ensureConnected(); err != nil {
		return "", err
	}

	response, err := t.sendCommand(cmd)

	attempts := 0
	for err != nil && attempts < t.retries {
		attempts++
		if recErr := t.reconnect(); recErr != nil {
			err = errors.Join(err, recErr)
			continue
		}
		response, err = t.sendCommand(cmd)
	}

	if err != nil {
		return "", err
	}

	return response, nil
}

func (t *tcpTransport) sendCommand(cmd string) (response string, err error) {
	encoded := t.encrypt(cmd)

	bufferLen := make([]byte, 4)
	binary.BigEndian.PutUint32(bufferLen, uint32(len(encoded)))

	_, err = t.conn.Write(bufferLen) // write the length of the command
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	_, err = t.conn.Write(encoded) // write command
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	_, err = io.ReadFull(t.conn, bufferLen)
	if err != nil {
		return "", fmt.Errorf("failed to read dnsdist-server response: %w", err)
	}

	receiveLenBuffer := binary.BigEndian.Uint32(bufferLen)
	receiveBuffer := make([]byte, receiveLenBuffer)
	_, err = io.ReadFull(t.conn, receiveBuffer)
	if err != nil {
		return "", fmt.Errorf("failed to read dnsdist-server response: %w", err)
	}

	decrypted, ok := t.decrypt(receiveBuffer)
	if !ok {
		return "", fmt.Errorf("failed to decrypt response: got: %s", decrypted)
	}

	return decrypted, nil
}

func (t *tcpTransport) encrypt(cmd string) []byte {
	// Encrypt using secretbox (NaCl)
	cmdBytes := []byte(cmd)
	encrypted := secretbox.Seal(nil, cmdBytes, &t.wNonce, &t.key)

	// Increment write nonce for next message
	incrementNonce(&t.wNonce)

	return encrypted
}

func (t *tcpTransport) decrypt(data []byte) (string, bool) {
	// Decrypt using secretbox (NaCl)
	decrypted, ok := secretbox.Open(nil, data, &t.rNonce, &t.key)
	if !ok {
		return "", false
	}

	// Increment read nonce for next message
	incrementNonce(&t.rNonce)

	return string(decrypted), true
}

func (t *tcpTransport) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

func incrementNonce(nonce *[transport.NONCE_LEN]byte) {
	value := binary.BigEndian.Uint32(nonce[:4])
	value++
	binary.BigEndian.PutUint32(nonce[:4], value)
}
