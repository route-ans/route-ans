# ANS Resolution Server

A high-performance, cryptographically-verified resolution service for the Agent Name Service (ANS) ecosystem. The resolution server translates ANSName identifiers into verified agent endpoints, acting as the "DNS resolver" layer for autonomous AI agents.

## Overview

The ANS Resolution Server provides:

- **Fast Resolution**: Sub-50ms cache-hit lookups, <100ms P99 latency
- **Cryptographic Verification**: Mandatory signature and Merkle proof validation
- **Protocol Agnostic**: Supports A2A, MCP, ACP, HTTPS, and custom protocols
- **Pluggable Architecture**: Swap cache, queue, store, and registry providers
- **Cloud Native**: Declarative YAML configuration, Kubernetes-ready
- **High Availability**: Stateless design for horizontal scaling

## Architecture

```
┌──────────────┐
│ Client Agent │
└──────┬───────┘
       │ ANS Query: GET /v1/resolve?name=mcp://...
       ▼
┌────────────────────────────┐
│   ANS Resolution Server    │
│────────────────────────────│
│ 1. API Gateway (Chi)       │
│ 2. Cache Layer (Pluggable) │
│ 3. ANSName Parser          │
│ 4. Registry Selector       │
│ 5. Registry Adapters       │
│ 6. Trust Verifier          │
│ 7. Policy Engine           │
└────────┬───────────┬───────┘
         │           │
         ▼           ▼
 ┌─────────────┐  ┌──────────────┐
 │ Registry A  │  │ Registry B   │
 │ (GoDaddy)   │  │ (Veritrust)  │
 └─────────────┘  └──────────────┘
```

## Quick Start

### Prerequisites

- Go 1.25+
- GoDaddy API credentials (for production) or use mock registry (for testing)
- Optional: Redis for distributed caching

### Running Locally

```bash
# Clone the repository
git clone https://github.com/route-ans/route-ans.git
cd route-ans

# Install dependencies
go mod download

# Run with default config (mock registry)
make run

# Or build and run
make build
./bin/ans-resolver --config configs/resolver-minimal.yaml
```

### Using GoDaddy Registry

1. **Get API Credentials**: Sign up at [GoDaddy Developer Portal](https://developer.godaddy.com/keys)

2. **Set Environment Variables**:
```bash
export GODADDY_API_KEY="your_api_key"
export GODADDY_API_SECRET="your_api_secret"
```

3. **Start the Resolver**:
```bash
./bin/ans-resolver --config configs/resolver-godaddy.yaml
```

4. **Test Resolution**:
```bash
# Resolve an agent registered in GoDaddy
curl "http://localhost:8080/v1/resolve?name=a2a://greeting.greet.PID-1234.v1.0.0.neelanjan.dev"
```

### Registering Your Agent

Generate certificates and register with GoDaddy:

```bash
# Generate CSRs for your agent
openssl req -new -newkey rsa:2048 -nodes \
  -keyout agent-server.key -out agent-server.csr \
  -subj "/CN=myagent.example.com/O=MyCompany/C=US"

openssl req -new -newkey rsa:2048 -nodes \
  -keyout agent-identity.key -out agent-identity.csr \
  -subj "/CN=a2a:\/\/myagent.capability.PID-XXX.v1.0.0.example.com/O=MyCompany/C=US"

# Register with GoDaddy
curl -X POST "https://api.godaddy.com/v1/agents/register" \
  -H "Authorization: sso-key YOUR_API_KEY:YOUR_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "a2a",
    "agentName": "myagent",
    "agentCapability": "capability",
    "provider": "PID-XXX",
    "version": "1.0.0",
    "extension": "example.com",
    "serverCsrPEM": "BASE64_ENCODED_CSR",
    "identityCsrPEM": "BASE64_ENCODED_CSR",
    "agentCategory": "AI/ML"
  }'
```

See [AGENT_REGISTRATION.md](AGENT_REGISTRATION.md) for detailed registration instructions.

### Using Docker

```bash
# Build the image
make docker-build

# Run the container
docker run -p 8080:8080 -p 9091:9091 ans-resolution-server:latest
```

### Using Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -k deployments/kubernetes/
```

## API Reference

### Resolve an ANSName

Resolve an agent's ANSName to its verified endpoint:

```bash
GET /v1/resolve?name=a2a://greeting.greet.PID-1234.v1.0.0.neelanjan.dev
```

**Response:**
```json
{
  "status": "verified",
  "agent": "a2a://greeting.greet.PID-1234.v1.0.0.neelanjan.dev",
  "protocol": "a2a",
  "endpoint": "https://greeting.neelanjan.dev",
  "expiresAt": "2026-03-05T10:45:00Z",
  "protocolExtensions": {
    "a2a": {
      "url": "https://greeting.neelanjan.dev/agent"
    }
  }
}
```

### Swagger UI

Interactive API documentation available at:
```
http://localhost:8080/swagger/
```

Or generate updated docs:
```bash
make docs
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Verified - agent found and trusted |
| 400 | Bad Request - invalid ANSName format |
| 404 | Not Found - agent not registered |
| 421 | Untrusted Agent - signature verification failed |
| 422 | Certificate Revoked |
| 429 | Rate Limited |
| 500 | Internal Server Error |

### Other Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /ready` | Readiness check |
| `GET /metrics` | Prometheus metrics |
| `POST /v1/resolve/batch` | Batch resolution |
| `GET /v1/agent/{ansName}` | Get agent details |
| `GET /v1/agent/{ansName}/verify` | Explicit verification |
| `GET /v1/search?q=...` | Search agents |

## Configuration

The server uses a declarative YAML configuration with environment variable substitution:

```yaml
apiVersion: ans.io/v1alpha1
kind: ResolverConfig
metadata:
  name: ans-resolver

spec:
  server:
    http:
      port: ${HTTP_PORT:8080}
      readTimeout: 30s

  # Pluggable cache provider
  cache:
    provider: ${CACHE_PROVIDER:memory}  # memory | redis | memcached
    ttl:
      default: 5m
      verified: 10m
    redis:
      address: ${REDIS_ADDRESS:localhost:6379}

  # Pluggable queue provider
  queue:
    provider: ${QUEUE_PROVIDER:memory}  # memory | redis-streams | kafka | nats

  # Pluggable store provider
  store:
    provider: ${STORE_PROVIDER:memory}  # memory | postgres | sqlite

  # Registry adapters
  registries:
    - name: primary
      type: ans-registry
      config:
        baseUrl: https://registry.ans.io

  # Trust verification
  trust:
    provider: file  # file | vault | k8s-secret
    verification:
      requireSignature: true
      requireMerkleProof: true
      checkRevocation: true
```

See `configs/resolver.yaml` for the complete configuration reference.

## Pluggable Providers

### Cache Providers

| Provider | Description | Config Key |
|----------|-------------|------------|
| `memory` | In-memory LRU cache | Default |
| `redis` | Redis cache | `cache.redis.*` |
| `memcached` | Memcached | `cache.memcached.*` |

### Queue Providers

| Provider | Description | Config Key |
|----------|-------------|------------|
| `memory` | In-memory channel | Default |
| `redis-streams` | Redis Streams | `queue.redis-streams.*` |
| `kafka` | Apache Kafka | `queue.kafka.*` |
| `nats` | NATS JetStream | `queue.nats.*` |

### Store Providers

| Provider | Description | Config Key |
|----------|-------------|------------|
| `memory` | In-memory map | Default |
| `postgres` | PostgreSQL | `store.postgres.*` |
| `sqlite` | SQLite | `store.sqlite.*` |

### Registry Adapters

| Adapter | Description | Status |
|---------|-------------|--------|
| `godaddy` | GoDaddy ANS Registry | ✅ Implemented |
| `mock` | Mock adapter for testing | ✅ Implemented |
| `veritrust` | DID/Verifiable Credentials | 🔜 Planned |

**Using GoDaddy Registry:**
```yaml
registries:
  - name: godaddy
    type: godaddy
    enabled: true
    config:
      baseURL: "https://api.godaddy.com/v1"
      apiKey: "${GODADDY_API_KEY}"
      apiSecret: "${GODADDY_API_SECRET}"
```

### Trust Providers

| Provider | Description |
|----------|-------------|
| `file` | File-based trust store |
| `vault` | HashiCorp Vault |
| `k8s-secret` | Kubernetes Secret |

## Observability

### Metrics (Prometheus)

```bash
# Available at /metrics or port 9091
curl http://localhost:9091/metrics
```

Key metrics:
- `ans_resolver_resolution_total` - Resolution requests by status
- `ans_resolver_resolution_duration_seconds` - Resolution latency histogram
- `ans_resolver_cache_hits_total` - Cache hit count
- `ans_resolver_registry_lookups_total` - Registry lookup count

### Tracing (OpenTelemetry)

Enable tracing in config:

```yaml
telemetry:
  tracing:
    enabled: true
    provider: otlp
    sampleRate: 0.1
    otlp:
      endpoint: localhost:4317
```

### Logging

```yaml
telemetry:
  logging:
    level: info  # debug | info | warn | error
    format: json  # json | text | pretty
```

## Development

### Prerequisites

- Go 1.22+
- Docker (optional)
- Redis (for redis providers)

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Run linter
make lint
```

### Project Structure

```
ans-resolution-server/
├── cmd/resolver/          # Entry point
├── internal/
│   ├── cache/             # Cache provider interface & implementations
│   ├── config/            # Configuration loading
│   ├── queue/             # Queue provider interface & implementations
│   ├── registry/          # Registry adapter interface & implementations
│   ├── resolver/          # Core resolution logic
│   ├── server/            # HTTP server
│   ├── store/             # Store provider interface & implementations
│   ├── telemetry/         # Logging, metrics, tracing
│   └── trust/             # Trust verification
├── pkg/ansname/           # Public ANSName parser
├── configs/               # Configuration files
├── deployments/
│   ├── docker/            # Dockerfile
│   └── kubernetes/        # K8s manifests
└── api/                   # API specifications
```

## License

Apache 2.0 - See [LICENSE](../LICENSE)

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md)
