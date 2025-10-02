# Redfish API Emulator Container using Sushy
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    gcc \
    python3-dev \
    libffi-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Sushy emulator and Docker Python client
RUN pip install --no-cache-dir \
    sushy-tools==1.1.0 \
    docker==7.0.0 \
    requests==2.31.0 \
    flask==2.3.3 \
    werkzeug==2.3.7

# Create application directories
RUN mkdir -p /var/lib/redfish /var/log/redfish /etc/redfish

# Copy Redfish emulator configuration and startup scripts
COPY docker/scripts/redfish-startup.sh /usr/local/bin/redfish-startup.sh
COPY docker/configs/redfish-config.conf /etc/redfish/redfish-config.conf

# Make scripts executable
RUN chmod +x /usr/local/bin/redfish-startup.sh

# Create redfish user
RUN useradd -m -s /bin/bash redfish && \
    chown -R redfish:redfish /var/lib/redfish /var/log/redfish

# Expose Redfish API port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8000/redfish/v1 || exit 1

# Start Redfish emulator
CMD ["/usr/local/bin/redfish-startup.sh"]