# VirtualBMC Container for IPMI simulation
FROM python:3.11-slim

# Install system dependencies for VirtualBMC, libvirt, and QEMU
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    dnsmasq-base \
    gcc \
    gir1.2-glib-2.0 \
    gnupg \
    ipmitool \
    libcairo2-dev \
    libffi-dev \
    libgirepository-2.0-dev \
    libglib2.0-dev \
    libvirt-clients \
    libvirt-daemon-system \
    libvirt-dev \
    libvirt-glib-1.0-0 \
    libxml2 \
    libxml2-dev \
    libxt-dev \
    lsb-release \
    netcat-openbsd \
    pkg-config \
    procps \
    python3-dev \
    python3-gi \
    python3-libxml2 \
    qemu-system-x86 \
    qemu-utils \
    supervisor \
    virtinst \
    && rm -rf /var/lib/apt/lists/*

# Install Docker CLI
RUN mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce-cli && \
    rm -rf /var/lib/apt/lists/*

# Install VirtualBMC and Docker Python client
RUN pip install --no-cache-dir \
    virtualbmc==3.1.0 \
    docker==7.0.0 \
    pyghmi==1.5.56 \
    requests==2.31.0 \
    pycairo \
    PyGObject


# Create VirtualBMC directories
RUN mkdir -p /var/lib/vbmc /var/log/vbmc /etc/vbmc

# Copy VirtualBMC configuration and startup scripts
COPY docker/scripts/virtualbmc-startup.sh /usr/local/bin/virtualbmc-startup.sh
COPY docker/supervisor/virtualbmc-supervisord.conf /etc/supervisor/conf.d/virtualbmc-supervisord.conf

# Create VirtualBMC user
RUN useradd -m -s /bin/bash vbmc && \
    chown -R vbmc:vbmc /var/lib/vbmc /var/log/vbmc

# Expose IPMI port
EXPOSE 623/udp

# Health check - test if VirtualBMC daemon is running
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD pgrep -f vbmcd || exit 1

# Start supervisor to manage VirtualBMC daemon
CMD ["/usr/local/bin/virtualbmc-startup.sh"]
