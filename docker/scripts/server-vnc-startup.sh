#!/bin/bash
set -e

# Server VNC startup script
# This script starts Xvfb and x11vnc for VNC desktop access

echo "Starting VNC server setup..."

# Set environment variables with defaults
DISPLAY=${DISPLAY:-":1"}
VNC_PORT=${VNC_PORT:-5901}
RESOLUTION=${RESOLUTION:-"1024x768x16"}

echo "VNC Configuration:"
echo "  Display: $DISPLAY"
echo "  Port: $VNC_PORT"
echo "  Resolution: $RESOLUTION"

# Kill any existing Xvfb or x11vnc processes
pkill -f "Xvfb $DISPLAY" || true
pkill -f "x11vnc.*:$VNC_PORT" || true
sleep 2

# Start Xvfb (X Virtual Framebuffer)
echo "Starting Xvfb on display $DISPLAY..."
Xvfb $DISPLAY -screen 0 $RESOLUTION &
XVFB_PID=$!

# Wait for Xvfb to start
sleep 3

# Verify Xvfb is running
if ! pgrep -f "Xvfb $DISPLAY" > /dev/null; then
    echo "ERROR: Xvfb failed to start"
    exit 1
fi

# Set the display for subsequent commands
export DISPLAY=$DISPLAY

# Start x11vnc VNC server
echo "Starting x11vnc on port $VNC_PORT..."
x11vnc \
    -display $DISPLAY \
    -forever \
    -nopw \
    -rfbport $VNC_PORT \
    -shared \
    -bg \
    -o /var/log/server/x11vnc.log

# Wait for x11vnc to start
sleep 2

# Verify x11vnc is running
if ! pgrep -f "x11vnc.*:$VNC_PORT" > /dev/null; then
    echo "ERROR: x11vnc failed to start"
    exit 1
fi

# Start a window manager for better VNC experience
echo "Starting window manager and terminal..."
fluxbox &
sleep 2
xterm -geometry 100x30+50+50 -title "Server Console" &

echo "VNC server started successfully!"
echo "  Display: $DISPLAY"
echo "  Port: $VNC_PORT"
echo "  Address: 0.0.0.0:$VNC_PORT"
echo ""
echo "Connect with: vncviewer $(hostname):$VNC_PORT"