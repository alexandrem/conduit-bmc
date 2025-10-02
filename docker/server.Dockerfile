# Server Container with SOL and VNC support
FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install system packages for SOL, VNC, and server simulation
RUN apt-get update && apt-get install -y \
    x11vnc \
    xvfb \
    xterm \
    twm \
    fluxbox \
    netcat-traditional \
    sudo \
    net-tools \
    systemd \
    systemd-sysv \
    util-linux \
    openssh-server \
    curl \
    wget \
    vim \
    htop \
    dbus \
    init \
    iproute2 \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Configure systemd for container
RUN systemctl set-default multi-user.target && \
    systemctl mask systemd-remount-fs.service \
                   dev-hugepages.mount \
                   sys-fs-fuse-connections.mount \
                   systemd-logind.service \
                   getty.target \
                   console-getty.service

# Enable serial console (SOL) on ttyS0
RUN systemctl enable serial-getty@ttyS0.service

# Create server user
RUN useradd -m -s /bin/bash -G sudo server && \
    echo 'server:server' | chpasswd && \
    echo 'root:root' | chpasswd

# Configure SSH for remote access
RUN systemctl enable ssh && \
    mkdir -p /var/run/sshd

# Create server startup script
COPY docker/scripts/server-startup.sh /usr/local/bin/server-startup.sh
RUN chmod +x /usr/local/bin/server-startup.sh

# Create VNC startup script
COPY docker/scripts/server-vnc-startup.sh /usr/local/bin/server-vnc-startup.sh
RUN chmod +x /usr/local/bin/server-vnc-startup.sh

# Create server status web interface script
COPY docker/scripts/server-web-interface.sh /usr/local/bin/server-web-interface.sh
RUN chmod +x /usr/local/bin/server-web-interface.sh

# Expose SSH and VNC ports
EXPOSE 22 5901 8080

# Create server data directory
RUN mkdir -p /var/lib/server && \
    chown server:server /var/lib/server

# Set environment variables
ENV DISPLAY=:1
ENV VNC_PORT=5901
ENV SOL_DEVICE=/dev/ttyS0
ENV SERVER_ID=01

# Health check - verify Xvfb is running
HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 \
    CMD pgrep Xvfb || exit 1

# Use custom entrypoint that starts all services
CMD ["/usr/local/bin/server-startup.sh"]