# Conduit

A secure tunnel service that manages multiple SSH tunnels to remote databases.

## Overview

Conduit creates and manages SSH tunnels, exposing them as stable local endpoints. Applications connect to Conduit as if remote databases were local, with no SSH configuration needed on the client side.

## Features

- Multiple simultaneous tunnels
- Automatic reconnection on failure
- Password and SSH key authentication
- Known hosts verification for production security
- Health check endpoints
- Configuration via YAML

## How it works
```
┌─────────────────────────────────────────────────────────────────────────┐
│ Conduit                                                                 │
│                                                                         │
│  localhost:1521 ════════════════════════════════════════════════════════▶ Bastion ══▶ Oracle database1
│  localhost:1522 ════════════════════════════════════════════════════════▶ Bastion ══▶ Oracle database2
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
        ▲              ▲
        │              │
  ┌─────┴─────┐  ┌─────┴─────┐
  │   App A   │  │   App B   │
  └───────────┘  └───────────┘

  Apps connect to localhost:1521 - no SSH config needed
```

## Configuration
```yaml
ssh:
  host: bastion.example.com
  port: 22
  user: tunnel-user
  password: ${SSH_PASSWORD}        # or use keyFile
  knownHostsFile: /config/known_hosts

tunnels:
  - name: database1
    remoteHost: oracle-database1.internal
    remotePort: 1521
    localPort: 1521

  - name: database2
    remoteHost: oracle-database2.internal
    remotePort: 1521
    localPort: 1522
```

## Usage
```bash
# Start Conduit
./conduit -config config.yaml
```

Applications connect directly to local ports:
```
host: localhost
port: 1521
database: database1
```