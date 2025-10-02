#!/bin/bash
set -e

echo "Starting noVNC web interface..."

# Create log dir
mkdir -p /var/log/novnc

# VNC server mappings
VNC_SERVER_01=${VNC_SERVER_01:-"dev-server-01:5901"}
VNC_SERVER_02=${VNC_SERVER_02:-"dev-server-02:5901"}
VNC_SERVER_03=${VNC_SERVER_03:-"dev-server-03:5901"}
NOVNC_PORT_01=${NOVNC_PORT_01:-6080}
NOVNC_PORT_02=${NOVNC_PORT_02:-6081}
NOVNC_PORT_03=${NOVNC_PORT_03:-6082}

echo "Configured noVNC instances:"
echo "  $VNC_SERVER_01 -> $NOVNC_PORT_01"
echo "  $VNC_SERVER_02 -> $NOVNC_PORT_02"
echo "  $VNC_SERVER_03 -> $NOVNC_PORT_03"

start_novnc() {
    local name=$1
    local target=$2
    local port=$3
    local logfile="/var/log/novnc/novnc-${name}.log"

    echo "Starting noVNC for $name ($target) on port $port..."
    cd /opt/novnc
    python3 -m websockify --web /opt/novnc $port $target > $logfile 2>&1 &
    echo $!  # Return PID
}

# Wait for VNC servers to be ready
for server in "$VNC_SERVER_01" "$VNC_SERVER_02" "$VNC_SERVER_03"; do
    IFS=':' read host port <<< "$server"
    echo "Checking $host:$port..."
    for i in {1..12}; do  # max 60s wait
        if nc -z "$host" "$port"; then
            echo "$host:$port ready"
            break
        fi
        echo "Waiting for $host:$port..."
        sleep 5
    done
done

# Start servers and store PIDs
PID_01=$(start_novnc "server-01" "$VNC_SERVER_01" "$NOVNC_PORT_01")
PID_02=$(start_novnc "server-02" "$VNC_SERVER_02" "$NOVNC_PORT_02")
PID_03=$(start_novnc "server-03" "$VNC_SERVER_03" "$NOVNC_PORT_03")

# Optional: create index.html
cat > /opt/novnc/index.html <<EOF
<!DOCTYPE html>
<html>
<head><title>BMC noVNC Access</title></head>
<body>
<h1>BMC Servers</h1>
<ul>
<li><a href="/vnc.html?host=localhost&port=$NOVNC_PORT_01">Server 01</a></li>
<li><a href="/vnc.html?host=localhost&port=$NOVNC_PORT_02">Server 02</a></li>
<li><a href="/vnc.html?host=localhost&port=$NOVNC_PORT_03">Server 03</a></li>
</ul>
</body>
</html>
EOF

# Monitor processes
while true; do
    sleep 30
    if ! kill -0 "$PID_01" 2>/dev/null; then
        echo "PID_01 ($PID_01) exited, restarting..."
        PID_01=$(start_novnc "server-01" "$VNC_SERVER_01" "$NOVNC_PORT_01")
    fi
    if ! kill -0 "$PID_02" 2>/dev/null; then
        echo "PID_02 ($PID_02) exited, restarting..."
        PID_02=$(start_novnc "server-02" "$VNC_SERVER_02" "$NOVNC_PORT_02")
    fi
    if ! kill -0 "$PID_03" 2>/dev/null; then
        echo "PID_03 ($PID_03) exited, restarting..."
        PID_03=$(start_novnc "server-03" "$VNC_SERVER_03" "$NOVNC_PORT_03")
    fi
done
