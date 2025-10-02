#!/bin/sh
set -e

# Export all BMC_ prefixed environment variables for Air to use
for var in $(env | grep '^BMC_' | cut -d= -f1); do
    export "$var"
done

# Export other important environment variables
export JWT_SECRET
export GATEWAY_ID
export REGION
export CONFIG_FILE

echo "Gateway entrypoint: Starting Air with environment variables:"
env | grep -E '^(BMC_|JWT_|GATEWAY_|REGION|CONFIG_)' | sort

# Change to the gateway directory and start Air
cd /workspace/gateway
exec air -c .air.toml