FROM golang:1.22-alpine

# Install CometBFT command for future operations
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" go install github.com/cometbft/cometbft/cmd/cometbft@latest

# Add the core binary file to /core directory
ADD ./openmesh-core /core/openmesh-core

# Create directories for logs and configuration files
RUN ["mkdir", "/core/logs"]
RUN ["mkdir", "/core/conf"]
RUN ["mkdir", "/core/dump"]

# Make the default CometBFT home dir and add the physical one to the image
# to make sure that all the config.toml and genesis.json are the same
RUN ["mkdir", "/tmp/cometbft-home"]
ADD default-cometbft-home /tmp/cometbft-home
ADD ./entrypoint.sh /core/entrypoint.sh
ADD ./bootstrap-entrypoint.sh /core/bootstrap-entrypoint.sh
ADD ./seeder-entrypoint.sh /core/seeder-entrypoint.sh
RUN ["chmod", "+x", "/core/entrypoint.sh"]
RUN ["chmod", "+x", "/core/bootstrap-entrypoint.sh"]
RUN ["chmod", "+x", "/core/seeder-entrypoint.sh"]
# RUN ["go", "install", "github.com/cometbft/cometbft/cmd/cometbft@latest"]

# Add a default entrypoint
# This will be overwrite in docker-compose.yml
ENTRYPOINT ["sh", "/core/entrypoint.sh"]
