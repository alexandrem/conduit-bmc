# Manager Configuration

This directory contains configuration files for the BMC Manager service.

## Configuration Files

### manager.yaml.example
Operational configuration for the Manager service. Copy to `manager.yaml` and customize as needed.

**Key Configuration Areas:**
- `log`: Logging level, format, and output configuration
- `http`: HTTP server timeouts and settings
- `manager.gateway_discovery`: Gateway health check and discovery settings
- `manager.server_management`: Server registration and heartbeat configuration
- `manager.customer_management`: Customer registration and API key settings
- `manager.rate_limit`: Rate limiting for API endpoints
- `manager.session_management`: Session TTLs and cleanup intervals
- `database`: Database connection pooling and migration settings
- `auth`: JWT token TTLs and session configuration
- `tls`: TLS/SSL configuration (optional)
- `metrics`: Prometheus metrics configuration

### manager.env.example
Environment variables for the Manager service. Copy to `manager.env` and set appropriate values.

**Required Variables:**
- `JWT_SECRET_KEY` - JWT signing secret (minimum 32 characters, use strong random value)
- `ENCRYPTION_KEY` - Data encryption key (minimum 32 characters)
- `DATABASE_URL` - Database connection string (e.g., `file:./manager.db` for SQLite)

**Common Variables:**
- `MANAGER_HOST` - Bind address (default: `0.0.0.0`)
- `MANAGER_PORT` - Listen port (default: `8080`)
- `ENVIRONMENT` - Environment name (`development`, `staging`, `production`)
- `LOG_LEVEL` - Logging level (`debug`, `info`, `warn`, `error`)
- `ADMIN_EMAILS` - Comma-separated list of admin user emails (e.g., `admin@example.com,ops@example.com`)

**Security Variables:**
- `TLS_ENABLED` - Enable TLS (default: `false`)
- `TLS_CERT_FILE` - Path to TLS certificate
- `TLS_KEY_FILE` - Path to TLS private key

**Feature Flags:**
- `CUSTOMER_ALLOW_SELF_REGISTRATION` - Allow customer self-registration (default: `false`)
- `CUSTOMER_EMAIL_VERIFICATION_REQUIRED` - Require email verification (default: `true`)
- `FEATURE_DEBUG_ENDPOINTS` - Enable debug endpoints (default: `false`)

## Setup Instructions

### Development Setup

1. **Copy example files:**
   ```bash
   cd manager/config
   cp manager.yaml.example manager.yaml
   cp manager.env.example manager.env
   ```

2. **Generate secure secrets:**
   ```bash
   # Generate JWT secret
   openssl rand -hex 32

   # Generate encryption key
   openssl rand -hex 32
   ```

3. **Edit manager.env:**
   ```bash
   # Required - Use generated secrets from step 2
   JWT_SECRET_KEY=<generated-jwt-secret>
   ENCRYPTION_KEY=<generated-encryption-key>
   DATABASE_URL=file:./manager.db

   # Development settings
   MANAGER_HOST=0.0.0.0
   MANAGER_PORT=8080
   ENVIRONMENT=development
   LOG_LEVEL=debug
   ```

4. **Start the service:**
   ```bash
   cd ../..
   make -C manager run
   ```

### Production Deployment

1. **Set environment variables via your deployment system:**
   ```bash
   export JWT_SECRET_KEY="$(openssl rand -hex 32)"
   export ENCRYPTION_KEY="$(openssl rand -hex 32)"
   export DATABASE_URL="postgres://user:pass@localhost/bmcdb"
   export ENVIRONMENT=production
   export LOG_LEVEL=info
   ```

2. **Use YAML file for operational configuration:**
   ```yaml
   # manager.yaml
   manager:
     rate_limit:
       requests_per_minute: 1000
       burst_size: 100
   log:
     level: info
     format: json
   ```

3. **Enable TLS in production:**
   ```bash
   export TLS_ENABLED=true
   export TLS_CERT_FILE=/etc/ssl/certs/manager.crt
   export TLS_KEY_FILE=/etc/ssl/private/manager.key
   ```

## Configuration Precedence

Configuration is loaded in order (later overrides earlier):
1. Default values (from struct tags in code)
2. YAML configuration file (`manager.yaml`)
3. Environment file (`manager.env`)
4. Environment variables
5. Service-specific environment variables (prefixed with `MANAGER_`)

Example:
```bash
# Global setting
export LOG_LEVEL=info

# Manager-specific override (takes precedence)
export MANAGER_LOG_LEVEL=debug
```

## Security Best Practices

1. **Never commit sensitive values to version control**
   - Add `manager.env` to `.gitignore`
   - Only commit `.example` files

2. **Use strong, randomly generated secrets**
   - Minimum 32 characters for all secrets
   - Use `openssl rand -hex 32` or similar

3. **Rotate secrets regularly**
   - Rotate JWT secrets every 90 days in production
   - Update encryption keys with proper migration

4. **Set appropriate file permissions**
   ```bash
   chmod 600 manager.env  # Only owner can read/write
   chmod 644 manager.yaml # Owner can read/write, others can read
   ```

5. **Enable TLS in production**
   - Always use TLS/SSL for production deployments
   - Use valid certificates from trusted CA

## Environment Variable Reference

### Required
| Variable         | Description                | Example                |
|------------------|----------------------------|------------------------|
| `JWT_SECRET_KEY` | JWT signing secret         | `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | Data encryption key        | `openssl rand -hex 32` |
| `DATABASE_URL`   | Database connection string | `file:./manager.db`    |

### Service Configuration
| Variable       | Default       | Description      |
|----------------|---------------|------------------|
| `MANAGER_HOST` | `0.0.0.0`     | Bind address     |
| `MANAGER_PORT` | `8080`        | Listen port      |
| `ENVIRONMENT`  | `development` | Environment name |
| `LOG_LEVEL`    | `info`        | Logging level    |

### Rate Limiting
| Variable                         | Default | Description          |
|----------------------------------|---------|----------------------|
| `RATE_LIMIT_ENABLED`             | `true`  | Enable rate limiting |
| `RATE_LIMIT_REQUESTS_PER_MINUTE` | `100`   | Global rate limit    |

### Customer Management
| Variable                               | Default | Description                |
|----------------------------------------|---------|----------------------------|
| `CUSTOMER_ALLOW_SELF_REGISTRATION`     | `false` | Allow self-registration    |
| `CUSTOMER_EMAIL_VERIFICATION_REQUIRED` | `true`  | Require email verification |

### Session Management
| Variable              | Default | Description         |
|-----------------------|---------|---------------------|
| `SESSION_PROXY_TTL`   | `1h`    | Proxy session TTL   |
| `SESSION_VNC_TTL`     | `4h`    | VNC session TTL     |
| `SESSION_CONSOLE_TTL` | `2h`    | Console session TTL |

### Metrics
| Variable          | Default | Description               |
|-------------------|---------|---------------------------|
| `METRICS_ENABLED` | `true`  | Enable Prometheus metrics |
| `METRICS_PORT`    | `9090`  | Metrics server port       |

See `manager.env.example` for complete list of available variables.
