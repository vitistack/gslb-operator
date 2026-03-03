package dnsdist

import (
    "crypto/rand"
    "encoding/binary"
    "fmt"
    "io"
    "net"
    "sync"
    "testing"

    "golang.org/x/crypto/nacl/secretbox"
)

// MockServer simulates a dnsdist console server for testing
type MockServer struct {
    listener net.Listener
    key      [KEY_LEN]byte
    addr     string
    handlers map[string]func(string) string // command -> response handler
    mu       sync.RWMutex
    running  bool
    wg       sync.WaitGroup
}

// NewMockServer creates a new mock dnsdist server with the given key
func NewMockServer(t *testing.T, key [KEY_LEN]byte) *MockServer {
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("failed to create mock server: %v", err)
    }

    ms := &MockServer{
        listener: listener,
        key:      key,
        addr:     listener.Addr().String(),
        handlers: make(map[string]func(string) string),
    }

    // Set default handlers
    ms.SetHandler("", func(cmd string) string { return "" }) // empty command for handshake
    ms.SetHandler("showRules()", func(cmd string) string { return "Rules:\n" })

    return ms
}

// Start begins accepting connections
func (ms *MockServer) Start() {
    ms.mu.Lock()
    ms.running = true
    ms.mu.Unlock()

    ms.wg.Add(1)
    go ms.acceptLoop()
}

// Stop stops the server and closes all connections
func (ms *MockServer) Stop() {
    ms.mu.Lock()
    ms.running = false
    ms.mu.Unlock()

    ms.listener.Close()
    ms.wg.Wait()
}

// Addr returns the server's address
func (ms *MockServer) Addr() string {
    return ms.addr
}

// SetHandler sets a response handler for a specific command
func (ms *MockServer) SetHandler(cmd string, handler func(string) string) {
    ms.mu.Lock()
    defer ms.mu.Unlock()
    ms.handlers[cmd] = handler
}

func (ms *MockServer) acceptLoop() {
    defer ms.wg.Done()

    for {
        conn, err := ms.listener.Accept()
        if err != nil {
            ms.mu.RLock()
            running := ms.running
            ms.mu.RUnlock()
            if !running {
                return
            }
            continue
        }

        ms.wg.Add(1)
        go ms.handleConnection(conn)
    }
}

func (ms *MockServer) handleConnection(conn net.Conn) {
    defer ms.wg.Done()
    defer conn.Close()

    // Read client nonce
    cNonce := make([]byte, NONCE_LEN)
    _, err := io.ReadFull(conn, cNonce)
    if err != nil {
        return
    }

    // Generate and send server nonce
    sNonce := make([]byte, NONCE_LEN)
    _, err = rand.Read(sNonce)
    if err != nil {
        return
    }

    _, err = conn.Write(sNonce)
    if err != nil {
        return
    }

    // Initialize read/write nonces
    var rNonce, wNonce [NONCE_LEN]byte
    halfNonce := NONCE_LEN / 2

    // Server's read nonce (client's write nonce)
    copy(rNonce[:halfNonce], sNonce[:halfNonce])
    copy(rNonce[halfNonce:], cNonce[halfNonce:])

    // Server's write nonce (client's read nonce)
    copy(wNonce[:halfNonce], cNonce[:halfNonce])
    copy(wNonce[halfNonce:], sNonce[halfNonce:])

    // Handle commands
    for {
        cmd, err := ms.receiveCommand(conn, &rNonce)
        if err != nil {
            return
        }

        response := ms.getResponse(cmd)

        err = ms.sendResponse(conn, response, &wNonce)
        if err != nil {
            return
        }
    }
}

func (ms *MockServer) receiveCommand(conn net.Conn, rNonce *[NONCE_LEN]byte) (string, error) {
    // Read length
    bufferLen := make([]byte, 4)
    _, err := io.ReadFull(conn, bufferLen)
    if err != nil {
        return "", err
    }

    // Read encrypted command
    cmdLen := binary.BigEndian.Uint32(bufferLen)
    encryptedCmd := make([]byte, cmdLen)
    _, err = io.ReadFull(conn, encryptedCmd)
    if err != nil {
        return "", err
    }

    // Decrypt
    decrypted, ok := secretbox.Open(nil, encryptedCmd, rNonce, &ms.key)
    if !ok {
        return "", fmt.Errorf("decryption failed")
    }

    incrementNonce(rNonce)

    return string(decrypted), nil
}

func (ms *MockServer) sendResponse(conn net.Conn, response string, wNonce *[NONCE_LEN]byte) error {
    // Encrypt response
    encrypted := secretbox.Seal(nil, []byte(response), wNonce, &ms.key)
    incrementNonce(wNonce)

    // Send length
    bufferLen := make([]byte, 4)
    binary.BigEndian.PutUint32(bufferLen, uint32(len(encrypted)))
    _, err := conn.Write(bufferLen)
    if err != nil {
        return err
    }

    // Send encrypted response
    _, err = conn.Write(encrypted)
    return err
}

func (ms *MockServer) getResponse(cmd string) string {
    ms.mu.RLock()
    defer ms.mu.RUnlock()

    if handler, ok := ms.handlers[cmd]; ok {
        return handler(cmd)
    }

    // Default: return empty response
    return ""
}