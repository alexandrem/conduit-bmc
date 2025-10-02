#!/bin/bash
set -e

# Redfish emulator startup script for container environment
# This script initializes Sushy emulator to provide Redfish API for Docker containers

echo "Starting Redfish emulator initialization..."

# Wait for Docker socket to be available
timeout=60
while [ $timeout -gt 0 ]; do
    if [ -S /var/run/docker.sock ]; then
        echo "Docker socket available"
        break
    fi
    echo "Waiting for Docker socket... ($timeout seconds remaining)"
    sleep 2
    timeout=$((timeout-2))
done

if [ $timeout -le 0 ]; then
    echo "ERROR: Docker socket not available after 60 seconds"
    exit 1
fi

# Set environment variables with defaults
SUSHY_EMULATOR_LISTEN_IP=${SUSHY_EMULATOR_LISTEN_IP:-"0.0.0.0"}
SUSHY_EMULATOR_LISTEN_PORT=${SUSHY_EMULATOR_LISTEN_PORT:-8000}
SUSHY_EMULATOR_SSL_CERT=${SUSHY_EMULATOR_SSL_CERT:-""}
SUSHY_EMULATOR_SSL_KEY=${SUSHY_EMULATOR_SSL_KEY:-""}
SUSHY_EMULATOR_OS_CLOUD=${SUSHY_EMULATOR_OS_CLOUD:-""}
SUSHY_EMULATOR_LIBVIRT_URI=${SUSHY_EMULATOR_LIBVIRT_URI:-""}
SUSHY_EMULATOR_IGNORE_BOOT_DEVICE=${SUSHY_EMULATOR_IGNORE_BOOT_DEVICE:-"True"}
SUSHY_EMULATOR_BOOT_LOADER_MAP=${SUSHY_EMULATOR_BOOT_LOADER_MAP:-""}
SUSHY_EMULATOR_VMEDIA_VERIFY_SSL=${SUSHY_EMULATOR_VMEDIA_VERIFY_SSL:-"True"}
SUSHY_EMULATOR_AUTH_FILE=${SUSHY_EMULATOR_AUTH_FILE:-""}
SUSHY_EMULATOR_SYSTEMS=${SUSHY_EMULATOR_SYSTEMS:-"server-01:server-01"}

echo "Redfish Emulator Configuration:"
echo "  Listen IP: $SUSHY_EMULATOR_LISTEN_IP"
echo "  Listen Port: $SUSHY_EMULATOR_LISTEN_PORT"
echo "  Systems: $SUSHY_EMULATOR_SYSTEMS"
echo "  Ignore Boot Device: $SUSHY_EMULATOR_IGNORE_BOOT_DEVICE"

# Create a simple Redfish server instead of relying on Sushy + Docker
echo "Creating simple Redfish HTTP server..."

cat > /tmp/simple-redfish-server.py << 'EOF'
#!/usr/bin/env python3
import json
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import logging
import time

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class RedfishHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        path = self.path
        logger.info(f"GET request: {path}")

        if path == "/redfish/v1":
            self.send_service_root()
        elif path.startswith("/redfish/v1/Systems"):
            self.send_systems()
        elif path.startswith("/redfish/v1/Chassis"):
            self.send_chassis()
        else:
            self.send_not_found()

    def do_POST(self):
        path = self.path
        content_length = int(self.headers.get('Content-Length', 0))
        post_data = self.rfile.read(content_length)

        logger.info(f"POST request: {path}")

        if "Actions" in path:
            self.send_action_response()
        else:
            self.send_not_found()

    def send_service_root(self):
        response = {
            "@odata.type": "#ServiceRoot.v1_0_0.ServiceRoot",
            "@odata.id": "/redfish/v1",
            "Id": "ServiceRoot",
            "Name": "BMC Redfish Service",
            "RedfishVersion": "1.0.0",
            "UUID": "12345678-1234-1234-1234-123456789012",
            "Systems": {"@odata.id": "/redfish/v1/Systems"},
            "Chassis": {"@odata.id": "/redfish/v1/Chassis"}
        }
        self.send_json_response(response)

    def send_systems(self):
        if self.path == "/redfish/v1/Systems":
            response = {
                "@odata.type": "#ComputerSystemCollection.ComputerSystemCollection",
                "@odata.id": "/redfish/v1/Systems",
                "Name": "Computer System Collection",
                "Members": [
                    {"@odata.id": "/redfish/v1/Systems/server-01"}
                ],
                "Members@odata.count": 1
            }
        else:
            # Individual system
            response = {
                "@odata.type": "#ComputerSystem.v1_0_0.ComputerSystem",
                "@odata.id": "/redfish/v1/Systems/server-01",
                "Id": "server-01",
                "Name": "Server 01",
                "SystemType": "Physical",
                "PowerState": "On",
                "Status": {"State": "Enabled", "Health": "OK"},
                "Actions": {
                    "#ComputerSystem.Reset": {
                        "target": "/redfish/v1/Systems/server-01/Actions/ComputerSystem.Reset"
                    }
                }
            }
        self.send_json_response(response)

    def send_chassis(self):
        response = {
            "@odata.type": "#ChassisCollection.ChassisCollection",
            "@odata.id": "/redfish/v1/Chassis",
            "Name": "Chassis Collection",
            "Members": [
                {"@odata.id": "/redfish/v1/Chassis/server-01"}
            ],
            "Members@odata.count": 1
        }
        self.send_json_response(response)

    def send_action_response(self):
        response = {"Message": "Action completed successfully"}
        self.send_json_response(response)

    def send_json_response(self, data, status=200):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        self.wfile.write(json.dumps(data, indent=2).encode())

    def send_not_found(self):
        self.send_response(404)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        error = {"error": "Resource not found"}
        self.wfile.write(json.dumps(error).encode())

    def log_message(self, format, *args):
        # Override to use our logger
        logger.info(f"{self.address_string()} - {format % args}")

# Start the server
server_address = ('0.0.0.0', 8000)
httpd = HTTPServer(server_address, RedfishHandler)

logger.info("Starting Redfish server on {}:{}".format(server_address[0], server_address[1]))
logger.info("Redfish Service Root: http://{}:{}/redfish/v1".format(server_address[0], server_address[1]))

try:
    httpd.serve_forever()
except KeyboardInterrupt:
    logger.info("Shutting down Redfish server")
    httpd.shutdown()
EOF

python3 /tmp/simple-redfish-server.py

# Create Sushy emulator configuration file
cat > /tmp/sushy-emulator-config.conf << EOF
SUSHY_EMULATOR_LISTEN_IP = "$SUSHY_EMULATOR_LISTEN_IP"
SUSHY_EMULATOR_LISTEN_PORT = $SUSHY_EMULATOR_LISTEN_PORT
SUSHY_EMULATOR_SSL_CERT = "$SUSHY_EMULATOR_SSL_CERT"
SUSHY_EMULATOR_SSL_KEY = "$SUSHY_EMULATOR_SSL_KEY"
SUSHY_EMULATOR_OS_CLOUD = "$SUSHY_EMULATOR_OS_CLOUD"
SUSHY_EMULATOR_LIBVIRT_URI = "$SUSHY_EMULATOR_LIBVIRT_URI"
SUSHY_EMULATOR_IGNORE_BOOT_DEVICE = $SUSHY_EMULATOR_IGNORE_BOOT_DEVICE
SUSHY_EMULATOR_BOOT_LOADER_MAP = "$SUSHY_EMULATOR_BOOT_LOADER_MAP"
SUSHY_EMULATOR_VMEDIA_VERIFY_SSL = $SUSHY_EMULATOR_VMEDIA_VERIFY_SSL
SUSHY_EMULATOR_AUTH_FILE = "$SUSHY_EMULATOR_AUTH_FILE"
EOF

# Set up Docker-based system configuration
export SUSHY_EMULATOR_DOCKER_SYSTEMS="$SUSHY_EMULATOR_SYSTEMS"

echo "Starting Sushy Redfish emulator..."

# Start Sushy emulator with Docker driver
exec sushy-emulator \
    --config /tmp/sushy-emulator-config.conf \
    --interface docker \
    --systems "$SUSHY_EMULATOR_SYSTEMS"