# Troubleshooting

* [Troubleshooting](#troubleshooting)
  * [VNC](#vnc)
    * [Common Issues](#common-issues)
    * [Network Tracing](#network-tracing)
  * [SOL](#sol)
    * [Common Issues](#common-issues-1)
    * [Network Tracing](#network-tracing-1)

## VNC

### Common Issues

#### 1. Browser hangs waiting for ServerInit

**Symptoms**: Browser shows "Connecting..." indefinitely, logs show authentication succeeded.

**Cause**: Agent sent ClientInit during auth but didn't cache ServerInit, or ServerInit wasn't replayed to browser.

**Fix**: Verify `GetServerInit()` returns non-empty data, check logs for "Replaying cached ServerInit".

#### 2. BMC closes connection after authentication

**Symptoms**: Logs show "connection reset by peer" immediately after authentication.

**Cause**: Agent didn't send ClientInit after authentication. Many VNC servers timeout if they don't receive ClientInit within a few seconds.

**Fix**: Ensure `SendClientInit()` is called in `Authenticate()` method.

#### 3. Browser shows "Security negotiation failed"

**Symptoms**: noVNC displays security error, connection closes immediately.

**Cause**: RFB proxy isn't properly handling browser's security type selection.

**Fix**: Verify `HandleBrowserHandshake()` sends correct security types list and reads browser's selection.

#### 4. Black screen after connection

**Symptoms**: Browser connects, handshake completes, but screen stays black.

**Cause**: Transparent proxying isn't forwarding FramebufferUpdate messages correctly.

**Fix**: Check `StreamToTCPProxy` is running and forwarding data bidirectionally, verify no errors in proxy goroutines.

### Network Tracing

For deep protocol debugging, capture traffic with `tcpdump`:

```bash
# Capture BMC VNC traffic
tcpdump -i any -w vnc-bmc.pcap port 5900

# Analyze in Wireshark with VNC dissector
wireshark vnc-bmc.pcap
```


## SOL

### Common Issues

#### 1. ipmiconsole not found

**Symptoms**: Agent logs show "ipmiconsole not found: install freeipmi-tools
package"

**Cause**: FreeIPMI tools not installed on agent host

**Fix**:

```bash
# Debian/Ubuntu
sudo apt-get install freeipmi-tools

# RHEL/CentOS
sudo yum install freeipmi

# macOS
brew install freeipmi
```

#### 2. Terminal stays in raw mode after crash

**Symptoms**: Terminal doesn't respond to Enter key, characters not echoed

**Cause**: CLI crashed before restoring terminal state

**Fix**:

```bash
# Reset terminal to cooked mode
reset

# Or manually restore
stty sane
```

**Prevention**: The SOLTerminal handler uses `defer t.restore()` to ensure
terminal is always restored.

#### 3. Ctrl+C doesn't work in terminal streaming

**Symptoms**: Ctrl+C sends literal ^C to console instead of exiting

**Cause**: Fixed in recent commit - terminal was in blocking mode

**Fix**: Update to latest version with non-blocking stdin reader and Ctrl+C
detection (byte 0x03)

**Code Location**: `cli/pkg/terminal/sol.go:257-265`

#### 4. Connection hangs after "Connected to server console"

**Symptoms**: Message printed but no console output appears

**Cause**: ipmiconsole subprocess failed to start or SOL session rejected by BMC

**Debug**:

```bash
# Check agent logs for ipmiconsole errors
journalctl -u local-agent -f

# Test ipmiconsole directly
ipmiconsole -h BMC_IP -u USERNAME -p PASSWORD
```

#### 5. Web console works but terminal streaming doesn't

**Symptoms**: Browser console works fine, but `--terminal` flag hangs or fails

**Cause**: Different code paths - web console uses WebSocket, terminal uses
Connect RPC

**Debug**:

- Check if gateway supports Connect RPC on correct port
- Verify HTTP/2 (h2c) is enabled on gateway
- Check firewall allows gRPC traffic

**Fix**:

```yaml
# gateway config
server:
    grpc_port: 8082  # Must be accessible from CLI
    enable_h2c: true  # Required for Connect RPC
```

#### 6. Console output is garbled or has escape sequences

**Symptoms**: Strange characters like `^[[0m` or `^M` appear

**Cause**: Terminal not properly handling ANSI escape codes

**Fix (Terminal Streaming)**:

- Ensure terminal supports ANSI (most modern terminals do)
- Use `TERM=xterm-256color` environment variable

**Fix (Web Console)**:

- XTerm.js handles this automatically
- Check browser console for JavaScript errors

#### 7. SOL session steals existing console session

**Symptoms**: "SOL session already active" error, or existing session gets
disconnected

**Cause**: BMC only supports one SOL session at a time (common limitation)

**Fix**:

- Close existing SOL session first
- Use `--dont-steal` flag with ipmiconsole (if available)
- Agent uses `--serial-keepalive` to maintain persistent sessions

**Alternative**: Some BMCs support multiple sessions - check BMC configuration:

```
# Redfish example
GET /redfish/v1/Systems/{id}/SerialConsole
{
  "MaxConcurrentSessions": 4  # BMC allows 4 concurrent SOL sessions
}
```

### Network Tracing

For deep protocol debugging:

**WebSocket (Web Console)**:

```bash
# Browser DevTools Network tab
# Filter: WS (WebSocket)
# Inspect frames: Text (JSON) or Binary
```

**Connect RPC (Terminal Streaming)**:

```bash
# Capture gRPC traffic with tcpdump
tcpdump -i any -w console-stream.pcap port 8082

# Analyze with Wireshark
# Protocol: HTTP/2
wireshark console-stream.pcap
```

**IPMI SOL Traffic**:

```bash
# Capture IPMI RMCP+ packets
tcpdump -i any port 623 -w ipmi-sol.pcap

# Analyze IPMI protocol
wireshark ipmi-sol.pcap
```
