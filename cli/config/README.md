# CLI Configuration

This directory contains configuration files for the BMC CLI tool.

## Configuration Files

### config.yaml.example
YAML configuration for the CLI tool. The CLI automatically creates and manages `~/.bmc-cli/config.yaml` after login.

**Configuration Structure:**
- `manager.endpoint` - Manager service URL
- `gateway.url` - Legacy gateway URL (for backward compatibility)
- `auth.access_token` - JWT access token (managed by login command)
- `auth.refresh_token` - JWT refresh token (managed by login command)
- `auth.token_expires_at` - Token expiration time (managed by login command)
- `auth.email` - Logged in user email (managed by login command)
- `auth.api_key` - Legacy API key (deprecated)
- `auth.token` - Legacy JWT token (deprecated)

### cli.env.example
Environment variables for the CLI tool. Copy to `.env` or `cli.env` and set appropriate values.

**Environment Variables:**
- `BMC_MANAGER_ENDPOINT` - Manager service URL (maps to `manager.endpoint`)
- `BMC_GATEWAY_URL` - Gateway URL (maps to `gateway.url`)
- `BMC_AUTH_ACCESS_TOKEN` - JWT access token (maps to `auth.access_token`)
- `BMC_AUTH_REFRESH_TOKEN` - JWT refresh token (maps to `auth.refresh_token`)
- `BMC_AUTH_API_KEY` - API key (maps to `auth.api_key`)
- `BMC_AUTH_EMAIL` - User email (maps to `auth.email`)

**Note:** All environment variables must be prefixed with `BMC_`. Nested config keys use underscores instead of dots.

## Setup Instructions

### Standard Setup (Recommended)

1. **Set manager endpoint (optional - defaults to localhost:8080):**
   ```bash
   export BMC_MANAGER_ENDPOINT=http://localhost:8080
   ```

2. **Login to authenticate:**
   ```bash
   bmc-cli auth login
   # or
   bmc-cli auth login user@example.com
   ```

3. **Use the CLI:**
   ```bash
   bmc-cli server list
   bmc-cli server show server-001
   bmc-cli server power on server-001
   ```

The CLI automatically:
- Creates `~/.bmc-cli/config.yaml` on first login
- Saves authentication tokens after login
- Refreshes tokens automatically when needed

### Automation Setup (API Key)

For scripts and automation, use an API key instead of interactive login:

1. **Set environment variables:**
   ```bash
   export BMC_MANAGER_ENDPOINT=http://localhost:8080
   export BMC_AUTH_API_KEY=your-api-key-here
   ```

2. **Use the CLI:**
   ```bash
   bmc-cli server list --output json
   ```

### Development Setup

1. **Copy example file:**
   ```bash
   cd cli/config
   cp cli.env.example .env
   ```

2. **Edit .env:**
   ```bash
   BMC_MANAGER_ENDPOINT=http://localhost:8080
   ```

3. **Use the CLI:**
   ```bash
   source .env
   ./bin/bmc-cli server list
   ```

## Configuration File Locations

The CLI searches for configuration in these locations (in order of precedence):

1. **Command-line flags** (highest priority)
   ```bash
   bmc-cli --gateway-url http://example.com server list
   ```

2. **Environment variables**
   ```bash
   export BMC_MANAGER_ENDPOINT=http://example.com
   ```

3. **Config file specified via --config flag**
   ```bash
   bmc-cli --config /path/to/config.yaml server list
   ```

4. **./config.yaml** (current directory)

5. **~/.bmc-cli/config.yaml** (user home directory - created automatically)

6. **/etc/bmc-cli/config.yaml** (system-wide)

7. **Default values** (lowest priority)
   - Manager endpoint: `http://localhost:8080`
   - Gateway URL: `http://localhost:8081`

## Authentication Methods

### 1. Interactive Login (Recommended)

```bash
# Login with email/password prompt
bmc-cli auth login

# Login with email specified
bmc-cli auth login user@example.com

# Check login status
bmc-cli auth status

# Logout (clears tokens)
bmc-cli auth logout
```

**How it works:**
- Prompts for email and password
- Obtains access token and refresh token from manager
- Saves tokens to `~/.bmc-cli/config.yaml`
- Automatically refreshes tokens when they expire
- Tokens are encrypted at rest

### 2. API Key (for automation)

```bash
# Set via environment variable
export BMC_AUTH_API_KEY=your-api-key-here
bmc-cli server list

# Set via .env file
echo "BMC_AUTH_API_KEY=your-api-key-here" >> .env
source .env
bmc-cli server list

# Set via command-line flag
bmc-cli --api-key your-api-key-here server list
```

### 3. JWT Token (for advanced use)

```bash
# Set via environment variable
export BMC_AUTH_ACCESS_TOKEN=your-jwt-token
bmc-cli server list

# Set via command-line flag
bmc-cli --token your-jwt-token server list
```

## Configuration Examples

### Example 1: Local Development

```yaml
# ~/.bmc-cli/config.yaml
bmc_manager:
  endpoint: http://localhost:8080

auth:
  access_token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  refresh_token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  token_expires_at: 2024-01-01T00:00:00Z
  email: dev@example.com
```

### Example 2: Production with Environment Variables

```bash
# ~/.bashrc or ~/.zshrc
export BMC_MANAGER_ENDPOINT=https://manager.example.com

# For automation/scripts
export BMC_AUTH_API_KEY=prod-api-key-here
```

### Example 3: Multiple Environments

```bash
# dev.env
BMC_MANAGER_ENDPOINT=http://localhost:8080

# staging.env
BMC_MANAGER_ENDPOINT=https://staging-manager.example.com

# production.env
BMC_MANAGER_ENDPOINT=https://manager.example.com

# Use different environments
source dev.env && bmc-cli server list
source staging.env && bmc-cli server list
source production.env && bmc-cli server list
```

## Command-Line Flags

All configuration options can be overridden via command-line flags:

```bash
# Override manager endpoint
bmc-cli --config /path/to/config.yaml server list

# Override gateway URL
bmc-cli --gateway-url http://production.example.com server list

# Set API key
bmc-cli --api-key your-api-key server list

# Set JWT token
bmc-cli --token your-jwt-token server list
```

## Environment Variable Reference

| Variable | Config Key | Description | Required |
|----------|------------|-------------|----------|
| `BMC_MANAGER_ENDPOINT` | `manager.endpoint` | Manager service URL | No (defaults to localhost:8080) |
| `BMC_GATEWAY_URL` | `gateway.url` | Gateway URL (legacy) | No (defaults to localhost:8081) |
| `BMC_AUTH_ACCESS_TOKEN` | `auth.access_token` | JWT access token | No (login creates it) |
| `BMC_AUTH_REFRESH_TOKEN` | `auth.refresh_token` | JWT refresh token | No (login creates it) |
| `BMC_AUTH_API_KEY` | `auth.api_key` | API key for authentication | No |
| `BMC_AUTH_EMAIL` | `auth.email` | User email | No |

**Note:** The `BMC_` prefix is required for all environment variables. Nested config keys use underscores:
- `manager.endpoint` → `BMC_MANAGER_ENDPOINT`
- `auth.access_token` → `BMC_AUTH_ACCESS_TOKEN`

## Common Usage Examples

### List servers

```bash
# Using saved login credentials
bmc-cli server list

# Using API key
bmc-cli --api-key your-api-key server list

# With JSON output for scripting
bmc-cli server list --output json
```

### Power management

```bash
bmc-cli server power on server-001
bmc-cli server power off server-001
bmc-cli server power status server-001
```

### Console access

```bash
# Open web console (default)
bmc-cli server console server-001

# Open VNC console
bmc-cli server vnc server-001

# Direct terminal streaming (advanced)
bmc-cli server console server-001 --terminal
```

### Automation script example

```bash
#!/bin/bash
# Check offline servers

export BMC_MANAGER_ENDPOINT=http://localhost:8080
export BMC_AUTH_API_KEY=your-api-key

# Get server list as JSON
servers=$(bmc-cli server list --output json)

# Process with jq
echo "$servers" | jq '.[] | select(.status=="offline") | .id'
```

## Security Best Practices

1. **Never commit credentials to version control**
   - Add `.env`, `config.yaml`, and `~/.bmc-cli/config.yaml` to `.gitignore`
   - Only commit `.example` files

2. **Use API keys for automation**
   - Generate dedicated API keys for scripts
   - Rotate API keys regularly
   - Use different API keys for different environments

3. **Protect configuration files**
   ```bash
   chmod 600 ~/.bmc-cli/config.yaml
   chmod 600 .env
   ```

4. **Use environment-specific endpoints**
   ```bash
   # Development
   export BMC_MANAGER_ENDPOINT=http://localhost:8080

   # Staging
   export BMC_MANAGER_ENDPOINT=https://staging-manager.example.com

   # Production
   export BMC_MANAGER_ENDPOINT=https://manager.example.com
   ```

5. **Clear tokens when done**
   ```bash
   # Logout to clear stored credentials
   bmc-cli auth logout
   ```

## Troubleshooting

### Connection errors

```bash
# Test manager endpoint
curl http://localhost:8080/health

# Verify configuration
cat ~/.bmc-cli/config.yaml

# Check environment variables
env | grep BMC_
```

### Authentication errors

```bash
# Re-login
bmc-cli auth logout
bmc-cli auth login

# Or use API key
export BMC_AUTH_API_KEY=your-api-key
bmc-cli server list
```

### Token expired

```bash
# Tokens are automatically refreshed, but if issues persist:
bmc-cli auth logout
bmc-cli auth login
```

### Config file not found

```bash
# Check if config exists
ls -la ~/.bmc-cli/config.yaml

# Login to create it
bmc-cli auth login

# Or manually create it
mkdir -p ~/.bmc-cli
cp config.yaml.example ~/.bmc-cli/config.yaml
# Edit the file with your settings
```

### Environment variable not working

```bash
# Make sure to use the BMC_ prefix
export BMC_MANAGER_ENDPOINT=http://localhost:8080  # Correct
export MANAGER_ENDPOINT=http://localhost:8080      # Wrong - missing BMC_ prefix

# Check if set correctly
echo $BMC_MANAGER_ENDPOINT
```
