#!/bin/sh
# Give it the same dumb key every time.
mkdir -p /core/default-cometbft-home/config/
echo "${COMET_NODE_KEY}" > /core/default-cometbft-home/config/node_key.json
rm /core/default-cometbft-home/config/priv_validator_key.json

echo ${COMET_NODE_KEY}

cometbft init --home /core/default-cometbft-home

cp /tmp/cometbft-home/config/genesis.json /core/default-cometbft-home/config/genesis.json
cp /tmp/cometbft-home/config/config.toml /core/default-cometbft-home/config/config.toml

# Remove the persistent_peers from config.toml
cd /core/default-cometbft-home/config && \
        sed -i 's/seed_mode = false/seed_mode = true/' config.toml && \
        sed -i 's/seeds = "2112a1fed60fa35e4044e7b8f0aebdc8e6d91f86@192.168.20.251:26656"/seeds = ""/' config.toml && \
        sed -i 's/max_num_inbound_peers = 20/max_num_inbound_peers = 1000/' config.toml


# Then the true entrypoint
cd /core && /core/openmesh-core --config /core/conf/config.yml
