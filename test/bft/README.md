# CometBFT Testing (Docker Compose)

## Usage

```shell
# Build docker image
chmod +x ./build-test.sh
./build-test.sh

# Start the docker-compose
docker-compose up

# Or use the docker run command
docker run --rm -v ./config:/core/conf -v ./dump:/core/dump --name openmesh-core openmesh-core
```

## Configuration file generation

Generate default CometBFT configuration for testing (already included in the Dockerfile):

```shell
go run github.com/cometbft/cometbft/cmd/cometbft@v0.38.0 init --home /tmp/cometbft-home
```

## Heap dump and performance measurement

**TODO: The CometBFT command is not generating heap dumps. Need to find out how to "provide profile address".** Reference: [CometBFT Debugging](https://docs.cometbft.com/v0.38/tools/debugging)

Install `cometbft` command for heap dump (already included in the Dockerfile):

```shell
go install github.com/cometbft/cometbft/cmd/cometbft@latest
```

Generate heap dump:

```shell
docker exec -it <name-of-the-container> sh
cometbft debug dump /core/dump --home=/tmp/cometbft-home --pprof-laddr=http://127.0.0.1:9081
```

Then you can find the heap dump files (`.zip` archive) under the `dump` directory.

## Configuration files assignment

`test/bft/config` is the root directory for the configuration files used by containers. This directory will be mounted into the docker container as `/core/conf`. Using the flag `--config` to specify which config file will be used by the container in `docker-compose.yml`.

## CometBFT log suppression

Use the following command in `Dockerfile` to suppress the CometBFT log to prevent them from spamming the stdout (already included in the Dockerfile).

```shell
sed -i 's/log_level = "info"/log_level = "error"' /tmp/cometbft-home/config/config.toml
```

## Scaling

Add or change the following configuration to `docker-compose.yml` to scale up:

```yaml
services:
  core-peer-1:
    image: openmesh-core:latest
    # Add or change the replica configuration to scale up
    deploy:
      replicas: 2
```
