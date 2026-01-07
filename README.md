# Conduit

A secure tunnel service that manages multiple SSH tunnels to remote databases.

## Overview

Conduit creates and manages SSH tunnels, exposing them as stable local endpoints. Applications connect to Conduit as if remote databases were local, with no SSH configuration needed on the client side.

## Features

- Multiple simultaneous tunnels
- Automatic reconnection on failure (per-tunnel configuration)
- Password and SSH key authentication
- Keyboard-interactive authentication support
- Known hosts verification for production security
- Hot reload configuration changes
- Configuration via YAML with environment variable expansion

## How it works
```
┌─────────────────────────────────────────────────────────────────────────┐
│ Conduit                                                                 │
│                                                                         │
│  localhost:1521 ══════════════════════════════════════════════════════▶ Bastion ══▶ Oracle database1
│  localhost:1522 ══════════════════════════════════════════════════════▶ Bastion ══▶ Oracle database2
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
        ▲              ▲
        │              │
  ┌─────┴─────┐  ┌─────┴─────┐
  │   App A   │  │   App B   │
  └───────────┘  └───────────┘

  Apps connect to localhost:1521 - no SSH config needed
```

## Installation

### From source
```bash
git clone https://github.com/pperesbr/conduit.git
cd conduit
go build -o conduit ./cmd/main.go
```

### Docker
```bash
docker build -t conduit:latest .
```

## Configuration

Create a `config.yaml` file:
```yaml
ssh:
  host: bastion.example.com
  port: 22
  user: tunnel-user
  password: ${SSH_PASSWORD}        # Environment variable expansion
  # keyFile: /path/to/id_rsa      # Or use SSH key instead of password
  # knownHostsFile: /config/known_hosts  # For production security

tunnels:
  - name: database1
    remoteHost: oracle-database1.internal
    remotePort: 1521
    localPort: 1521
    autoRestart:
      enabled: true
      interval: 30s

  - name: database2
    remoteHost: oracle-database2.internal
    remotePort: 1521
    localPort: 1522
    autoRestart:
      enabled: true
      interval: 30s
```

### Configuration Options

#### SSH

| Field | Required | Description |
|-------|----------|-------------|
| `host` | Yes | SSH bastion/jump server hostname |
| `port` | No | SSH port (default: 22) |
| `user` | Yes | SSH username |
| `password` | * | SSH password (supports `${ENV_VAR}` syntax) |
| `keyFile` | * | Path to SSH private key |
| `knownHostsFile` | No | Path to known_hosts file (recommended for production) |

\* Either `password` or `keyFile` is required.

#### Tunnels

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique tunnel identifier |
| `remoteHost` | Yes | Target host (from bastion's perspective) |
| `remotePort` | Yes | Target port |
| `localPort` | Yes | Local port to expose |
| `autoRestart.enabled` | No | Enable automatic reconnection (default: false) |
| `autoRestart.interval` | No | Health check interval (e.g., `30s`, `1m`) |

## Usage

### Running locally
```bash
# Set SSH password
export SSH_PASSWORD="your-password"

# Start Conduit
./conduit -config config.yaml
```

### Running with Docker
```bash
# Using docker run
docker run -d \
  --name conduit \
  -p 1521:1521 \
  -p 1522:1522 \
  -v $(pwd)/config.yaml:/app/config/config.yaml:ro \
  -e SSH_PASSWORD="your-password" \
  conduit:latest

# Using docker-compose
export SSH_PASSWORD="your-password"
docker-compose up -d
```

### docker-compose.yaml
```yaml
version: '3.8'

services:
  conduit:
    build: .
    container_name: conduit
    restart: unless-stopped
    ports:
      - "1521:1521"
      - "1522:1522"
    volumes:
      - ./config.yaml:/app/config/config.yaml:ro
    environment:
      - SSH_PASSWORD=${SSH_PASSWORD}
```

## Connecting Applications

Once Conduit is running, applications connect to local ports:

### Oracle (Go)
```go
dsn := "oracle://user:password@127.0.0.1:1521/database1"
db, err := sql.Open("oracle", dsn)
```

### Oracle (Python)
```python
import oracledb
conn = oracledb.connect(user="user", password="password", dsn="127.0.0.1:1521/database1")
```

### Oracle (Java)
```java
String url = "jdbc:oracle:thin:@127.0.0.1:1521:database1";
Connection conn = DriverManager.getConnection(url, "user", "password");
```

## Hot Reload

Conduit watches the configuration file for changes. When you modify `config.yaml`:

- New tunnels are automatically added and started
- Removed tunnels are stopped and cleaned up
- Changed tunnels are restarted with new configuration

No restart required!

## Logs
```bash
# Local
./conduit -config config.yaml

# Docker
docker-compose logs -f
```

Example output:
```
2026/01/07 19:50:25 conduit: starting with config /app/config/config.yaml
2026/01/07 19:50:25 conduit: loaded 2 tunnel(s) via g0004830@10.113.114.9:22
2026/01/07 19:50:25 conduit: added tunnel database1 (10.240.47.114:1521 -> localhost:1521)
2026/01/07 19:50:25 conduit: added tunnel database2 (10.240.47.3:1521 -> localhost:1522)
2026/01/07 19:50:25 conduit: tunnel database1 status: running
2026/01/07 19:50:25 conduit: tunnel database2 status: running
2026/01/07 19:50:25 conduit: watching config file for changes
```

## Graceful Shutdown

Conduit handles `SIGINT` and `SIGTERM` signals for graceful shutdown:
```bash
# Stop gracefully
kill -SIGTERM $(pgrep conduit)

# Or with Docker
docker-compose down
```

## Security Recommendations

1. **Use SSH keys** instead of passwords in production
2. **Use known_hosts** to verify bastion identity
3. **Run as non-root** (Docker image already does this)
4. **Use secrets management** for credentials (K8s Secrets, Vault, etc.)