# Openmesh Core Testnet

## Usage

Build the docker images:

```shell
chmod +x ./build-test.sh
./build-test.sh
```

Start the Docker Compose cluster:

```shell
docker-compose up -d
```

Start the genesis node only:

```shell
docker-compose up core-bootstrap-validator -d
```

Start the non-genesis nodes only:

```shell
docker-compose up core-validator -d
```
