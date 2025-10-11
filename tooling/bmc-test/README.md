# BMC Test Utilities

Testing and diagnostic utilities for BMC (Baseboard Management Controller)
operations.

## Table of Contents

- [vnc-connect](#vnc-connect) - VNC connection testing utility
- [sol-connect](#sol-connect) - Serial-over-LAN (SOL) connection testing utility

## vnc-connect

Swiss Army Knife for testing VNC connections to BMC systems such as Dell iDRAC,
HP iLO, and Supermicro IPMI.

### Features

- **Standard VNC Protocol Support** - RFB 3.3, 3.7, 3.8
- **TLS/SSL Encrypted Connections** - Secure BMC access with
  VeNCrypt/RFB-over-TLS
- **VNC Authentication** - Password-based DES challenge-response
- **Connection Diagnostics** - Detailed logging at multiple verbosity levels
- **Comprehensive Testing** - Validates full VNC flow from handshake to
  framebuffer updates
- **Reuses Local-Agent Logic** - Same battle-tested VNC implementation as the
  production system

### Building

```bash
# Build the binary
go build -o bin/vnc-connect ./cmd/vnc-connect

# Run directly with go run
go run cmd/vnc-connect/main.go [flags]
```

### Usage

```bash
vnc-connect --host <ip> --port <port> [flags]

Flags:
  --host string          VNC server hostname or IP address (required)
  --port int             VNC server port (default 5901)
  --password string      VNC password (omit for no authentication)
  --tls                  Enable TLS encryption
  --tls-insecure         Skip TLS certificate verification (default true)
  --timeout duration     Connection timeout (default 30s)
  -v, --verbose          Enable verbose logging (info level)
  --debug                Enable debug logging (most detailed)
  -h, --help             Help for vnc-connect
```

### Examples

#### Test Dell iDRAC VNC with TLS

```bash
vnc-connect --host 10.147.8.25 --port 5901 --password secret --tls --debug
```

#### Test Standard VNC Without TLS

```bash
vnc-connect --host 192.168.1.100 --port 5900 --password secret --verbose
```

#### Test VNC Without Password

```bash
vnc-connect --host 192.168.1.100 --port 5900
```

#### Test with Custom Timeout

```bash
vnc-connect --host 10.0.0.100 --port 5901 --timeout 60s --verbose
```

### Output Example

```console
$ vnc-connect --host 10.147.8.25 --port 5901 --password <secret> --verbose

4:07PM INF VNC Test Client starting has_password=true host=10.147.8.25 port=5901 timeout=30s tls=false
4:07PM INF Creating VNC transport...
4:07PM INF Connecting to VNC server...
4:07PM INF VNC security type negotiated rfb_version="RFB 3.8" security_type="VNC Authentication" transport=native-tcp
4:07PM INF VNC authentication completed successfully security_type="VNC Authentication" transport=native-tcp
4:07PM INF ✅ VNC connection successful!
4:07PM INF ✅ RFB handshake completed
4:07PM INF ✅ Authentication successful
4:07PM INF Requesting framebuffer update...
4:07PM INF ✅ FramebufferUpdateRequest sent
4:07PM INF Testing data transfer (waiting for FramebufferUpdate)...
4:07PM INF ✅ Data transfer working - received FramebufferUpdate bytes_received=4 data_hex=00000001

============================================================
VNC Test Results
============================================================
Host:           10.147.8.25:5901
TLS:            false
Authentication: VNC Authentication (password provided)
Status:         ✅ SUCCESS
============================================================

4:07PM INF VNC test completed successfully
```

### What Gets Tested

The utility performs a complete VNC connection test:

1. **TCP/TLS Connection** - Establishes connection to BMC VNC server
2. **RFB Handshake** - Negotiates protocol version (RFB 3.3/3.7/3.8)
3. **Security Negotiation** - Selects authentication method
4. **VNC Authentication** - Performs DES challenge-response if password provided
5. **ClientInit/ServerInit** - Exchanges client/server initialization messages
6. **FramebufferUpdate** - Requests and receives screen data from BMC

### Troubleshooting

#### Connection Timeout

```
VNC read timeout: read tcp ... i/o timeout
```

- Verify host/port are correct
- Check firewall rules allow VNC traffic
- Try increasing timeout: `--timeout 60s`

#### Authentication Failed

```
VNC authentication failed
```

- Verify password is correct
- Check BMC user has KVM/console permissions
- Some BMCs require specific user roles

#### TLS Certificate Errors

```
TLS handshake failed
```

- Use `--tls-insecure` to skip certificate verification (self-signed certs)
- For production, provide proper CA certificates

### Technical Details

- **Protocol**: RFC 6143 (RFB - Remote Framebuffer Protocol)
- **Transport**: Native TCP or TLS-wrapped TCP
- **Authentication**: VNC Authentication (Type 2, DES challenge-response)
- **Dependencies**: Uses `local-agent/pkg/vnc` for VNC protocol implementation

### Related Documentation

- VNC Protocol Flow: `docs/technical/vnc-protocol-flow.md`
- RFB Specification: RFC 6143
- Architecture: `docs/.ai/ARCHITECTURE-AI.md`

## sol-connect

Swiss Army Knife for testing Serial-over-LAN (SOL) console connections to BMC
systems such as Dell iDRAC, HP iLO, and Supermicro IPMI.

### Features

- **IPMI SOL Support** - Using FreeIPMI's `ipmiconsole` with PTY allocation
- **Redfish Serial Console** - WebSocket-based serial console
- **Connection Diagnostics** - Detects common failure patterns (connection
  closes, permission issues)
- **Bidirectional Testing** - Validates both read and write operations
- **PTY-Aware** - Properly handles pseudo-terminal requirements for IPMI
- **Reuses Local-Agent Logic** - Same battle-tested SOL implementation as the
  production system

### Building

```bash
# Build the binary
go build -o bin/sol-connect ./cmd/sol-connect

# Run directly with go run
go run cmd/sol-connect/main.go [flags]
```

### Usage

```bash
sol-connect --host <ip> --username <user> --password <pass> [flags]

Flags:
  --host string          BMC hostname or IP address (required)
  --port int             IPMI port (default 623)
  --username string      BMC username (required)
  --password string      BMC password (required)
  --type string          SOL type: 'ipmi' or 'redfish' (default "ipmi")
  --timeout duration     Connection timeout (default 30s)
  -v, --verbose          Enable verbose logging (info level)
  --debug                Enable debug logging (most detailed)
  -h, --help             Help for sol-connect
```

### Examples

#### Test Dell iDRAC IPMI SOL

```bash
sol-connect --host 10.147.8.25 --port 623 \
  --username admin --password secret \
  --type ipmi --debug
```

#### Test Redfish Serial Console

```bash
sol-connect --host 10.147.8.25 \
  --username admin --password secret \
  --type redfish --verbose
```

#### Test with Custom Timeout

```bash
sol-connect --host 192.168.1.100 \
  --username admin --password secret \
  --timeout 60s --verbose
```

### Output Example

#### Successful Connection

```console
$ sol-connect --host 10.147.8.25 --username k810249 --password <secret> --type ipmi --verbose

8:55PM INF SOL Test Client starting host=10.147.8.25 port=623 timeout=30000 type=ipmi username=k810249
8:55PM INF Creating IPMI SOL client... endpoint=10.147.8.25:623
8:55PM INF Connecting to BMC...
8:55PM INF ✅ SOL connection successful!
8:55PM INF ✅ Authentication successful
8:55PM INF Testing data transfer (waiting for console output)...
8:55PM INF ✅ Data transfer working - received console data bytes_received=18 data_preview="\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n"
8:55PM INF Testing console input (sending newline)...
8:55PM INF ✅ Console input working
8:55PM INF Received console response bytes_received=255 response="PXE-E51: No DHCP or proxyDHCP offers were received..."

============================================================
SOL Test Results
============================================================
Host:           10.147.8.25:623
Type:           ipmi
Username:       k810249
Status:         ✅ SUCCESS
============================================================

Connection is working! You can now use this endpoint for console access.
8:55PM INF SOL test completed successfully
```

#### Connection Closed by BMC

```console
$ sol-connect --host 10.147.8.25 --username admin --password wrong --type ipmi

8:51PM INF SOL Test Client starting...
8:51PM INF Connecting to BMC...
8:51PM INF ✅ SOL connection successful!
8:51PM ERR ❌ BMC is closing the connection! bytes_received=28 message="[closing the connection]"

============================================================
SOL Connection Error
============================================================
Status:         ❌ FAILED - Connection Closed by BMC
Message:        [closing the connection]
------------------------------------------------------------
Possible causes:
  1. SOL is disabled in BMC settings
  2. User lacks Serial Console privileges
  3. Another session is active (check BMC web interface)
  4. BMC requires specific configuration
  5. PTY is required but not available (IPMI only)
============================================================
```

### What Gets Tested

The utility performs a complete SOL connection test:

1. **Authentication** - Validates credentials with BMC
2. **Session Creation** - Establishes SOL/serial console session
3. **Data Read** - Tests receiving console output from server
4. **Data Write** - Tests sending input to console
5. **Bidirectional Flow** - Verifies round-trip communication
6. **Connection Close Detection** - Identifies when BMC rejects the session

### Troubleshooting

#### Connection Closed by BMC

```
❌ BMC is closing the connection!
Message: [closing the connection]
```

**Possible causes:**

1. **SOL Disabled** - Enable Serial Console in BMC settings
2. **Missing Permissions** - User needs "Serial Console" or "Console Redirection"
   privilege
3. **Active Session** - Another SOL session may be active (BMCs often limit to 1
   session)
4. **Wrong Credentials** - Verify username/password are correct
5. **No PTY** (IPMI only) - The system running `sol-connect` must support PTY
   allocation

#### Connection Timeout

```
SOL connection failed: context deadline exceeded
```

- Verify host/port are correct
- Check firewall rules allow IPMI traffic (port 623/UDP for IPMI)
- Try increasing timeout: `--timeout 60s`
- For Redfish, ensure HTTPS (port 443) is accessible

#### Authentication Failed

```
Failed to create SOL session: authentication failed
```

- Verify username/password are correct
- Check BMC user has SOL/console permissions
- Try logging into BMC web interface with same credentials
- Some BMCs require specific privilege levels (Administrator vs Operator)

#### IPMI-Specific: ipmiconsole Not Found

```
ipmiconsole not found: install freeipmi-tools package
```

**Solution:**

```bash
# Ubuntu/Debian
sudo apt-get install freeipmi-tools

# macOS
brew install freeipmi

# RHEL/CentOS
sudo yum install freeipmi
```

### Technical Details

- **IPMI Protocol**: Uses FreeIPMI's `ipmiconsole` with PTY (pseudo-terminal)
  allocation
- **Redfish Protocol**: WebSocket-based serial console over HTTPS
- **PTY Requirement** (IPMI): `ipmiconsole` requires a PTY to function properly,
  otherwise it exits with `tcgetattr: Inappropriate ioctl for device`
- **Dependencies**: Uses `local-agent/pkg/sol` for SOL protocol implementation
- **Port Defaults**: IPMI uses port 623/UDP, Redfish uses 443/TCP (HTTPS)

### Common BMC-Specific Notes

#### Dell iDRAC

- iDRAC 9+ may require IPMI fallback even when Redfish is available
- Ensure "Serial Console" is enabled in iDRAC settings
- User must have "Console" privilege
- Default IPMI port: 623

#### HP iLO

- iLO 4+ supports both IPMI and Redfish serial console
- User needs "Remote Console" privilege
- May require "Virtual Serial Port" enabled in iLO settings

#### Supermicro IPMI

- Standard IPMI SOL support
- May require "SOL Enable" in IPMI configuration
- Check "Serial Port" settings match (baud rate, flow control)
