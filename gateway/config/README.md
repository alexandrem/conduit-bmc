# Gateway Configuration

This directory contains configuration files for the BMC Gateway service.

## Configuration Files

### gateway.yaml.example
Operational configuration for the Gateway service. Copy to `gateway.yaml` and customize as needed.

**Key Configuration Areas:**
- `log`: Logging level, format, and output configuration
- `http`: HTTP server timeouts and settings
- `gateway.proxy`: BMC proxy timeouts and retry configuration
- `gateway.websocket`: WebSocket settings for VNC/console streaming
- `gateway.session_management`: Session TTLs and Redis configuration
- `gateway.webui`: Web UI settings for VNC/console viewers
- `gateway.agent_connections`: Agent connection and load balancing settings
- `gateway.rate_limit`: Rate limiting for different request types
- `auth`: JWT token validation configuration
- `tls`: TLS/SSL configuration (optional)
- `metrics`: Prometheus metrics configuration

### gateway.env.example
Environment variables for the Gateway service. Copy to `gateway.env` and set appropriate values.

**Required Variables:**
- `BMC_MANAGER_ENDPOINT` - Manager service URL (e.g., `http://localhost:8080`)

**Common Variables:**
- `GATEWAY_HOST` - Bind address (default: `0.0.0.0`)
- `GATEWAY_PORT` - Listen port (default: `8081`)
- `GATEWAY_REGION` - Service region identifier (default: `default`)
- `GATEWAY_DATACENTERS` - Comma-separated list of datacenter IDs
- `ENVIRONMENT` - Environment name (`development`, `staging`, `production`)
- `LOG_LEVEL` - Logging level (`debug`, `info`, `warn`, `error`)

**Session Storage:**
- `REDIS_ENDPOINT` - Redis server address (optional, for distributed sessions)
- `REDIS_PASSWORD` - Redis authentication password
- `SESSION_USE_IN_MEMORY_STORE` - Use in-memory storage instead of Redis (default: `true`)

**Web UI Configuration:**
- `WEBUI_ENABLED` - Enable web UI (default: `true`)
- `WEBUI_TITLE` - Browser title for web interface
- `WEBUI_VNC_AUTO_CONNECT` - Auto-connect VNC sessions (default: `true`)

## Setup Instructions

### Development Setup

1. **Copy example files:**
   ```bash
   cd gateway/config
   cp gateway.yaml.example gateway.yaml
   cp gateway.env.example gateway.env
   ```

2. **Edit gateway.env:**
   ```bash
   # Required - Manager service endpoint
   BMC_MANAGER_ENDPOINT=http://localhost:8080

   # Development settings
   GATEWAY_HOST=0.0.0.0
   GATEWAY_PORT=8081
   GATEWAY_REGION=default
   GATEWAY_DATACENTERS=dc-local-01,dc-docker-01

   ENVIRONMENT=development
   LOG_LEVEL=debug

   # Use in-memory session storage for development
   SESSION_USE_IN_MEMORY_STORE=true
   ```

3. **Start the service:**
   ```bash
   cd ../..
   make -C gateway run
   ```

### Production Deployment

1. **Set environment variables via your deployment system:**
   ```bash
   export BMC_MANAGER_ENDPOINT="https://manager.example.com"
   export GATEWAY_REGION=us-east-1
   export GATEWAY_DATACENTERS=dc-us-east-1a,dc-us-east-1b
   export ENVIRONMENT=production
   export LOG_LEVEL=info

   # Use Redis for distributed session storage
   export SESSION_USE_IN_MEMORY_STORE=false
   export REDIS_ENDPOINT=redis.example.com:6379
   export REDIS_PASSWORD=<redis-password>
   ```

2. **Use YAML file for operational configuration:**
   ```yaml
   # gateway.yaml
   gateway:
     rate_limit:
       requests_per_minute: 1000
       vnc_requests_per_minute: 10
     proxy:
       bmc_timeout: 60s
       max_retries: 3
   log:
     level: info
     format: json
   ```

3. **Enable TLS in production:**
   ```bash
   export TLS_ENABLED=true
   export TLS_CERT_FILE=/etc/ssl/certs/gateway.crt
   export TLS_KEY_FILE=/etc/ssl/private/gateway.key
   ```

## Configuration Precedence

Configuration is loaded in order (later overrides earlier):
1. Default values (from struct tags in code)
2. YAML configuration file (`gateway.yaml`)
3. Environment file (`gateway.env`)
4. Environment variables
5. Service-specific environment variables (prefixed with `GATEWAY_`)

Example:
```bash
# Global setting
export LOG_LEVEL=info

# Gateway-specific override (takes precedence)
export GATEWAY_LOG_LEVEL=debug
```

## Session Storage

### In-Memory Storage (Development)
- Fast and simple
- No external dependencies
- Sessions lost on restart
- Not suitable for multi-instance deployments

```bash
SESSION_USE_IN_MEMORY_STORE=true
```

### Redis Storage (Production)
- Distributed session storage
- Sessions persist across restarts
- Supports multiple Gateway instances
- Requires Redis server

```bash
SESSION_USE_IN_MEMORY_STORE=false
REDIS_ENDPOINT=redis.example.com:6379
REDIS_PASSWORD=<secure-password>
REDIS_DATABASE=0
```

## Web UI Configuration

The Gateway serves web-based interfaces for VNC and console access:

```bash
# Enable/disable web UI
WEBUI_ENABLED=true

# Customize appearance
WEBUI_TITLE="BMC Management Console"
WEBUI_THEME_COLOR=#2563eb

# VNC viewer settings
WEBUI_VNC_AUTO_CONNECT=true
WEBUI_VNC_SHOW_PASSWORD=false
```

## WebSocket Configuration

Configure WebSocket settings for VNC streaming:

```bash
# VNC streaming quality
WEBSOCKET_VNC_FRAME_RATE=15  # FPS
WEBSOCKET_VNC_QUALITY=6       # 0-9 (higher = better quality)
WEBSOCKET_VNC_COMPRESSION=2   # 0-9 (higher = more compression)
```

## Security Best Practices

1. **Never commit sensitive values to version control**
   - Add `gateway.env` to `.gitignore`
   - Only commit `.example` files

2. **Enable TLS in production**
   - Always use TLS/SSL for production deployments
   - Use valid certificates from trusted CA

3. **Secure Redis connection**
   - Use strong Redis password
   - Enable Redis TLS if possible
   - Restrict Redis network access

4. **Set appropriate file permissions**
   ```bash
   chmod 600 gateway.env  # Only owner can read/write
   chmod 644 gateway.yaml # Owner can read/write, others can read
   ```

5. **Configure rate limiting**
   - Adjust rate limits based on expected traffic
   - Set lower limits for expensive operations (VNC, console)

## Environment Variable Reference

### Required
| Variable | Description | Example |
|----------|-------------|---------|
| `BMC_MANAGER_ENDPOINT` | Manager service URL | `http://localhost:8080` |

### Service Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_HOST` | `0.0.0.0` | Bind address |
| `GATEWAY_PORT` | `8081` | Listen port |
| `GATEWAY_REGION` | `default` | Service region |
| `GATEWAY_DATACENTERS` | - | Comma-separated datacenter IDs |
| `ENVIRONMENT` | `development` | Environment name |
| `LOG_LEVEL` | `info` | Logging level |

### Session Storage
| Variable | Default | Description |
|----------|---------|-------------|
| `SESSION_USE_IN_MEMORY_STORE` | `true` | Use in-memory storage |
| `REDIS_ENDPOINT` | - | Redis server address |
| `REDIS_PASSWORD` | - | Redis password |
| `REDIS_DATABASE` | `0` | Redis database number |

### Session Management
| Variable | Default | Description |
|----------|---------|-------------|
| `SESSION_PROXY_TTL` | `1h` | Proxy session TTL |
| `SESSION_VNC_TTL` | `4h` | VNC session TTL |
| `SESSION_CONSOLE_TTL` | `2h` | Console session TTL |
| `SESSION_TOKEN_LENGTH` | `32` | Session token length |

### WebSocket Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `WEBSOCKET_VNC_FRAME_RATE` | `15` | VNC frame rate (FPS) |
| `WEBSOCKET_VNC_QUALITY` | `6` | VNC quality (0-9) |
| `WEBSOCKET_VNC_COMPRESSION` | `2` | VNC compression (0-9) |

### Web UI Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `WEBUI_ENABLED` | `true` | Enable web UI |
| `WEBUI_TITLE` | `BMC Management Console` | Browser title |
| `WEBUI_THEME_COLOR` | `#2563eb` | Theme color |
| `WEBUI_VNC_AUTO_CONNECT` | `true` | Auto-connect VNC |
| `WEBUI_VNC_SHOW_PASSWORD` | `false` | Show VNC password |

### Proxy Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_BMC_TIMEOUT` | `60s` | BMC operation timeout |
| `PROXY_IPMI_TIMEOUT` | `30s` | IPMI operation timeout |
| `PROXY_REDFISH_TIMEOUT` | `45s` | Redfish operation timeout |
| `PROXY_MAX_RETRIES` | `3` | Maximum retry attempts |

### Agent Connections
| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_MAX_CONNECTIONS` | `100` | Max agent connections |
| `AGENT_CONNECTION_TIMEOUT` | `30s` | Connection timeout |
| `AGENT_HEARTBEAT_INTERVAL` | `30s` | Heartbeat interval |
| `LOAD_BALANCER_ALGORITHM` | `round_robin` | Load balancing algorithm |

### Rate Limiting
| Variable | Default | Description |
|----------|---------|-------------|
| `RATE_LIMIT_ENABLED` | `true` | Enable rate limiting |
| `RATE_LIMIT_REQUESTS_PER_MINUTE` | `1000` | Global rate limit |
| `RATE_LIMIT_VNC_REQUESTS_PER_MINUTE` | `10` | VNC session rate limit |
| `RATE_LIMIT_CONSOLE_REQUESTS_PER_MINUTE` | `20` | Console session rate limit |

### Metrics
| Variable | Default | Description |
|----------|---------|-------------|
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `METRICS_PORT` | `9091` | Metrics server port |

See `gateway.env.example` for complete list of available variables.
