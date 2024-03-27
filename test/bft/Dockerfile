FROM golang:1.22-alpine

# Add the core binary file to /core directory
ADD ./openmesh-core /core/openmesh-core

# Create directories for logs and configuration files
RUN ["mkdir", "/core/logs"]
RUN ["mkdir", "/core/conf"]

# Initialise CometBFT configuration using the default configuration
# go run github.com/cometbft/cometbft/cmd/cometbft@v0.38.0 init --home /tmp/cometbft-home
RUN ["go", "run", "github.com/cometbft/cometbft/cmd/cometbft@v0.38.0", "init", "--home", "/tmp/cometbft-home"]
# Suppress CometBFT logs to prevent them form spamming the stdout
RUN ["sed", "-i", "s/log_level = \"info\"/log_level = \"error\"/", "/tmp/cometbft-home/config/config.toml"]

# This ends here
# The Openmesh Core will be started in docker-compose.yml
ENTRYPOINT ["/core/openmesh-core", "--config", "/core/conf/config1.yml"]
