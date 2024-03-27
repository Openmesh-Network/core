# CometBFT Testing (Docker Compose)

## Configuration file generation

Generate default CometBFT configuration for testing:

```shell
go run github.com/cometbft/cometbft/cmd/cometbft@v0.38.0 init --home /tmp/cometbft-home
```

## Configuration files assignment

`test/bft/config` is the root directory for the configuration files used by containers. This directory will be mounted into the docker container as `/core/conf`. Using the flag `--config` to specify which config file will be used by the container in `docker-compose.yml`.

## CometBFT log suppression

Use the following command in `Dockerfile` to suppress the CometBFT log to prevent them from spamming the stdout.

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
