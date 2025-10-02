#!/bin/sh
set -e

# Export all BMC_ prefixed environment variables for Air to use
for var in $(env | grep '^BMC_' | cut -d= -f1); do
    export "$var"
done

# Export other important environment variables
export CONFIG_FILE

echo "Local Agent entrypoint: Starting Air with environment variables:"
env | grep -E '^(BMC_|CONFIG_)' | sort

# Update Air config to use the config file from environment
if [ -n "$CONFIG_FILE" ]; then
    # Create a temporary Air config with the correct config file
    sed "s|args_bin = \[\]|args_bin = [\"-config=$CONFIG_FILE\"]|" .air.toml > tmp/air-runtime.toml
    AIR_CONFIG="tmp/air-runtime.toml"
else
    AIR_CONFIG=".air.toml"
fi

# Change to the local-agent directory and start Air
cd /workspace/local-agent
exec air -c "$AIR_CONFIG"