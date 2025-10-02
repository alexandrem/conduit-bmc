# noVNC Container for web-based VNC access
FROM alpine:3.18

# Install dependencies
RUN apk add --no-cache \
    bash \
    curl \
    netcat-openbsd \
    git \
    python3 py3-pip \
    && pip install --no-cache-dir six

# Clone noVNC and websockify
RUN git clone https://github.com/novnc/noVNC.git /opt/novnc && \
    git clone https://github.com/novnc/websockify.git /opt/websockify && \
    cd /opt/websockify && python3 setup.py install

# Copy startup script
COPY docker/scripts/novnc-startup.sh /usr/local/bin/novnc-startup.sh
RUN chmod +x /usr/local/bin/novnc-startup.sh

# Create log directory
RUN mkdir -p /var/log/novnc

# Expose ports for multiple servers
EXPOSE 6080 6081 6082

# Healthcheck (for first server)
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:6080 || exit 1

# Default command
CMD ["/usr/local/bin/novnc-startup.sh"]
