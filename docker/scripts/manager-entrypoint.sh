#!/bin/sh
set -e

# Export all BMC_ prefixed environment variables for Air to use
for var in $(env | grep '^BMC_' | cut -d= -f1); do
    export "$var"
done

# Export other important environment variables
export JWT_SECRET
export CONFIG_FILE

echo "Manager entrypoint: Starting Air with environment variables:"
env | grep -E '^(BMC_|JWT_|CONFIG_)' | sort

# Change to the manager directory and start Air
cd /workspace/manager
exec air -c .air.toml