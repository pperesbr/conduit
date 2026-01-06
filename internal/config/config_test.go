package config

import (
	"os"
	"path/filepath"
	"testing"
)

func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	return configPath
}

func TestLoad_ValidConfig(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com
  port: 22

tunnels:
  - name: sig
    remoteHost: oracle-sig
    remotePort: 1521
    localPort: 1521
`
	configPath := createTempConfig(t, content)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SSH.User != "testuser" {
		t.Errorf("expected user 'testuser', got '%s'", cfg.SSH.User)
	}

	if cfg.SSH.Host != "bastion.com" {
		t.Errorf("expected host 'bastion.com', got '%s'", cfg.SSH.Host)
	}

	if len(cfg.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(cfg.Tunnels))
	}

	if cfg.Tunnels[0].Name != "sig" {
		t.Errorf("expected tunnel name 'sig', got '%s'", cfg.Tunnels[0].Name)
	}
}

func TestLoad_WithKeyFile(t *testing.T) {
	// Cria arquivo de chave temporário
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_rsa")
	keyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBwNlsVOBKosw+jG0cxb/L2sHf0luTMKCyLFLWCOIHzVAAAAJhPUzrTT1M6
0wAAAAtzc2gtZWQyNTUxOQAAACBwNlsVOBKosw+jG0cxb/L2sHf0luTMKCyLFLWCOIHzVA
AAAECpVPKPdliGs+H4XUjDJmWTafFnhrpCLVFb8FkUdsLfE3A2WxU4EqizD6MbRzFv8vaw
d/SW5MwoLIsUtYI4gfNUAAAAEHRlc3RAZXhhbXBsZS5jb20BAgMEBQ==
-----END OPENSSH PRIVATE KEY-----`
	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}

	content := `
ssh:
  user: testuser
  keyFile: ` + keyPath + `
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SSH.KeyFile != keyPath {
		t.Errorf("expected keyFile '%s', got '%s'", keyPath, cfg.SSH.KeyFile)
	}
}

func TestLoad_WithEnvVar(t *testing.T) {
	os.Setenv("TEST_SSH_PASSWORD", "secret123")
	defer os.Unsetenv("TEST_SSH_PASSWORD")

	content := `
ssh:
  user: testuser
  password: ${TEST_SSH_PASSWORD}
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SSH.Password != "secret123" {
		t.Errorf("expected password 'secret123', got '%s'", cfg.SSH.Password)
	}
}

func TestLoad_MultipleTunnels(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - name: sig
    remoteHost: oracle-sig
    remotePort: 1521
    localPort: 1521
  - name: ods
    remoteHost: oracle-ods
    remotePort: 1521
    localPort: 1522
  - name: postgres
    remoteHost: pg-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Tunnels) != 3 {
		t.Errorf("expected 3 tunnels, got %d", len(cfg.Tunnels))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/path/that/does/not/exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: [invalid yaml
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_MissingSSHHost(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing ssh.host")
	}
}

func TestValidate_MissingSSHUser(t *testing.T) {
	content := `
ssh:
  password: testpass
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing ssh.user")
	}
}

func TestValidate_MissingSSHAuth(t *testing.T) {
	content := `
ssh:
  user: testuser
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing ssh auth")
	}
}

func TestValidate_NoTunnels(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels: []
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for no tunnels")
	}
}

func TestValidate_DuplicateTunnelName(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server1
    remotePort: 5432
    localPort: 5432
  - name: db
    remoteHost: db-server2
    remotePort: 5432
    localPort: 5433
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for duplicate tunnel name")
	}
}

func TestValidate_DuplicateLocalPort(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - name: db1
    remoteHost: db-server1
    remotePort: 5432
    localPort: 5432
  - name: db2
    remoteHost: db-server2
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for duplicate localPort")
	}
}

func TestValidate_MissingTunnelName(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - remoteHost: db-server
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing tunnel name")
	}
}

func TestValidate_MissingRemoteHost(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - name: db
    remotePort: 5432
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing remoteHost")
	}
}

func TestValidate_InvalidRemotePort(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 0
    localPort: 5432
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid remotePort")
	}
}

func TestValidate_InvalidLocalPort(t *testing.T) {
	content := `
ssh:
  user: testuser
  password: testpass
  host: bastion.com

tunnels:
  - name: db
    remoteHost: db-server
    remotePort: 5432
    localPort: 0
`
	configPath := createTempConfig(t, content)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid localPort")
	}
}
