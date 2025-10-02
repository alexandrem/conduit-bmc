#!/bin/bash
set -e

# Server web interface script
# This script provides a simple HTTP server showing server status

echo "Starting server status web interface..."

SERVER_ID=${SERVER_ID:-"01"}
WEB_PORT=${WEB_PORT:-8080}

# Create a simple status page
create_status_page() {
    cat > /tmp/server-status.html << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Server $SERVER_ID Status</title>
    <meta http-equiv="refresh" content="30">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background-color: #f5f5f5; }
        .container { background-color: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; }
        .status { background-color: #d4edda; color: #155724; padding: 10px; border-radius: 4px; margin: 10px 0; }
        .info { background-color: #e2e3e5; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .metric { margin: 5px 0; }
        pre { background-color: #f8f9fa; padding: 10px; border-radius: 4px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🖥️ Server $SERVER_ID Status Dashboard</h1>

        <div class="status">
            ✅ Server is running and accessible
        </div>

        <div class="info">
            <h3>📊 Server Information</h3>
            <div class="metric"><strong>Server ID:</strong> $SERVER_ID</div>
            <div class="metric"><strong>Hostname:</strong> $(hostname)</div>
            <div class="metric"><strong>IP Address:</strong> $(hostname -I | awk '{print $1}' || echo "N/A")</div>
            <div class="metric"><strong>Uptime:</strong> $(uptime -p 2>/dev/null || echo "N/A")</div>
            <div class="metric"><strong>Last Updated:</strong> $(date)</div>
        </div>

        <div class="info">
            <h3>🔌 Access Methods</h3>
            <div class="metric"><strong>📟 SOL Console:</strong> Available via IPMI (port 623)</div>
            <div class="metric"><strong>🖥️ VNC Desktop:</strong> Port $VNC_PORT</div>
            <div class="metric"><strong>🔧 SSH Access:</strong> Port 22 (user: server, pass: server)</div>
            <div class="metric"><strong>📡 IPMI BMC:</strong> Available via VirtualBMC</div>
            <div class="metric"><strong>🌐 Redfish API:</strong> Available via Sushy emulator</div>
        </div>

        <div class="info">
            <h3>🔄 Running Processes</h3>
            <pre>$(ps aux | head -10)</pre>
        </div>

        <div class="info">
            <h3>💾 Memory Usage</h3>
            <pre>$(free -h 2>/dev/null || echo "Memory information not available")</pre>
        </div>

        <div class="info">
            <h3>🌐 Network Interfaces</h3>
            <pre>$(ip addr show 2>/dev/null | grep -E "^\d|inet " || echo "Network information not available")</pre>
        </div>
    </div>
</body>
</html>
EOF
}

# Function to handle HTTP requests
handle_request() {
    local request_line
    read -r request_line

    # Extract the requested path
    local path=$(echo "$request_line" | awk '{print $2}')

    case "$path" in
        "/")
            # Regenerate status page with current information
            create_status_page

            echo "HTTP/1.1 200 OK"
            echo "Content-Type: text/html"
            echo "Connection: close"
            echo ""
            cat /tmp/server-status.html
            ;;
        "/health")
            echo "HTTP/1.1 200 OK"
            echo "Content-Type: text/plain"
            echo "Connection: close"
            echo ""
            echo "OK"
            ;;
        "/api/status")
            echo "HTTP/1.1 200 OK"
            echo "Content-Type: application/json"
            echo "Connection: close"
            echo ""
            echo "{\"server_id\":\"$SERVER_ID\",\"hostname\":\"$(hostname)\",\"status\":\"running\",\"timestamp\":\"$(date -Iseconds)\"}"
            ;;
        *)
            echo "HTTP/1.1 404 Not Found"
            echo "Content-Type: text/plain"
            echo "Connection: close"
            echo ""
            echo "Not Found"
            ;;
    esac
}

# Start simple HTTP server
echo "Starting HTTP server on port $WEB_PORT..."

while true; do
    # Use netcat to listen for HTTP requests
    if command -v nc >/dev/null 2>&1; then
        nc -l -p $WEB_PORT -c handle_request
    elif command -v ncat >/dev/null 2>&1; then
        ncat -l $WEB_PORT -c handle_request
    else
        echo "Warning: netcat not available, web interface disabled"
        sleep 60
    fi
done