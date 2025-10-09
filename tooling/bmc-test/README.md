# BMC Test Utilities

Testing and diagnostic utilities for BMC (Baseboard Management Controller)
operations.

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
