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
- Helm chart for Kubernetes deployment

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
# Pull from GitHub Container Registry
docker pull ghcr.io/pperesbr/conduit:latest

# Or build locally
docker build -t conduit:latest .
```

### Helm (Kubernetes)
```bash
# Clone the repository
git clone https://github.com/pperesbr/conduit.git
cd conduit

# Install with Helm
helm install conduit charts/conduit --namespace conduit --create-namespace
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
  ghcr.io/pperesbr/conduit:latest

# Using docker-compose
export SSH_PASSWORD="your-password"
docker-compose up -d
```

### docker-compose.yaml
```yaml
version: '3.8'

services:
  conduit:
    image: ghcr.io/pperesbr/conduit:latest
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

### Running on Kubernetes (K3s/K8s)

#### Option 1: Using SSH Key (Recommended)
```bash
# 1. Create namespace
kubectl create namespace conduit

# 2. Create secret with SSH private key
kubectl create secret generic conduit-ssh-key \
  --namespace conduit \
  --from-file=ssh-key=/path/to/your/private-key

# 3. Install with Helm
helm install conduit charts/conduit \
  --namespace conduit \
  --set ssh.host=bastion.example.com \
  --set ssh.port=22 \
  --set ssh.user=tunnel-user \
  --set ssh.keySecret=conduit-ssh-key \
  --set ssh.keySecretKey=ssh-key \
  --set "tunnels[0].name=database1" \
  --set "tunnels[0].remoteHost=oracle-db1.internal" \
  --set "tunnels[0].remotePort=1521" \
  --set "tunnels[0].localPort=1521" \
  --set "tunnels[0].autoRestart.enabled=true" \
  --set "tunnels[0].autoRestart.interval=30s" \
  --set "tunnels[1].name=database2" \
  --set "tunnels[1].remoteHost=oracle-db2.internal" \
  --set "tunnels[1].remotePort=1521" \
  --set "tunnels[1].localPort=1522" \
  --set "tunnels[1].autoRestart.enabled=true" \
  --set "tunnels[1].autoRestart.interval=30s" \
  --set hostNetwork=true
```

#### Option 2: Using Password
```bash
# 1. Create namespace
kubectl create namespace conduit

# 2. Install with Helm
helm install conduit charts/conduit \
  --namespace conduit \
  --set ssh.host=bastion.example.com \
  --set ssh.port=22 \
  --set ssh.user=tunnel-user \
  --set ssh.password=your-password \
  --set "tunnels[0].name=database1" \
  --set "tunnels[0].remoteHost=oracle-db1.internal" \
  --set "tunnels[0].remotePort=1521" \
  --set "tunnels[0].localPort=1521" \
  --set hostNetwork=true
```

#### Using values.yaml file

Create a `my-values.yaml`:
```yaml
ssh:
  host: bastion.example.com
  port: 22
  user: tunnel-user
  keySecret: conduit-ssh-key
  keySecretKey: ssh-key

tunnels:
  - name: database1
    remoteHost: oracle-db1.internal
    remotePort: 1521
    localPort: 1521
    autoRestart:
      enabled: true
      interval: 30s

  - name: database2
    remoteHost: oracle-db2.internal
    remotePort: 1521
    localPort: 1522
    autoRestart:
      enabled: true
      interval: 30s

hostNetwork: true

resources:
  limits:
    cpu: 100m
    memory: 128Mi
  requests:
    cpu: 50m
    memory: 64Mi
```

Install:
```bash
helm install conduit charts/conduit \
  --namespace conduit \
  --create-namespace \
  -f my-values.yaml
```

#### Upgrade
```bash
helm upgrade conduit charts/conduit \
  --namespace conduit \
  -f my-values.yaml
```

#### Uninstall
```bash
helm uninstall conduit --namespace conduit
kubectl delete namespace conduit
```

### Helm Chart Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `ghcr.io/pperesbr/conduit` |
| `image.tag` | Image tag | `1.0.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `replicaCount` | Number of replicas | `1` |
| `ssh.host` | SSH bastion hostname | `""` |
| `ssh.port` | SSH port | `22` |
| `ssh.user` | SSH username | `""` |
| `ssh.password` | SSH password | `""` |
| `ssh.existingSecret` | Existing secret for password | `""` |
| `ssh.secretKey` | Key in secret for password | `ssh-password` |
| `ssh.keySecret` | Secret name containing SSH key | `""` |
| `ssh.keySecretKey` | Key in secret for SSH key | `ssh-key` |
| `tunnels` | List of tunnel configurations | `[]` |
| `hostNetwork` | Use host network | `false` |
| `service.type` | Service type | `ClusterIP` |
| `resources.limits.cpu` | CPU limit | `100m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `50m` |
| `resources.requests.memory` | Memory request | `64Mi` |

## Connecting Applications

### Local/Docker

Applications connect to `localhost` or `127.0.0.1`:
```
host: 127.0.0.1
port: 1521
```

### Kubernetes

Applications connect via the service:
```
host: conduit.conduit.svc.cluster.local
port: 1521
```

Or if in the same namespace:
```
host: conduit
port: 1521
```

### Oracle (Go)
```go
// Local/Docker
dsn := "oracle://user:password@127.0.0.1:1521/database1"

// Kubernetes
dsn := "oracle://user:password@conduit.conduit.svc.cluster.local:1521/database1"

db, err := sql.Open("oracle", dsn)
```

### Oracle (Python)
```python
import oracledb

# Local/Docker
conn = oracledb.connect(user="user", password="password", dsn="127.0.0.1:1521/database1")

# Kubernetes
conn = oracledb.connect(user="user", password="password", dsn="conduit.conduit.svc.cluster.local:1521/database1")
```

### Oracle (Java)
```java
// Local/Docker
String url = "jdbc:oracle:thin:@127.0.0.1:1521:database1";

// Kubernetes
String url = "jdbc:oracle:thin:@conduit.conduit.svc.cluster.local:1521:database1";

Connection conn = DriverManager.getConnection(url, "user", "password");
```

## Hot Reload

Conduit watches the configuration file for changes. When you modify `config.yaml`:

- New tunnels are automatically added and started
- Removed tunnels are stopped and cleaned up
- Changed tunnels are restarted with new configuration

No restart required!

In Kubernetes, update the Helm release to change tunnels:
```bash
helm upgrade conduit charts/conduit --namespace conduit -f my-values.yaml
```

## Logs
```bash
# Local
./conduit -config config.yaml

# Docker
docker-compose logs -f

# Kubernetes
kubectl logs -n conduit -l app.kubernetes.io/name=conduit -f
```

Example output:
```
2026/01/07 21:38:40 conduit: starting with config /app/config/config.yaml
2026/01/07 21:38:40 conduit: loaded 2 tunnel(s) via tunnel-user@bastion.example.com:22
2026/01/07 21:38:40 conduit: added tunnel database1 (oracle-db1.internal:1521 -> localhost:1521)
2026/01/07 21:38:40 conduit: added tunnel database2 (oracle-db2.internal:1521 -> localhost:1522)
2026/01/07 21:38:40 conduit: tunnel database1 status: running
2026/01/07 21:38:40 conduit: tunnel database2 status: running
2026/01/07 21:38:40 conduit: watching config file for changes
```

## Graceful Shutdown

Conduit handles `SIGINT` and `SIGTERM` signals for graceful shutdown:
```bash
# Local
kill -SIGTERM $(pgrep conduit)

# Docker
docker-compose down

# Kubernetes
kubectl delete pod -n conduit -l app.kubernetes.io/name=conduit
```

## Troubleshooting

### Kubernetes: "No route to host"

If you see this error, the pod cannot reach the bastion. Solutions:

1. **Enable hostNetwork**: Add `--set hostNetwork=true` to use the node's network
2. **Check firewall**: Ensure the bastion allows connections from K8s node IPs
3. **Test from node**: SSH to a K8s node and test `nc -zv bastion-ip 22`

### Kubernetes: "Permission denied" on SSH key

Ensure the secret is created correctly:
```bash
kubectl create secret generic conduit-ssh-key \
  --namespace conduit \
  --from-file=ssh-key=/path/to/private-key
```

### Tunnel keeps restarting

Check if the remote host is reachable from the bastion:
```bash
# SSH to bastion
ssh user@bastion

# Test connection to remote host
nc -zv oracle-db.internal 1521
```

## Security Recommendations

1. **Use SSH keys** instead of passwords in production
2. **Use known_hosts** to verify bastion identity
3. **Run as non-root** (Docker image already does this)
4. **Use secrets management** for credentials (K8s Secrets, Vault, etc.)
5. **Limit network access** to the bastion from K8s nodes only
6. **Use dedicated SSH user** with minimal permissions
