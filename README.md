<p align="center"> <img src="https://github.com/Horinsoft/caimandb/blob/main/ui/img/logo.png" alt="CaimanDB Logo" width="100%"/> </p>

# CaimanDB

**Distributed Document-Oriented Database Engine with Native Sharding, Raft-based Clustering, WAL, ACID Transactions, and SQL-like Query Language (NQL).**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/caimandb/caimandb)](https://goreportcard.com/report/github.com/caimandb/caimandb)
[![Documentation](https://img.shields.io/badge/docs-reference-blue.svg)](https://caimandb.io/docs)


### NQL (Native Query Language)

- **Nql english**: [NQLSINTAX.en🍔](https://github.com/Horinsoft/caimandb/docs/NQL_SYNTAX.en.md)
- **Nql español**: [NQLSINTAX.es🌮](https://github.com/Horinsoft/caimandb/docs/NQL_SYNTAX.es.md)
- **Nql desland**: [NQLSINTAX.de🍺](https://github.com/Horinsoft/caimandb/docs/NQL_SYNTAX.de.md)
- **Website**: [https://caimandb.io](https://caimandb.io)

---

## Table of Contents

1. [Key Features](#key-features)
2. [Architecture](#architecture)
3. [System Requirements](#system-requirements)
4. [Installation and Compilation](#installation-and-compilation)
5. [Configuration](#configuration)
6. [Execution](#execution)
7. [Usage](#usage)
8. [Documentation](#documentation)
9. [Contributing](#contributing)
10. [License](#license)

---

## Key Features

**Document-Oriented Storage**
- Native JSON document storage with flexible schema
- Automatic and manual indexing for efficient queries
- Optional schema validation and constraints

**Distribution and Scalability**
- Automatic sharding for horizontal data distribution
- Replication and fault tolerance via Raft consensus protocol
- Load balancing across cluster nodes
- Zero-downtime horizontal scaling

**Persistence and Consistency**
- Write-Ahead Log (WAL) for durability and crash recovery
- Full ACID transactions (Atomic, Consistent, Isolated, Durable)
- Checkpoints and snapshots for fast recovery

**Query Language and APIs**
- NQL (Native Query Language): SQL-like syntax optimized for documents
- RESTful HTTP API with OpenAPI documentation
- Interactive CLI for administration and queries
- Concurrent connection and session support

**Monitoring and Administration**
- Real-time cluster statistics
- Structured logging with configurable levels
- Health check and metrics endpoints
- Remote administration via CLI

---

## Architecture

CaimanDB implements a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────┐
│                      Presentation Layer                         │
│                   CLI / HTTP API / WebSocket                    │
├─────────────────────────────────────────────────────────────────┤
│                     Processing Layer                            │
│              NQL Parser / Query Engine / Session Manager        │
├─────────────────────────────────────────────────────────────────┤
│                    Coordination Layer                           │
│      Transaction Manager / Index Manager / Sharding Manager     │
├─────────────────────────────────────────────────────────────────┤
│                      Consistency Layer                          │
│           Raft Consensus Layer / Cluster State Machine          │
├─────────────────────────────────────────────────────────────────┤
│                      Persistence Layer                          │
│                 WAL (Journal) / Storage Engine                  │
├─────────────────────────────────────────────────────────────────┤
│                    Physical Storage Layer                       │
│                      File System / Disk                         │
└─────────────────────────────────────────────────────────────────┘
```

For detailed architecture and disk storage information, refer to [`docs/architecture.md`](docs/architecture.md).

---

## System Requirements

**Minimum Requirements**
- Operating System: Linux, macOS, or Windows
- Go 1.22 or higher (for source compilation)
- 1 GB RAM (2 GB recommended for production)
- 1 GB disk space (10 GB recommended for production)

**Optional Requirements**
- Make (for automated commands)
- Docker and Docker Compose (for container deployment)
- Git (for cloning the repository)

---

## Installation and Compilation

### Obtaining the Source Code

```bash
git clone https://github.com/Horinsoft/caimandb.git
cd caimandb
```

### Automated Compilation

The project includes build scripts for different operating systems:

**Unix Systems (Linux / macOS):**
```bash
./scripts/build.sh
```

**Windows:**
```cmd
build.bat
```

**Using Makefile (Unix):**
```bash
make build
```

### Manual Compilation

```bash
go build -o bin/caimandb ./cmd/caimandb
```

### Cross-Platform Compilation

Generate binaries for different platforms from a single system:

**Linux (amd64):**
```bash
GOOS=linux GOARCH=amd64 go build -o bin/caimandb-linux-amd64 ./cmd/caimandb
```

**Linux (ARM64):**
```bash
GOOS=linux GOARCH=arm64 go build -o bin/caimandb-linux-arm64 ./cmd/caimandb
```

**Windows (amd64):**
```bash
GOOS=windows GOARCH=amd64 go build -o bin/caimandb-windows-amd64.exe ./cmd/caimandb
```

**macOS (amd64):**
```bash
GOOS=darwin GOARCH=amd64 go build -o bin/caimandb-darwin-amd64 ./cmd/caimandb
```

**macOS (ARM64):**
```bash
GOOS=darwin GOARCH=arm64 go build -o bin/caimandb-darwin-arm64 ./cmd/caimandb
```

### Build Verification

```bash
# Static code analysis
go vet ./...

# Unit tests execution
go test ./...

# Dependency verification
go mod verify
go mod tidy
```

### System Installation

```bash
# Install to $GOPATH/bin
go install ./cmd/caimandb

# Or copy manually
cp bin/caimandb /usr/local/bin/
```

---


## View UI

```port
http://localhost:1555/
```

---

## Configuration

### Automatic Configuration Generation

On first startup, CaimanDB automatically generates:
- The `configs/` directory if it doesn't exist
- The `configs/caimandb.conf` file with default values
- Data directories (`data/`, `wal/`, etc.) according to configuration

### Configuration Structure

The configuration file uses JSON format:

```json
{
  "server": {
    "port": 8080,
    "host": "0.0.0.0",
    "read_timeout": 30,
    "write_timeout": 30
  },
  "storage": {
    "data_dir": "./data",
    "wal_dir": "./wal",
    "max_document_size": 16777216,
    "compression_enabled": true
  },
  "cluster": {
    "mode": "standalone",
    "node_id": "node1",
    "raft_port": 9000,
    "peers": ["node2:9000", "node3:9000"]
  },
  "sharding": {
    "enabled": false,
    "shard_count": 4,
    "replication_factor": 3,
    "max_documents_per_shard": 1000000
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  },
  "security": {
    "auth_enabled": false,
    "tls_enabled": false,
    "tls_cert_file": "",
    "tls_key_file": ""
  }
}
```

### Environment Variables

All configuration options can be overridden using environment variables with the `CAIMANDB_` prefix:

```bash
# Server configuration
export CAIMANDB_SERVER_PORT=9090
export CAIMANDB_SERVER_HOST=127.0.0.1

# Storage configuration
export CAIMANDB_STORAGE_DATA_DIR=/var/lib/caimandb/data
export CAIMANDB_STORAGE_WAL_DIR=/var/lib/caimandb/wal

# Cluster configuration
export CAIMANDB_CLUSTER_MODE=cluster
export CAIMANDB_CLUSTER_NODE_ID=node2
export CAIMANDB_CLUSTER_PEERS=node1:9000,node3:9000

# Logging configuration
export CAIMANDB_LOGGING_LEVEL=debug
```

### Using Custom Configuration

```bash
./bin/caimandb -config /path/to/custom/config.json
```

For a complete reference of all configuration options, refer to [`docs/configuration.md`](docs/configuration.md).

---

## Execution

### Development Mode

```bash
# Development script with hot reload
./scripts/run-dev.sh

# Or manually in development mode
./bin/caimandb -dev
```

### Production Mode

```bash
./bin/caimandb
```

### Execution with Specific Configuration

```bash
./bin/caimandb -config /etc/caimandb/config.json
```

### Docker Deployment

**Building the Image:**
```bash
make docker
# or manually
docker build -t caimandb:latest -f deployments/docker/Dockerfile .
```

**Running with Docker Compose:**
```bash
cd deployments/docker
docker-compose up -d
```

**Status Verification:**
```bash
docker-compose ps
docker-compose logs -f
```

### Running as a Service (Systemd)

Example service file `/etc/systemd/system/caimandb.service`:

```ini
[Unit]
Description=CaimanDB Database Service
After=network.target

[Service]
Type=simple
User=caimandb
Group=caimandb
WorkingDirectory=/opt/caimandb
ExecStart=/opt/caimandb/bin/caimandb -config /etc/caimandb/config.json
Restart=always
RestartSec=10
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

---

## Usage

### Command Line Interface (CLI)

**Starting the Interactive CLI:**
```bash
./bin/caimandb cli
```

**Executing Individual Commands:**
```bash
./bin/caimandb exec "ls"
```

**Administration Commands:**
```bash
# Cluster status
./bin/caimandb cluster status

# List nodes
./bin/caimandb cluster nodes

# Database statistics
./bin/caimandb stats
```

## Documentation

| Document | Description |
|----------|-------------|
| [`docs/architecture.md`](docs/architecture.md) | Complete system architecture, layers, and data flow |
| [`docs/configuration.md`](docs/configuration.md) | Detailed configuration reference and environment variables |
| [`docs/nql-reference.md`](docs/nql-reference.md) | Complete NQL language reference with examples |
| [`docs/api/http-api.md`](docs/api/http-api.md) | HTTP API specification with usage examples |
| [`docs/known-limitations.md`](docs/known-limitations.md) | Known limitations and development roadmap |
| [`examples/quickstart.md`](examples/quickstart.md) | Quick start guide with practical examples |
| [`CHANGELOG.md`](CHANGELOG.md) | Version history and changes made |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Guide for contributing to the project |

---

## Repository Structure

```
caimandb/
├── cmd/
│   └── caimandb/              # Application entry point
├── internal/
│   └── caimandb/              # Engine, server, and CLI (main package)
│       └── parse/             # NQL tokenizer (independent subpackage)
├── configs/
│   ├── caimandb.conf.example  # Configuration template
│   └── caimandb.conf          # Active configuration (auto-generated)
├── deployments/
│   └── docker/
│       ├── Dockerfile
│       └── docker-compose.yml
├── docs/
│   ├── architecture.md
│   ├── configuration.md
│   ├── nql-reference.md
│   ├── known-limitations.md
│   └── api/
│       └── http-api.md
├── examples/
│   └── quickstart.md
├── scripts/
│   ├── build.sh               # Unix build script
│   ├── build.bat              # Windows build script
│   ├── test.sh
│   └── run-dev.sh
├── test/
│   ├── integration/
│   ├── fixtures/
│   └── README.md
├── .github/workflows/ci.yml   # Continuous integration
├── CHANGELOG.md
├── CONTRIBUTING.md
├── LICENSE
├── Makefile
└── go.mod
```

---

## Contributing

We welcome contributions to CaimanDB. To participate:

1. **Fork** the repository
2. Create a **branch** for your feature (`git checkout -b feature/new-feature`)
3. Make your changes and **commit** (`git commit -am 'Add new feature'`)
4. **Push** to the branch (`git push origin feature/new-feature`)
5. Open a **Pull Request**

### Development Guide

```bash
# Install development dependencies
make deps
go mod download

# Run tests with coverage
make test-coverage
go test -cover ./...

# Format code
make fmt
go fmt ./...

# Check code style
make lint
golangci-lint run
```

For more details, refer to [`CONTRIBUTING.md`](CONTRIBUTING.md).

---

## Known Limitations

CaimanDB is under active development. Currently, the main code in `internal/caimandb` remains as a single Go package due to high internal coupling between components. Static analysis shows that over 40 files directly access unexported fields of the central `Engine`, and several subsystems maintain circular references.

The only exception is `internal/caimandb/parse`, which implements the NQL tokenizer and is completely independent.

For more information about this architectural decision and the refactoring roadmap, refer to [`docs/known-limitations.md`](docs/known-limitations.md).

---

## Post-Compilation Verification

This project has undergone structural reorganization. To ensure code integrity in your local environment:

```bash
# Full compilation
go build ./...

# Static analysis
go vet ./...

# Clean and verify dependencies
go mod tidy
go mod verify

# Run unit tests
go test ./...

# Run integration tests
go test -tags=integration ./test/integration/...
```

If any step fails, please report the issue in the issue tracking system.

---

## Support and Community

- **Issue Reporting**: [GitHub Issues](https://github.com/Horinsoft/caimandb/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Horinsoft/caimandb/discussions)
- **Documentation**: [Project Wiki](https://github.com/Horinsoft/caimandb/wiki)
- **Website**: [https://caimandb.io](https://caimandb.io)

---

## License

This project is licensed under the Apache License, Version 2.0. See the [`LICENSE`](LICENSE) file for details.

Copyright 2026 CaimanDB Contributors
**CaimanDB** - Distributed Document Database for the Modern Era
