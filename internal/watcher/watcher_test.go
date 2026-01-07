package watcher

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pperesbr/conduit/internal/config"
	"github.com/pperesbr/conduit/internal/manager"
	"github.com/pperesbr/gokit/pkg/tunnel"
	"golang.org/x/crypto/ssh"
)

// TestNew_Success tests the successful creation and initialization of a Watcher using valid configuration and manager instances.
func TestNew_Success(t *testing.T) {
	configPath := createTempConfigFile(t, validConfigContent())

	sshCfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := manager.NewManager(sshCfg)

	w, err := New(configPath, mgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Stop()

	if w == nil {
		t.Fatal("expected watcher to be created")
	}
}

// TestStart_Success verifies that the Watcher starts successfully with a valid configuration and stops without errors.
func TestStart_Success(t *testing.T) {
	configPath := createTempConfigFile(t, validConfigContent())

	sshCfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := manager.NewManager(sshCfg)

	w, _ := New(configPath, mgr)

	err := w.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Stop()
}

// TestStop verifies the behavior of the Stop method, ensuring it stops the watcher without errors during execution.
func TestStop(t *testing.T) {
	configPath := createTempConfigFile(t, validConfigContent())

	sshCfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := manager.NewManager(sshCfg)

	w, _ := New(configPath, mgr)
	_ = w.Start()

	err := w.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestWatcher_DetectsFileChange verifies that the file watcher detects changes in the configuration file and reloads it.
func TestWatcher_DetectsFileChange(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	port := sshServer.Addr().(*net.TCPAddr).Port
	localPort1 := randomPort()
	localPort2 := randomPort()

	initialConfig := fmt.Sprintf(`
ssh:
  user: testuser
  password: testpass
  host: 127.0.0.1
  port: %d

tunnels:
  - name: tunnel1
    remoteHost: 127.0.0.1
    remotePort: 1521
    localPort: %d
`, port, localPort1)

	configPath := createTempConfigFile(t, initialConfig)

	mgr := manager.NewManager(sshCfg)

	w, _ := New(configPath, mgr)
	err := w.Start()
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()
	defer mgr.StopAll()

	time.Sleep(100 * time.Millisecond)

	newConfig := fmt.Sprintf(`
ssh:
  user: testuser
  password: testpass
  host: 127.0.0.1
  port: %d

tunnels:
  - name: tunnel1
    remoteHost: 127.0.0.1
    remotePort: 1521
    localPort: %d
  - name: tunnel2
    remoteHost: 127.0.0.1
    remotePort: 1522
    localPort: %d
`, port, localPort1, localPort2)

	err = os.WriteFile(configPath, []byte(newConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write new config: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("expected 2 tunnels, got %d: %v", len(list), list)
	}
}

// TestWatcher_InvalidConfigKeepsCurrentState verifies that the watcher retains the current state when an invalid config is provided.
func TestWatcher_InvalidConfigKeepsCurrentState(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	port := sshServer.Addr().(*net.TCPAddr).Port
	localPort1 := randomPort()

	initialConfig := fmt.Sprintf(`
ssh:
  user: testuser
  password: testpass
  host: 127.0.0.1
  port: %d

tunnels:
  - name: tunnel1
    remoteHost: 127.0.0.1
    remotePort: 1521
    localPort: %d
`, port, localPort1)

	configPath := createTempConfigFile(t, initialConfig)

	mgr := manager.NewManager(sshCfg)
	mgr.Add(config.TunnelConfig{Name: "tunnel1", RemoteHost: "127.0.0.1", RemotePort: 1521, LocalPort: localPort1})

	w, _ := New(configPath, mgr)
	_ = w.Start()
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	invalidConfig := `
ssh:
  user: testuser

tunnels: []
`
	err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	list := mgr.List()
	if len(list) != 1 {
		t.Errorf("expected 1 tunnel (unchanged), got %d", len(list))
	}
}

// TestWatcher_DetectsFileRemoveAndRecreate verifies that the file watcher detects file removal and recreation, accurately reloading configuration.
func TestWatcher_DetectsFileRemoveAndRecreate(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	port := sshServer.Addr().(*net.TCPAddr).Port
	localPort1 := randomPort()
	localPort2 := randomPort()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialConfig := fmt.Sprintf(`
ssh:
  user: testuser
  password: testpass
  host: 127.0.0.1
  port: %d

tunnels:
  - name: tunnel1
    remoteHost: 127.0.0.1
    remotePort: 1521
    localPort: %d
`, port, localPort1)

	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	mgr := manager.NewManager(sshCfg)

	w, _ := New(configPath, mgr)
	_ = w.Start()
	defer w.Stop()
	defer mgr.StopAll()

	time.Sleep(100 * time.Millisecond)

	os.Remove(configPath)

	time.Sleep(100 * time.Millisecond)

	newConfig := fmt.Sprintf(`
ssh:
  user: testuser
  password: testpass
  host: 127.0.0.1
  port: %d

tunnels:
  - name: tunnel1
    remoteHost: 127.0.0.1
    remotePort: 1521
    localPort: %d
  - name: tunnel2
    remoteHost: 127.0.0.1
    remotePort: 1522
    localPort: %d
`, port, localPort1, localPort2)

	err = os.WriteFile(configPath, []byte(newConfig), 0644)
	if err != nil {
		t.Fatalf("failed to recreate config: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("expected 2 tunnels after recreate, got %d: %v", len(list), list)
	}
}

// randomPort generates and returns a random port number within the range of 20000 to 29999.
func randomPort() int {
	n, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return int(n.Int64()) + 20000
}

// createTempConfigFile creates a temporary configuration file with the provided content and returns its file path.
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	return configPath
}

// validConfigContent generates and returns a valid YAML configuration for SSH and tunnel setup with a random local port.
func validConfigContent() string {
	port := randomPort()
	return fmt.Sprintf(`
ssh:
  user: testuser
  password: testpass
  host: localhost
  port: 22

tunnels:
  - name: test
    remoteHost: localhost
    remotePort: 1521
    localPort: %d
`, port)
}

// setupTestSSHServer creates and starts a test SSH server, returning a listener and an SSHConfig for client connections.
func setupTestSSHServer(t *testing.T) (net.Listener, *tunnel.SSHConfig) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleTestSSHConnection(conn, serverConfig)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	cfg, err := tunnel.NewSSHConfig("testuser", "testpass", "", "127.0.0.1", "", port)
	if err != nil {
		listener.Close()
		t.Fatalf("failed to create ssh config: %v", err)
	}

	return listener, cfg
}

// handleTestSSHConnection manages an incoming SSH connection and handles direct-tcpip channels for tunneling data.
func handleTestSSHConnection(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() == "direct-tcpip" {
			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(requests)

			var payload struct {
				DestHost   string
				DestPort   uint32
				OriginHost string
				OriginPort uint32
			}
			ssh.Unmarshal(newChannel.ExtraData(), &payload)

			destAddr := fmt.Sprintf("%s:%d", payload.DestHost, payload.DestPort)
			destConn, err := net.Dial("tcp", destAddr)
			if err != nil {
				channel.Close()
				continue
			}

			go func() {
				defer channel.Close()
				defer destConn.Close()
				io.Copy(channel, destConn)
			}()
			go func() {
				defer channel.Close()
				defer destConn.Close()
				io.Copy(destConn, channel)
			}()
		}
	}
}
