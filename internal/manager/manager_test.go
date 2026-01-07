package manager

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/pperesbr/conduit/internal/config"
	"github.com/pperesbr/gokit/pkg/tunnel"
	"golang.org/x/crypto/ssh"
)

// TestNewManager validates the creation of a new Manager instance and ensures the initial collection of tunnels is empty.
func TestNewManager(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)

	mgr := NewManager(cfg)

	if mgr == nil {
		t.Fatal("expected manager to be created")
	}

	if len(mgr.List()) != 0 {
		t.Errorf("expected 0 tunnels, got %d", len(mgr.List()))
	}
}

// TestAdd_Success tests the successful addition of a tunnel configuration to the manager.
func TestAdd_Success(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "sigitm",
		RemoteHost: "oracle-sigitm",
		RemotePort: 1521,
		LocalPort:  1521,
	}

	err := mgr.Add(tunnelCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mgr.List()) != 1 {
		t.Errorf("expected 1 tunnel, got %d", len(mgr.List()))
	}
}

// TestAdd_Duplicate verifies that attempting to add a duplicate tunnel configuration returns an error.
func TestAdd_Duplicate(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "sigitm",
		RemoteHost: "oracle-sigitm",
		RemotePort: 1521,
		LocalPort:  1521,
	}

	_ = mgr.Add(tunnelCfg)
	err := mgr.Add(tunnelCfg)

	if err == nil {
		t.Fatal("expected error for duplicate tunnel")
	}
}

// TestAdd_MultipleTunnels verifies the addition of multiple tunnel configurations to the tunnel manager without errors.
// Ensures the correct number of tunnels are added to the manager.
func TestAdd_MultipleTunnels(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnels := []config.TunnelConfig{
		{Name: "sigitm", RemoteHost: "oracle-sigitm", RemotePort: 1521, LocalPort: 1521},
		{Name: "ods", RemoteHost: "oracle-ods", RemotePort: 1521, LocalPort: 1522},
		{Name: "postgres", RemoteHost: "pg-server", RemotePort: 5432, LocalPort: 5432},
	}

	for _, tc := range tunnels {
		if err := mgr.Add(tc); err != nil {
			t.Fatalf("unexpected error adding %s: %v", tc.Name, err)
		}
	}

	if len(mgr.List()) != 3 {
		t.Errorf("expected 3 tunnels, got %d", len(mgr.List()))
	}
}

// TestRemove_Success verifies the successful removal of a tunnel and ensures there are no remaining tunnels in the manager.
func TestRemove_Success(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "sigitm",
		RemoteHost: "oracle-sigitm",
		RemotePort: 1521,
		LocalPort:  1521,
	}

	_ = mgr.Add(tunnelCfg)
	err := mgr.Remove("sigitm")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mgr.List()) != 0 {
		t.Errorf("expected 0 tunnels, got %d", len(mgr.List()))
	}
}

// TestRemove_NotFound verifies that attempting to remove a non-existent tunnel results in an appropriate error response.
func TestRemove_NotFound(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	err := mgr.Remove("not-exists")

	if err == nil {
		t.Fatal("expected error for non-existent tunnel")
	}
}

// TestStart_Success verifies that a tunnel is successfully started and its status is updated to running without errors.
func TestStart_Success(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
	}

	_ = mgr.Add(tunnelCfg)
	err := mgr.Start("test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Stop("test")

	status := mgr.Status()
	if status["test"] != tunnel.StatusRunning {
		t.Errorf("expected status Running, got %s", status["test"])
	}
}

// TestStart_NotFound verifies that attempting to start a non-existent tunnel returns an error as expected.
func TestStart_NotFound(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	err := mgr.Start("not-exists")

	if err == nil {
		t.Fatal("expected error for non-existent tunnel")
	}
}

// TestStop_Success verifies that a tunnel can be stopped successfully and ensures its status is updated to "Stopped".
func TestStop_Success(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
	}

	_ = mgr.Add(tunnelCfg)
	_ = mgr.Start("test")
	err := mgr.Stop("test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := mgr.Status()
	if status["test"] != tunnel.StatusStopped {
		t.Errorf("expected status Stopped, got %s", status["test"])
	}
}

// TestRestart_Success verifies that restarting a tunnel transitions it to the running state successfully without errors.
func TestRestart_Success(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
	}

	_ = mgr.Add(tunnelCfg)
	_ = mgr.Start("test")
	err := mgr.Restart("test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Stop("test")

	status := mgr.Status()
	if status["test"] != tunnel.StatusRunning {
		t.Errorf("expected status Running, got %s", status["test"])
	}
}

// TestStartAll_Success verifies that all tunnels start successfully without errors, and their statuses are set to Running.
func TestStartAll_Success(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnels := []config.TunnelConfig{
		{Name: "t1", RemoteHost: "127.0.0.1", RemotePort: 1521, LocalPort: 0},
		{Name: "t2", RemoteHost: "127.0.0.1", RemotePort: 1522, LocalPort: 0},
	}

	for _, tc := range tunnels {
		_ = mgr.Add(tc)
	}

	errors := mgr.StartAll()
	defer mgr.StopAll()

	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}

	status := mgr.Status()
	for _, tc := range tunnels {
		if status[tc.Name] != tunnel.StatusRunning {
			t.Errorf("expected %s to be Running, got %s", tc.Name, status[tc.Name])
		}
	}
}

// TestStopAll_Success ensures that all active tunnels are stopped without errors and verifies their status as stopped.
func TestStopAll_Success(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnels := []config.TunnelConfig{
		{Name: "t1", RemoteHost: "127.0.0.1", RemotePort: 1521, LocalPort: 0},
		{Name: "t2", RemoteHost: "127.0.0.1", RemotePort: 1522, LocalPort: 0},
	}

	for _, tc := range tunnels {
		_ = mgr.Add(tc)
	}

	_ = mgr.StartAll()
	errors := mgr.StopAll()

	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errors), errors)
	}

	status := mgr.Status()
	for _, tc := range tunnels {
		if status[tc.Name] != tunnel.StatusStopped {
			t.Errorf("expected %s to be Stopped, got %s", tc.Name, status[tc.Name])
		}
	}
}

// TestGet_Exists verifies that an added tunnel can be retrieved successfully using the manager's Get method.
func TestGet_Exists(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "sigitm",
		RemoteHost: "oracle-sigitm",
		RemotePort: 1521,
		LocalPort:  1521,
	}

	_ = mgr.Add(tunnelCfg)
	tun := mgr.Get("sigitm")

	if tun == nil {
		t.Fatal("expected tunnel to exist")
	}
}

// TestGet_NotExists verifies that the Manager's Get method returns nil when querying a non-existent tunnel by name.
func TestGet_NotExists(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tun := mgr.Get("not-exists")

	if tun != nil {
		t.Fatal("expected tunnel to be nil")
	}
}

// TestList verifies that tunnels can be successfully added to the manager and retrieved using the List method.
func TestList(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnels := []config.TunnelConfig{
		{Name: "sigitm", RemoteHost: "oracle-sigitm", RemotePort: 1521, LocalPort: 1521},
		{Name: "ods", RemoteHost: "oracle-ods", RemotePort: 1521, LocalPort: 1522},
	}

	for _, tc := range tunnels {
		_ = mgr.Add(tc)
	}

	list := mgr.List()

	if len(list) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(list))
	}
}

// TestStatus is a unit test that verifies the status of a tunnel after its initialization in the Manager.
func TestStatus(t *testing.T) {
	cfg, _ := tunnel.NewSSHConfig("user", "pass", "", "localhost", "", 22)
	mgr := NewManager(cfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "sigitm",
		RemoteHost: "oracle-sigitm",
		RemotePort: 1521,
		LocalPort:  1521,
	}

	_ = mgr.Add(tunnelCfg)
	status := mgr.Status()

	if status["sigitm"] != tunnel.StatusStopped {
		t.Errorf("expected status Stopped, got %s", status["sigitm"])
	}
}

// TestHealthCheck verifies the health status of an SSH tunnel managed by the Manager and ensures it is marked as healthy.
func TestHealthCheck(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
	}

	_ = mgr.Add(tunnelCfg)
	_ = mgr.Start("test")
	defer mgr.Stop("test")

	health := mgr.HealthCheck()

	if len(health) != 1 {
		t.Fatalf("expected 1 health status, got %d", len(health))
	}

	if !health[0].Healthy {
		t.Errorf("expected tunnel to be healthy")
	}
}

// TestUnhealthy_NoProblems validates that no tunnels are reported as unhealthy when all configured tunnels are functioning correctly.
func TestUnhealthy_NoProblems(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
	}

	_ = mgr.Add(tunnelCfg)
	_ = mgr.Start("test")
	defer mgr.Stop("test")

	unhealthy := mgr.Unhealthy()

	if len(unhealthy) != 0 {
		t.Errorf("expected 0 unhealthy, got %d", len(unhealthy))
	}
}

// TestStart_WithAutoRestart verifies that a tunnel with auto-restart enabled is properly started and monitored for restarts.
func TestStart_WithAutoRestart(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
		AutoRestart: config.AutoRestartConfig{
			Enabled:  true,
			Interval: 100 * time.Millisecond,
		},
	}

	_ = mgr.Add(tunnelCfg)
	err := mgr.Start("test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Stop("test")

	mgr.mu.RLock()
	_, hasAutoRestart := mgr.tunnelDones["test"]
	mgr.mu.RUnlock()

	if !hasAutoRestart {
		t.Error("expected auto restart to be started")
	}
}

// TestStop_StopsAutoRestart verifies that the Stop function disables the auto-restart behavior for a specific tunnel.
func TestStop_StopsAutoRestart(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
		AutoRestart: config.AutoRestartConfig{
			Enabled:  true,
			Interval: 100 * time.Millisecond,
		},
	}

	_ = mgr.Add(tunnelCfg)
	_ = mgr.Start("test")
	_ = mgr.Stop("test")

	mgr.mu.RLock()
	_, hasAutoRestart := mgr.tunnelDones["test"]
	mgr.mu.RUnlock()

	if hasAutoRestart {
		t.Error("expected auto restart to be stopped")
	}
}

// TestClose verifies the behavior of the Close method, ensuring tunnels are properly stopped and status is updated correctly.
func TestClose(t *testing.T) {
	sshServer, sshCfg := setupTestSSHServer(t)
	defer sshServer.Close()

	mgr := NewManager(sshCfg)

	tunnelCfg := config.TunnelConfig{
		Name:       "test",
		RemoteHost: "127.0.0.1",
		RemotePort: 1521,
		LocalPort:  0,
	}

	_ = mgr.Add(tunnelCfg)
	_ = mgr.Start("test")

	err := mgr.Close()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	status := mgr.Status()
	if status["test"] != tunnel.StatusStopped {
		t.Errorf("expected status Stopped, got %s", status["test"])
	}
}

// setupTestSSHServer creates and starts a test SSH server for unit testing, returning the listener and SSH configuration.
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

// handleTestSSHConnection handles an incoming SSH connection, sets up channels, and forwards traffic to the requested destination.
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
