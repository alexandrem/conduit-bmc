#!/bin/bash
set -e

echo "Starting server container initialization..."

# Environment variables with defaults
SERVER_ID=${SERVER_ID:-"01"}
VNC_PORT=${VNC_PORT:-5901}
SOL_DEVICE=${SOL_DEVICE:-"/dev/ttyS0"}

echo "Server Configuration:"
echo "  Server ID: $SERVER_ID"
echo "  VNC Port: $VNC_PORT"
echo "  SOL Device: $SOL_DEVICE"

# Create log directory
mkdir -p /var/log/server

# Pick a free X display
DISPLAY_NUM=1
while [ -e /tmp/.X${DISPLAY_NUM}-lock ]; do
    DISPLAY_NUM=$((DISPLAY_NUM+1))
done
export DISPLAY=:$DISPLAY_NUM
rm -f /tmp/.X${DISPLAY_NUM}-lock /tmp/.X${DISPLAY_NUM}-key

echo "Using DISPLAY=$DISPLAY"

# Start Xvfb
echo "Starting Xvfb..."
Xvfb $DISPLAY -screen 0 1024x768x16 &
sleep 1

# Start x11vnc
echo "Starting x11vnc on port $VNC_PORT..."
x11vnc -display $DISPLAY -rfbport $VNC_PORT -nopw -forever -shared &
sleep 1

# Start a simple terminal in the virtual desktop
echo "Starting test desktop applications..."
xterm -geometry 80x24+10+10 -title "Server $SERVER_ID Console" &

# Optional: start SSH daemon (for development)
echo "Starting SSH daemon..."
/usr/sbin/sshd || true

# Optional: simulate serial console (for SOL)
if [ -c "$SOL_DEVICE" ]; then
    echo "Starting serial console on $SOL_DEVICE..."
    getty -L $SOL_DEVICE 115200 vt100 &
else
    echo "Warning: $SOL_DEVICE not available, SOL access may not work"
fi

# Create server info file
cat > /var/lib/server/server-info.txt << EOF
Server ID: $SERVER_ID
Hostname: $(hostname)
IP Address: $(hostname -I | awk '{print $1}')
Started: $(date)
DISPLAY: $DISPLAY
VNC Port: $VNC_PORT
SOL Device: $SOL_DEVICE
EOF

echo "✅ Server $SERVER_ID is ready!"
echo "Access methods:"
echo "  🖥️  VNC Desktop: Connect to $(hostname):$VNC_PORT"
echo "  📟 SOL Console: $SOL_DEVICE (if available)"
echo "  🔧 SSH Access: ssh server@$(hostname) (password: server)"
echo ""

# Monitor Xvfb and x11vnc
while true; do
    sleep 30

    if ! pgrep -f "Xvfb $DISPLAY" > /dev/null; then
        echo "Xvfb stopped, restarting..."
        Xvfb $DISPLAY -screen 0 1024x768x16 &
        sleep 1
        xterm -geometry 80x24+10+10 -title "Server $SERVER_ID Console" &
    fi

    if ! pgrep -f "x11vnc.*:$VNC_PORT" > /dev/null; then
        echo "x11vnc stopped, restarting..."
        x11vnc -display $DISPLAY -rfbport $VNC_PORT -nopw -forever -shared &
        sleep 1
    fi
done
