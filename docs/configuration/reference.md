# Configuration Reference

Complete API reference for all ANS Resolution Server configuration options.

!!! info "Implementation Status"
    Some configuration options are designed for future features. These are marked with badges:
    
    - 🟢 **Implemented** - Fully functional
    - 🟡 **Partial** - Basic implementation, limited functionality
    - 🔴 **Future** - Not yet implemented, reserved for future use
    - ⚠️ **Optional** - Can be disabled/omitted

## Overview

The resolver uses YAML configuration with environment variable substitution.

**Format:** `${VAR_NAME:default_value}`

**Example:**

```yaml
apiKey: "${GODADDY_API_KEY:your-key-here}"
```

## Configuration Structure

```yaml
server:         🟢 HTTP/gRPC server settings
cache:          🟢 Cache provider configuration
queue:          🟢 Message queue (disabled by default)
store:          🟡 Data store (basic search only)
registries:     🟢 Registry adapters (GoDaddy, mock)
trust:          🟢 Trust and verification (can disable)
rateLimit:      🟢 Rate limiting
telemetry:      🟢 Logging, metrics, tracing
```

---

## Complete Configuration Reference

### `server` - Server Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Configures the HTTP and optional gRPC servers that handle incoming resolution requests.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `http` | HTTPConfig | - | HTTP server configuration | 🟢 Required |
| `grpc` | GRPCConfig | - | gRPC server (optional, for future use) | 🔴 Future |
| `tls` | TLSConfig | - | TLS configuration (HTTPS) | ⚠️ Optional |
| `gracefulShutdownTimeout` | duration | 30s | Time to wait for connections to close during shutdown | 🟢 |

**Implementation Notes:**

- HTTP server is the primary interface
- gRPC support is planned but not implemented
- TLS can be configured but typically handled by reverse proxy

**Example:**
```yaml
server:
  http:
    host: 0.0.0.0
    port: 8080
    readTimeout: 30s
    writeTimeout: 30s
    idleTimeout: 120s
  gracefulShutdownTimeout: 30s
```

---

### `server.http` - HTTP Server Settings

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Controls the HTTP server that exposes the REST API and Swagger UI.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `host` | string | 0.0.0.0 | Network interface to bind to (0.0.0.0 = all interfaces) | 🟢 |
| `port` | int | 8080 | TCP port to listen on | 🟢 |
| `readTimeout` | duration | 30s | Maximum time to read the entire request | 🟢 |
| `writeTimeout` | duration | 30s | Maximum time to write the response | 🟢 |
| `idleTimeout` | duration | 120s | Maximum time to wait for next request when keep-alive is enabled | 🟢 |
| `maxHeaderBytes` | int | 1048576 | Maximum size of request headers (1MB) | 🟢 |

**Performance Tips:**

- Increase `readTimeout`/`writeTimeout` if resolution takes >30s (e.g., slow DNS)
- Decrease `idleTimeout` to free connections faster under high load
- Increase `maxHeaderBytes` only if you need larger headers (rarely needed)

---

### `cache` - Cache Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Configures caching to speed up repeated resolution requests. Critical for performance.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `provider` | string | memory | Cache backend: `memory` (in-process) or `redis` (distributed) | 🟢 |
| `ttl` | TTLConfig | - | Time-to-live settings for different record types | 🟢 |
| `memory` | MemoryCacheConfig | - | In-memory cache settings (used when provider=memory) | 🟢 |
| `redis` | RedisCacheConfig | - | Redis cache settings (used when provider=redis) | 🟢 |

**Cache Strategy:**

- **Cache hit**: Sub-millisecond response time
- **Cache miss**: 100-500ms (registry lookup + verification)
- **Recommended**: Start with `memory`, upgrade to `redis` for multi-instance deployments

**Example:**
```yaml
cache:
  provider: memory
  ttl:
    default: 300s      # 5 minutes - general TTL
    verified: 600s     # 10 minutes - verified agents cached longer
    revoked: 3600s     # 1 hour - revoked agents cached to prevent repeated checks
    notFound: 60s      # 1 minute - not-found results cached briefly
  memory:
    maxSize: 10000
    cleanupInterval: 60s
```

**When to use Redis:**

- Running multiple resolver instances
- Need cache sharing across instances
- Handling >1000 requests/second
- Want persistent cache across restarts

---

### `cache.ttl` - Cache TTL Settings

🟢 **Status:** Fully Implemented  
**Type:** Object

Controls how long different types of records are cached before being re-fetched from the registry.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `default` | duration | 300s | Default TTL for records without specific status | 🟢 |
| `verified` | duration | 600s | TTL for successfully verified agent records | 🟢 |
| `revoked` | duration | 3600s | TTL for revoked agents (cached longer to avoid repeated lookups) | 🟢 |
| `notFound` | duration | 60s | TTL for "agent not found" results (short to catch new registrations) | 🟢 |

**TTL Strategy:**

- **Longer TTL** = Better performance, less registry load, potentially stale data
- **Shorter TTL** = More up-to-date, higher registry load, slower responses
- **Revoked agents** cached longest - revocation rarely changes
- **Not-found** cached shortest - agents might get registered

---

### `cache.redis` - Redis Cache Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Used when:** `cache.provider: redis`

Redis configuration for distributed caching across multiple resolver instances.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `address` | string | localhost:6379 | Redis server address (host:port) | 🟢 |
| `password` | string | "" | Redis authentication password | 🟢 |
| `db` | int | 0 | Redis database number (0-15) | 🟢 |
| `maxRetries` | int | 3 | Max retry attempts for failed operations | 🟢 |
| `poolSize` | int | 10 | Max connections in pool | 🟢 |
| `minIdleConns` | int | 5 | Min idle connections kept open | 🟢 |
| `dialTimeout` | duration | 5s | Timeout for establishing connection | 🟢 |
| `readTimeout` | duration | 3s | Timeout for read operations | 🟢 |
| `writeTimeout` | duration | 3s | Timeout for write operations | 🟢 |
| `poolTimeout` | duration | 4s | Timeout waiting for connection from pool | 🟢 |
| `idleTimeout` | duration | 5m | Timeout before closing idle connections | 🟢 |
| `keyPrefix` | string | ans:cache: | Prefix for all cache keys (prevents collisions) | 🟢 |

**Example:**
```yaml
cache:
  provider: redis
  redis:
    address: "${REDIS_ADDRESS:localhost:6379}"
    password: "${REDIS_PASSWORD:}"
    db: 0
    poolSize: 20        # Increase for high-traffic deployments
    keyPrefix: ans:cache:
```

**Performance Tuning:**

- Increase `poolSize` for higher concurrency
- Adjust timeouts based on network latency to Redis
- Use separate Redis instance from application data
- Monitor connection pool usage

---

### `queue` - Queue Configuration

🟢 **Status:** Implemented - Disabled by Default  
**Type:** Object  
**Required:** Yes

!!! info "Optional Feature"
    The queue system is **fully implemented** and processes registry events for cache invalidation and metrics tracking. It's disabled by default since most deployments don't need it. Enable it when you have registry event sources (webhooks, Redis streams) or need distributed cache coordination.

Message queue configuration for handling asynchronous registry events.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | **false** | Enable/disable queue event processing | 🟢 ⚠️ Disabled by default |
| `provider` | string | memory | Queue provider: `memory` (🟢), `redis-streams` (🟡 config only) | 🟢 |
| `bufferSize` | int | 1000 | Buffer size for in-memory queue | 🟢 |
| `redis-streams` | RedisStreamsConfig | - | Redis Streams settings | 🟡 Config only |

**What it does when enabled:**

1. **Cache invalidation** - Automatically invalidates cache when registry events arrive
2. **Event processing** - Handles registered, renewed, revoked, deprecated, expired events
3. **Metrics tracking** - Records queue statistics (processed, failed, pending)
4. **Background processing** - Async event handling without blocking resolution

**Event Types Processed:**

- `registered` - Cache invalidated, metrics recorded
- `renewed` - Cache invalidated for fresh lookup
- `revoked` - Cache entry removed immediately
- `deprecated` - Cache entry removed
- `expired` - Cache entry removed

**Default Configuration (Disabled):**
```yaml
queue:
  enabled: false      # No overhead when disabled
  provider: memory    # Use in-memory queue for testing
  bufferSize: 1000
```

**When to enable:**

- Registry provides event webhooks or SSE streams
- Running multiple resolver instances needing cache coordination
- Using Redis Streams for distributed event processing
- Need metrics on registry event activity

**Enable for testing:**
```yaml
queue:
  enabled: true
  provider: memory
  bufferSize: 100
```

---

### `store` - Data Store Configuration

🟡 **Status:** Partial Implementation - Basic Functionality Only  
**Type:** Object  
**Required:** Yes

!!! info "Limited Functionality"
    The store is used for `/search` and `/discover` endpoints. With the in-memory provider, it only contains agents that were previously resolved, making search results incomplete. Full-text search requires PostgreSQL (not implemented).

Data store for agent records, used by search/discovery endpoints.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `provider` | string | memory | Store provider: `memory` (working), `postgres`/`sqlite` (future) | 🟡 |
| `memory` | MemoryStoreConfig | - | In-memory store settings | 🟡 |
| `postgres` | PostgresConfig | - | PostgreSQL settings (full-text search) | 🔴 Future |

**How it works:**

1. When an agent is resolved, it's asynchronously saved to the store (goroutine)
2. Search endpoints query the store
3. With memory provider: Only previously resolved agents are searchable
4. With postgres provider: Could support full agent catalog (not implemented)

**Current Limitations:**

- No full-text indexing
- Limited to ~10k agents in memory
- Search only works for cached/resolved agents
- Results inconsistent across multiple instances

**Example:**
```yaml
store:
  provider: memory
  memory:
    maxSize: 100000    # Max agents to keep in memory
```

**Future Enhancement:**

- PostgreSQL backend for persistent storage
- Full-text search across all registered agents
- Advanced filtering (tags, capabilities, protocols)
- Distributed search across multiple registries

---

### `registries` - Registry Adapters

🟢 **Status:** Fully Implemented (GoDaddy, Mock)  
**Type:** Array of RegistryConfig  
**Required:** Yes (at least one)

!!! success "Working Registries"
    - ✅ **GoDaddy** - Fully functional, production-ready
    - ✅ **Mock** - For testing/development

Configures connections to ANS registries where agents are registered.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `name` | string | - | Human-readable registry identifier | 🟢 |
| `type` | string | - | Registry type: `godaddy` (working), `mock` (working) | 🟢 |
| `enabled` | bool | true | Enable/disable this registry | 🟢 |
| `priority` | int | 1 | Priority when multiple registries configured (lower = higher) | 🟢 |
| `timeout` | duration | 10s | Max time for registry API calls | 🟢 |
| `retries` | int | 3 | Max retry attempts on transient failures | 🟢 |
| `retryBackoff` | duration | 1s | Initial backoff duration (exponential backoff used) | 🟢 |
| `config` | map | - | Registry-specific configuration (see below) | 🟢 |

**Multi-Registry Support:**
You can configure multiple registries with different priorities. The resolver will try them in priority order until one succeeds.

**Example:**
```yaml
registries:
  - name: godaddy-primary
    type: godaddy
    enabled: true
    priority: 1
    timeout: 10s
    retries: 3
    retryBackoff: 1s
    config:
      baseURL: "https://api.godaddy.com/v1"
      apiKey: "${GODADDY_API_KEY}"
      apiSecret: "${GODADDY_API_SECRET}"
```

### `registries[].config` - GoDaddy Registry

**Type:** Object  
**When:** `type: godaddy`

| Field | Type | Required | Description | Status |
|-------|------|----------|-------------|--------|
| `baseURL` | string | Yes | GoDaddy API base URL | 🟢 |
| `apiKey` | string | Yes | API key from developer portal | 🟢 |
| `apiSecret` | string | Yes | API secret | 🟢 |

**URLs:**

- Production: `https://api.godaddy.com/v1`
- OTE (test): `https://api.ote-godaddy.com/v1`

**Get credentials:** https://developer.godaddy.com/keys

---

### `trust` - Trust and Verification

**Type:** Object  
**Required:** Yes

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `provider` | string | file | Trust provider: `file`, `vault`, `k8s-secret` | 🟢 |
| `verification` | VerificationConfig | - | Verification settings | 🟢 |
| `file` | FileTrustConfig | - | File-based trust settings | 🟢 |

**Example:**
```yaml
trust:
  provider: file
  verification:
    enabled: true
    mode: permissive
    requireSignature: true
    requireMerkleProof: false
    checkRevocation: true
```

---

### `trust.verification` - Verification Settings

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | true | Enable trust verification | 🟢 |
| `mode` | string | permissive | Mode: `strict`, `permissive`, `disabled` | 🟢 |
| `requireSignature` | bool | true | Require registry signature | 🟢 |
| `requireMerkleProof` | bool | false | Require Merkle inclusion proof | 🔴 Future |
| `checkRevocation` | bool | true | Check certificate revocation | 🟢 |
| `allowExpiredGracePeriod` | duration | 24h | Grace period for expired certs | 🟢 |
| `ocsp` | OCSPConfig | - | OCSP settings | 🟢 |
| `crl` | CRLConfig | - | CRL settings | 🟢 |

**Modes:**

- `strict` - All checks must pass, fail on error
- `permissive` - Log failures but continue
- `disabled` - Skip all verification (NOT ANS compliant)

**⚠️ Security Warning:** Only use `enabled: false` or `mode: disabled` for development/testing!

---

### `rateLimit` - Rate Limiting

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | true | Enable rate limiting | 🟢 |
| `limits` | map | - | Limit profiles | 🟢 |

**Example:**
```yaml
rateLimit:
  enabled: true
  limits:
    anonymous:
      requests: 100
      window: 1m
    authenticated:
      requests: 1000
      window: 1m
```

---

### `telemetry` - Telemetry Configuration

**Type:** Object

| Field | Type | Description | Status |
|-------|------|-------------|--------|
| `logging` | LoggingConfig | Logging settings | 🟢 |
| `metrics` | MetricsConfig | Prometheus metrics | 🟢 |
| `tracing` | TracingConfig | Distributed tracing | 🔴 Future |

**Example:**
```yaml
telemetry:
  logging:
    level: info
    format: json
    output: stdout
  metrics:
    enabled: true
    port: 9091
  tracing:
    enabled: false
```

---

### `telemetry.logging` - Logging Configuration

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `level` | string | info | Log level: `debug`, `info`, `warn`, `error` | 🟢 |
| `format` | string | json | Format: `json`, `text` | 🟢 |
| `output` | string | stdout | Output: `stdout`, `stderr`, `file` | 🟢 |

---

### `telemetry.metrics` - Metrics Configuration

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | true | Enable Prometheus metrics | 🟢 |
| `namespace` | string | ans | Metric namespace | 🟢 |
| `subsystem` | string | resolver | Metric subsystem | 🟢 |
| `path` | string | /metrics | Metrics endpoint path | 🟢 |
| `port` | int | 9091 | Metrics server port | 🟢 |
| `host` | string | 0.0.0.0 | Metrics server host | 🟢 |

---

## Environment Variables

All string values support environment variable substitution:

**Syntax:** `${VAR_NAME:default_value}`

**Examples:**
```yaml
# Required variable (no default)
apiKey: "${GODADDY_API_KEY}"

# With default value
port: "${HTTP_PORT:8080}"
host: "${HTTP_HOST:0.0.0.0}"

# Redis connection
address: "${REDIS_ADDRESS:localhost:6379}"
password: "${REDIS_PASSWORD:}"
```

**Commonly used variables:**

- `GODADDY_API_KEY` - GoDaddy API key
- `GODADDY_API_SECRET` - GoDaddy API secret
- `REDIS_ADDRESS` - Redis server address
- `REDIS_PASSWORD` - Redis password
- `HTTP_PORT` - HTTP server port
- `LOG_LEVEL` - Logging level

---

## Complete Example Configurations

See example configurations in `configs/`:

| File | Description | Status |
|------|-------------|--------|
| `resolver-minimal-godaddy.yaml` | Minimal config with only essentials | 🟢 Recommended |
| `resolver-godaddy.yaml` | Standard GoDaddy config with in-memory cache | 🟢 Production-ready |
| `resolver-godaddy-redis.yaml` | GoDaddy with Redis cache (queue disabled) | 🟢 Production-ready |
| `resolver-queue-test.yaml` | Test config with queue enabled | 🟢 Development/Testing |
| `resolver-minimal.yaml` | Mock registry for testing | 🟢 Development |

---

## Validation

**Validate your config:**
```bash
# Check if config loads without errors
./bin/ans-resolver -config configs/your-config.yaml -validate

# Dry-run (load config but don't start server)
./bin/ans-resolver -config configs/your-config.yaml -dry-run
```

**IDE Validation:**

VS Code users can get autocomplete and validation by adding to `.vscode/settings.json`:
```json
{
  "yaml.schemas": {
    "./docs/config-schema.json": "configs/*.yaml"
  }
}
```

---

## Duration Format

All duration fields use Go duration format:

**Units:**

- `ns` - nanoseconds
- `us` - microseconds
- `ms` - milliseconds
- `s` - seconds
- `m` - minutes
- `h` - hours

**Examples:**

- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h30m` - 1.5 hours
- `100ms` - 100 milliseconds

---

## Need Help?

- **Configuration issues:** Check logs for validation errors
- **GoDaddy setup:** See `docs/AGENT_REGISTRATION.md`
- **Feature flags:** See `docs/FEATURE_FLAGS.md`
- **API documentation:** https://route-ans.github.io/route-ans
