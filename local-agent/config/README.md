# Local Agent Configuration

This directory contains configuration files for the BMC Local Agent service.

## Configuration Files

### agent.yaml.example
Operational configuration for the Local Agent service. Copy to `agent.yaml` and customize as needed.

**Key Configuration Areas:**
- `log`: Logging level, format, and output configuration
- `http`: HTTP server timeouts and settings
- `agent.bmc_discovery`: BMC discovery and network scanning configuration
- `agent.bmc_operations`: BMC operation timeouts and concurrency settings
- `agent.bmc_operations.ipmi`: IPMI-specific configuration
- `agent.bmc_operations.redfish`: Redfish-specific configuration
- `agent.vnc`: VNC/KVM console configuration
- `agent.serial_console`: Serial console (SOL) configuration
- `agent.connection_management`: Gateway connection and heartbeat settings
- `agent.health_monitoring`: Health check and monitoring configuration
- `agent.security`: Network access control and audit logging
- `static.hosts`: Static BMC host configuration (legacy)
- `tls`: TLS/SSL configuration (optional)
- `metrics`: Prometheus metrics configuration

### agent.env.example
Environment variables for the Local Agent service. Copy to `agent.env` and set appropriate values.

**Required Variables:**
- `AGENT_ID` - Unique agent identifier (e.g., `agent-001`)
- `AGENT_NAME` - Human-readable agent name (e.g., `DC Local Agent 01`)
- `AGENT_DATACENTER_ID` - Datacenter identifier (e.g., `dc-local-01`)
- `AGENT_GATEWAY_ENDPOINT` - Gateway service URL (e.g., `http://localhost:8081`)
- `AGENT_ENCRYPTION_KEY` - Encryption key (minimum 32 characters, use strong random value)

**Common Variables:**
- `AGENT_REGION` - Region identifier (default: `default`)
- `ENVIRONMENT` - Environment name (`development`, `staging`, `production`)
- `LOG_LEVEL` - Logging level (`debug`, `info`, `warn`, `error`)

**BMC Discovery:**
- `BMC_DISCOVERY_ENABLED` - Enable automatic BMC discovery (default: `true`)
- `BMC_DISCOVERY_NETWORK_RANGES` - Comma-separated CIDR ranges to scan
- `BMC_DISCOVERY_SCAN_INTERVAL` - How often to scan (default: `5m`)

**BMC Operations:**
- `BMC_OPERATION_TIMEOUT` - General operation timeout (default: `30s`)
- `BMC_POWER_OPERATION_TIMEOUT` - Power operation timeout (default: `60s`)
- `BMC_CONSOLE_TIMEOUT` - Console session timeout (default: `300s`)

**IPMI Configuration:**
- `IPMI_INTERFACE` - IPMI interface type (default: `lanplus`)
- `IPMI_CIPHER_SUITE` - Cipher suite to use (default: `3`)
- `IPMI_SOL_BAUD_RATE` - SOL baud rate (default: `115200`)

**Redfish Configuration:**
- `REDFISH_HTTP_TIMEOUT` - HTTP request timeout (default: `30s`)
- `REDFISH_INSECURE_SKIP_VERIFY` - Skip TLS verification (default: `false`)

**VNC Configuration:**
- `VNC_ENABLED` - Enable VNC support (default: `true`)
- `VNC_MAX_CONNECTIONS` - Max concurrent VNC connections (default: `5`)

**Serial Console Configuration:**
- `SERIAL_CONSOLE_ENABLED` - Enable serial console support (default: `true`)
- `SERIAL_CONSOLE_MAX_SESSIONS` - Max concurrent console sessions (default: `10`)

## Setup Instructions

### Development Setup

1. **Copy example files:**
   ```bash
   cd local-agent/config
   cp agent.yaml.example agent.yaml
   cp agent.env.example agent.env
   ```

2. **Generate secure encryption key:**
   ```bash
   openssl rand -hex 32
   ```

3. **Edit agent.env:**
   ```bash
   # Required - Agent identification
   AGENT_ID=agent-001
   AGENT_NAME=DC Local Agent 01
   AGENT_DATACENTER_ID=dc-local-01
   AGENT_REGION=default

   # Required - Gateway connection
   AGENT_GATEWAY_ENDPOINT=http://localhost:8081

   # Required - Security (use generated key from step 2)
   AGENT_ENCRYPTION_KEY=<generated-encryption-key>

   # Development settings
   ENVIRONMENT=development
   LOG_LEVEL=debug

   # BMC discovery - adjust network ranges to match your environment
   BMC_DISCOVERY_ENABLED=true
   BMC_DISCOVERY_NETWORK_RANGES=192.168.1.0/24,10.0.0.0/24
   BMC_DISCOVERY_SCAN_INTERVAL=5m
   ```

4. **Start the service:**
   ```bash
   cd ../..
   make -C local-agent run
   ```

### Production Deployment

1. **Set environment variables via your deployment system:**
   ```bash
   # Agent identification
   export AGENT_ID="agent-prod-001"
   export AGENT_NAME="Production Agent - DC1"
   export AGENT_DATACENTER_ID="dc-us-east-1"
   export AGENT_REGION=us-east-1

   # Gateway connection
   export AGENT_GATEWAY_ENDPOINT="https://gateway.example.com"

   # Security
   export AGENT_ENCRYPTION_KEY="$(openssl rand -hex 32)"

   # Environment
   export ENVIRONMENT=production
   export LOG_LEVEL=info

   # BMC discovery - production networks only
   export BMC_DISCOVERY_ENABLED=true
   export BMC_DISCOVERY_NETWORK_RANGES=10.10.0.0/16,10.20.0.0/16
   ```

2. **Use YAML file for operational configuration:**
   ```yaml
   # agent.yaml
   agent:
     bmc_discovery:
       scan_interval: 10m
       max_concurrent: 100
     bmc_operations:
       max_concurrent_operations: 20
   log:
     level: info
     format: json
   ```

3. **Enable TLS in production:**
   ```bash
   export TLS_ENABLED=true
   export TLS_CERT_FILE=/etc/ssl/certs/agent.crt
   export TLS_KEY_FILE=/etc/ssl/private/agent.key
   ```

4. **Configure security settings:**
   ```bash
   # Network access control
   export SECURITY_ALLOWED_NETWORKS=10.0.0.0/8,172.16.0.0/12

   # Enable audit logging
   export SECURITY_ENABLE_AUDIT_LOGGING=true
   export SECURITY_AUDIT_LOG_PATH=/var/log/bmc-agent/audit.log
   ```

## Configuration Precedence

Configuration is loaded in order (later overrides earlier):
1. Default values (from struct tags in code)
2. YAML configuration file (`agent.yaml`)
3. Environment file (`agent.env`)
4. Environment variables
5. Service-specific environment variables (prefixed with `AGENT_`)

Example:
```bash
# Global setting
export LOG_LEVEL=info

# Agent-specific override (takes precedence)
export AGENT_LOG_LEVEL=debug
```

## BMC Discovery

The agent can automatically discover BMC endpoints in your network:

### Automatic Discovery (Recommended)
```bash
# Enable discovery
BMC_DISCOVERY_ENABLED=true

# Specify network ranges to scan (CIDR notation)
BMC_DISCOVERY_NETWORK_RANGES=192.168.1.0/24,10.0.0.0/24

# Configure scan parameters
BMC_DISCOVERY_SCAN_INTERVAL=5m
BMC_DISCOVERY_SCAN_TIMEOUT=10s
BMC_DISCOVERY_MAX_CONCURRENT=50
```

### Static Configuration (Legacy)
For environments where discovery is not possible, you can configure static hosts in the YAML file:

```yaml
static:
  hosts:
    - id: server-001
      customer_id: customer-1
      control_endpoint:
        endpoint: https://192.168.1.100
        type: redfish
        username: admin
        password: password
      sol_endpoint:
        type: redfish_serial
        endpoint: https://192.168.1.100/redfish/v1/Systems/1/SerialInterfaces/1
      vnc_endpoint:
        type: novnc_proxy
        endpoint: vnc://192.168.1.100:5900
```

## IPMI Configuration

Configure IPMI settings for BMC operations:

```bash
# Interface and authentication
IPMI_INTERFACE=lanplus
IPMI_CIPHER_SUITE=3
IPMI_PRIVILEGE_LEVEL=ADMINISTRATOR

# Timeouts
IPMI_SESSION_TIMEOUT=20s

# SOL (Serial-over-LAN) settings
IPMI_SOL_BAUD_RATE=115200
IPMI_SOL_AUTHENTICATION=true
IPMI_SOL_ENCRYPTION=true
```

## Redfish Configuration

Configure Redfish settings for BMC operations:

```bash
# HTTP client settings
REDFISH_HTTP_TIMEOUT=30s
REDFISH_INSECURE_SKIP_VERIFY=false

# Authentication
REDFISH_AUTH_METHOD=basic
REDFISH_SESSION_COOKIE=true
REDFISH_SESSION_TIMEOUT=30m
```

## VNC/KVM Configuration

Configure VNC console access:

```bash
# Enable VNC support
VNC_ENABLED=true

# Connection limits
VNC_PORT=5900
VNC_BIND_ADDRESS=127.0.0.1
VNC_MAX_CONNECTIONS=5

# Streaming quality
VNC_FRAME_RATE=15
VNC_QUALITY=6

# Security
VNC_ENABLE_AUTHENTICATION=true
```

## Serial Console Configuration

Configure serial console (SOL) access:

```bash
# Enable serial console support
SERIAL_CONSOLE_ENABLED=true

# Session limits
SERIAL_CONSOLE_MAX_SESSIONS=10
SERIAL_CONSOLE_SESSION_TIMEOUT=2h

# Serial port settings
SERIAL_CONSOLE_DEFAULT_BAUD_RATE=115200
SERIAL_CONSOLE_BUFFER_SIZE=8192
```

## Security Best Practices

1. **Never commit sensitive values to version control**
   - Add `agent.env` to `.gitignore`
   - Only commit `.example` files

2. **Use strong, randomly generated secrets**
   - Minimum 32 characters for encryption key
   - Use `openssl rand -hex 32`

3. **Limit network ranges for BMC discovery**
   - Only scan trusted networks
   - Avoid public IP ranges
   - Use specific CIDR notation

4. **Set appropriate file permissions**
   ```bash
   chmod 600 agent.env  # Only owner can read/write
   chmod 644 agent.yaml # Owner can read/write, others can read
   ```

5. **Enable TLS verification in production**
   ```bash
   SECURITY_ENABLE_TLS_VERIFICATION=true
   REDFISH_INSECURE_SKIP_VERIFY=false
   ```

6. **Configure network access control**
   ```bash
   # Only allow RFC1918 private networks
   SECURITY_ALLOWED_NETWORKS=192.168.0.0/16,10.0.0.0/8,172.16.0.0/12
   SECURITY_DENY_PRIVATE_NETWORKS=false
   ```

7. **Enable audit logging**
   ```bash
   SECURITY_ENABLE_AUDIT_LOGGING=true
   SECURITY_AUDIT_LOG_PATH=/var/log/bmc-agent/audit.log
   ```

## Environment Variable Reference

### Required
| Variable | Description | Example |
|----------|-------------|---------|
| `AGENT_ID` | Unique agent identifier | `agent-001` |
| `AGENT_NAME` | Human-readable agent name | `DC Local Agent 01` |
| `AGENT_DATACENTER_ID` | Datacenter identifier | `dc-local-01` |
| `AGENT_GATEWAY_ENDPOINT` | Gateway service URL | `http://localhost:8081` |
| `AGENT_ENCRYPTION_KEY` | Encryption key (32+ chars) | `openssl rand -hex 32` |

### Service Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_REGION` | `default` | Region identifier |
| `ENVIRONMENT` | `development` | Environment name |
| `LOG_LEVEL` | `info` | Logging level |

### BMC Discovery
| Variable | Default | Description |
|----------|---------|-------------|
| `BMC_DISCOVERY_ENABLED` | `true` | Enable BMC discovery |
| `BMC_DISCOVERY_SCAN_INTERVAL` | `5m` | Scan interval |
| `BMC_DISCOVERY_NETWORK_RANGES` | - | CIDR ranges to scan |
| `BMC_DISCOVERY_SCAN_TIMEOUT` | `10s` | Scan timeout |
| `BMC_DISCOVERY_MAX_CONCURRENT` | `50` | Max concurrent scans |

### BMC Operations
| Variable | Default | Description |
|----------|---------|-------------|
| `BMC_OPERATION_TIMEOUT` | `30s` | Operation timeout |
| `BMC_POWER_OPERATION_TIMEOUT` | `60s` | Power operation timeout |
| `BMC_CONSOLE_TIMEOUT` | `300s` | Console timeout |
| `BMC_MAX_RETRIES` | `3` | Max retry attempts |
| `BMC_MAX_CONCURRENT_OPERATIONS` | `10` | Max concurrent operations |

### IPMI Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `IPMI_INTERFACE` | `lanplus` | IPMI interface |
| `IPMI_CIPHER_SUITE` | `3` | Cipher suite |
| `IPMI_PRIVILEGE_LEVEL` | `ADMINISTRATOR` | Privilege level |
| `IPMI_SESSION_TIMEOUT` | `20s` | Session timeout |
| `IPMI_SOL_BAUD_RATE` | `115200` | SOL baud rate |
| `IPMI_SOL_AUTHENTICATION` | `true` | Enable SOL auth |
| `IPMI_SOL_ENCRYPTION` | `true` | Enable SOL encryption |

### Redfish Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `REDFISH_HTTP_TIMEOUT` | `30s` | HTTP timeout |
| `REDFISH_INSECURE_SKIP_VERIFY` | `false` | Skip TLS verification |
| `REDFISH_AUTH_METHOD` | `basic` | Auth method |
| `REDFISH_SESSION_COOKIE` | `true` | Use session cookies |
| `REDFISH_SESSION_TIMEOUT` | `30m` | Session timeout |

### VNC Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `VNC_ENABLED` | `true` | Enable VNC support |
| `VNC_PORT` | `5900` | VNC port |
| `VNC_BIND_ADDRESS` | `127.0.0.1` | Bind address |
| `VNC_MAX_CONNECTIONS` | `5` | Max connections |
| `VNC_FRAME_RATE` | `15` | Frame rate (FPS) |
| `VNC_QUALITY` | `6` | Quality (0-9) |
| `VNC_ENABLE_AUTHENTICATION` | `true` | Enable authentication |

### Serial Console Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| `SERIAL_CONSOLE_ENABLED` | `true` | Enable serial console |
| `SERIAL_CONSOLE_DEFAULT_BAUD_RATE` | `115200` | Default baud rate |
| `SERIAL_CONSOLE_BUFFER_SIZE` | `8192` | Buffer size (bytes) |
| `SERIAL_CONSOLE_SESSION_TIMEOUT` | `2h` | Session timeout |
| `SERIAL_CONSOLE_MAX_SESSIONS` | `10` | Max concurrent sessions |

### Connection Management
| Variable | Default | Description |
|----------|---------|-------------|
| `CONNECTION_CONNECT_TIMEOUT` | `10s` | Connect timeout |
| `CONNECTION_RECONNECT_INTERVAL` | `30s` | Reconnect interval |
| `CONNECTION_HEARTBEAT_INTERVAL` | `30s` | Heartbeat interval |
| `CONNECTION_HEARTBEAT_TIMEOUT` | `90s` | Heartbeat timeout |
| `CONNECTION_REGISTRATION_INTERVAL` | `60s` | Registration interval |

### Health Monitoring
| Variable | Default | Description |
|----------|---------|-------------|
| `HEALTH_MONITORING_ENABLED` | `true` | Enable health monitoring |
| `HEALTH_MONITORING_CHECK_INTERVAL` | `60s` | Check interval |
| `HEALTH_MONITORING_CPU_THRESHOLD` | `80.0` | CPU threshold (%) |
| `HEALTH_MONITORING_MEMORY_THRESHOLD` | `85.0` | Memory threshold (%) |
| `HEALTH_MONITORING_DISK_THRESHOLD` | `90.0` | Disk threshold (%) |

### Security
| Variable | Default | Description |
|----------|---------|-------------|
| `SECURITY_ENABLE_TLS_VERIFICATION` | `true` | Enable TLS verification |
| `SECURITY_ALLOWED_NETWORKS` | - | Allowed CIDR ranges |
| `SECURITY_DENY_PRIVATE_NETWORKS` | `false` | Deny private networks |
| `SECURITY_ENABLE_AUDIT_LOGGING` | `true` | Enable audit logging |
| `SECURITY_AUDIT_LOG_PATH` | `/var/log/bmc-agent/audit.log` | Audit log path |

### Metrics
| Variable | Default | Description |
|----------|---------|-------------|
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `METRICS_PORT` | `9092` | Metrics server port |

See `agent.env.example` for complete list of available variables.
