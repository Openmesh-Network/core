# Openmesh Core

This is the repository of Openmesh Core.

## Usage

Default usage:

```shell
go run main.go
```

Specify customised configuration file:

```shell
go run main.go -config <path-to-config>/config.yml
```

The default value for `-config` is `./config.yml`.

Build and run:

```shell
go build -o openmesh-core
./openmesh-core -config <path-to-config>/config.yml
```

## Project Configuration

- p2p: Libp2p networking configurations.
    - addr: Libp2p listening address, `0.0.0.0` for localhost.
    - port: Libp2p listening port, `0` for random port.
    - groupName: For classifying nodes. Only nodes with the same `groupName` can discover each other.
    - peerLimit: How many peers this node can have (inclusive).

## Project Layout Guide

- Root directory:
  - `config.yml`: Project configuration file (see the sections above for usage).
  - `internal/`: Unexported (private) libraries.
    - `config/`: Project configuration support.
    - `core/`: Top-level instance and libraries.
    - `networking/`: Networking supporting libraries (for both overlay networking and inter-node networking).
      - `p2p/`: P2P networking implementations (based on `libp2p`).
  - `main.go`: The main function.
  - Potential exported (public) libraries.

## PProf Performance Measurement

Enable pprof in the configuration file:

```yaml
pprof:
  enabled: true
  addr: 127.0.0.1:9081
```

After starting the application, use the following commands to get a heap dump:

```shell
# Generate heap dump file
curl http://127.0.0.1:9081/debug/pprof/heap > heap.out

# Generate goroutine dump file (optional)
curl http://127.0.0.1:9081/debug/pprof/goroutine> goroutine.out

# Open the heap dump in a more human-friendly way
go tool pprof -http=:9082 heap.out
```
