#!/bin/bash
set -e

# VirtualBMC startup script for container environment
# This script initializes VirtualBMC to control Docker containers via IPMI

echo "Starting VirtualBMC initialization..."

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
VBMC_SERVER_NAME=${VBMC_SERVER_NAME:-"server-01"}
VBMC_DOCKER_CONTAINER=${VBMC_DOCKER_CONTAINER:-"server-01"}
IPMI_USERNAME=${IPMI_USERNAME:-"ipmiusr"}
IPMI_PASSWORD=${IPMI_PASSWORD:-"test"}
IPMI_PORT=${IPMI_PORT:-623}
IPMI_ADDRESS=${IPMI_ADDRESS:-"0.0.0.0"}

echo "VirtualBMC Configuration:"
echo "  Server Name: $VBMC_SERVER_NAME"
echo "  Docker Container: $VBMC_DOCKER_CONTAINER"
echo "  IPMI Username: $IPMI_USERNAME"
echo "  IPMI Port: $IPMI_PORT"
echo "  IPMI Address: $IPMI_ADDRESS"

# Wait for target container to be running
echo "Waiting for target container '$VBMC_DOCKER_CONTAINER' to be running..."
timeout=120
while [ $timeout -gt 0 ]; do
    if docker ps --format "table {{.Names}}" | grep -q "^${VBMC_DOCKER_CONTAINER}$"; then
        echo "Target container '$VBMC_DOCKER_CONTAINER' is running"
        break
    fi
    echo "Waiting for container '$VBMC_DOCKER_CONTAINER'... ($timeout seconds remaining)"
    sleep 5
    timeout=$((timeout-5))
done

if [ $timeout -le 0 ]; then
    echo "ERROR: Container '$VBMC_DOCKER_CONTAINER' not running after 120 seconds"
    exit 1
fi

# Initialize VirtualBMC configuration directory
vbmcd_config_dir="/var/lib/vbmc"
mkdir -p "$vbmcd_config_dir"

# Start libvirt daemons
echo "Starting libvirt daemons..."
mkdir -p /var/run/libvirt
mkdir -p /var/log/libvirt
mkdir -p /var/log/libvirt/qemu
mkdir -p /etc/libvirt
mkdir -p /var/lib/libvirt/qemu

# Configure libvirt to disable security driver (not needed in container)
# Must create config before starting daemon
cat > /etc/libvirt/qemu.conf <<EOF
# Disable security driver for container environment
security_driver = "none"
user = "root"
group = "root"
dynamic_ownership = 0
remember_owner = 0

# Disable cgroup management in container
cgroup_controllers = []
cgroup_device_acl = []
EOF

# Also configure main libvirtd config
cat > /etc/libvirt/libvirtd.conf <<EOF
# Listen for local connections only
listen_tls = 0
listen_tcp = 0
unix_sock_group = "root"
unix_sock_rw_perms = "0777"
auth_unix_ro = "none"
auth_unix_rw = "none"
EOF

# Start virtlogd (required for VM console logging)
/usr/sbin/virtlogd -d

# Start libvirtd
/usr/sbin/libvirtd -d
sleep 3

# Verify libvirt is running
if ! virsh -c qemu:///system list > /dev/null 2>&1; then
    echo "ERROR: libvirt failed to start"
    exit 1
fi
echo "libvirt started successfully"

# Note: We don't need libvirt networking for VirtualBMC
# VirtualBMC only requires the VM to be defined in libvirt, not running

# Create a minimal QEMU VM for the server
echo "Creating QEMU VM for '$VBMC_SERVER_NAME'..."
VM_DISK="/var/lib/libvirt/images/${VBMC_SERVER_NAME}.qcow2"
mkdir -p /var/lib/libvirt/images

# Create a small disk image (1GB)
if [ ! -f "$VM_DISK" ]; then
    qemu-img create -f qcow2 "$VM_DISK" 1G
fi

# Create VM definition manually (virt-install has Python compatibility issues)
echo "Creating VM definition for '$VBMC_SERVER_NAME'..."
cat > /tmp/${VBMC_SERVER_NAME}.xml <<EOF
<domain type='qemu'>
  <name>$VBMC_SERVER_NAME</name>
  <memory unit='MiB'>512</memory>
  <vcpu>1</vcpu>
  <os>
    <type arch='x86_64' machine='pc'>hvm</type>
    <boot dev='hd'/>
  </os>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='$VM_DISK'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <serial type='file'>
      <source path='/var/log/libvirt/qemu/${VBMC_SERVER_NAME}-serial.log'/>
      <target port='0'/>
    </serial>
  </devices>
</domain>
EOF

virsh -c qemu:///system define /tmp/${VBMC_SERVER_NAME}.xml
echo "VM '$VBMC_SERVER_NAME' created"

# Start VirtualBMC daemon
echo "Starting VirtualBMC daemon..."
vbmcd --foreground &
VBMCD_PID=$!
sleep 2

# Add the BMC for the VM
echo "Adding BMC for VM '$VBMC_SERVER_NAME'..."
vbmc add "$VBMC_SERVER_NAME" \
    --port "$IPMI_PORT" \
    --address "$IPMI_ADDRESS" \
    --username "$IPMI_USERNAME" \
    --password "$IPMI_PASSWORD"

# Start the BMC
echo "Starting BMC for '$VBMC_SERVER_NAME'..."
vbmc start "$VBMC_SERVER_NAME"

# Show BMC status
echo "VirtualBMC Status:"
vbmc list

echo "VirtualBMC is ready! IPMI server listening on ${IPMI_ADDRESS}:${IPMI_PORT}"
echo "Test with: ipmitool -I lanplus -H <host> -p ${IPMI_PORT} -U ${IPMI_USERNAME} -P ${IPMI_PASSWORD} power status"

# Keep the container running
tail -f /dev/null