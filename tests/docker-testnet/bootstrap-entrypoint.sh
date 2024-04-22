#!/bin/sh
# Copy the default config to the container
cp -r /tmp/cometbft-home/ /core/default-cometbft-home

# Remove the persistent_peers from config.toml
cd /core/default-cometbft-home/config && \
  sed -i 's/persistent_peers = "2a41ce3551fdf4dfdd2100b7ebb02538b7b7e996@192.168.20.250:26656"/persistent_peers = ""/' config.toml && \
  sed -i 's/prometheus = false/prometheus = true/' config.toml && \
  sed -i 's/seed_mode = false/seed_mode = true/' config.toml

# Find ipv4 address
ADDRESS="$(hostname -i):26656"
sed -i "s/external_address = \"\"/external_address = \"$ADDRESS\"/" /core/default-cometbft-home/config/config.toml

# Then the true entrypoint
cd /core && /core/openmesh-core --config /core/conf/config.yml
