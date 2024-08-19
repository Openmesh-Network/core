# Openmesh Core

This is the repository of Openmesh Core which can operate as standalone software or as apart of an Xnode.

## What is Openmesh Core?
Openmesh core is node software for interacting with the openmesh network to collect, seed and fetch data chosen by the Openmesh DAO. 
The core operates within an Intel SGX trusted execution environment, which it uses to prove to other nodes that it is running the same software.

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
## System Requirements
Further testing is required to determine the specs needed to run this software. TO-DO

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

## Openmesh Core Architecture
The openmesh core currently consists of the following software stack: 

1. CometBFT engine for consensus
2. BadgerDB for storage 
3. LibP2P for networking
