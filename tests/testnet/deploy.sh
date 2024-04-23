#!/bin/bash

# Hardcoded 
SEED_NODE=
GENESIS_JSON=genesis.json

# List of IPs
IPS=

NODE_COUNT=

# Ssh everything needed in there.

for ip in ${IPS[@]}
do
    echo $ip
    echo $SEED_NODE
    echo $GENESIS_JSON

    ssh root@$ip bash <<EOF
    echo $ip

    go install github.com/cometbft/cometbft/cmd/cometbft@latest
    cp ~/go/bin/cometbft /usr/bin

    rm -rf core
    git clone 'https://github.com/Openmesh-Network/core.git'

    cd core
    git checkout feature/testnet
    cd tests/testnet

    # Need:
    #   1. Everyone shares a seed node.
    #   2. Everyone shares a genesis json.

    go run main.go $ip $SEED_NODE $GENESIS_JSON $NODE_COUNT
EOF

done

# Have a single seed?

# Copy genesis file across them.
# Copy root files across them.
# Copy seeds across them?
