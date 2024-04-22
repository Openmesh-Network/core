# CometBFT Documents for Openmesh Core

## Get Docker network IP address

Run the following command inside the docker container:

```shell
hostname -i
```

## Peer IDs

### Node Address ("uppercase id")

Example: `96675E49F94DFD5FE6341E994B36C07CD5224469`

Can be found in the `config/priv_validator_key.json` under the CometBFT configuration directory.

### Node ID ("lowercase id")

Example: `2a41ce3551fdf4dfdd2100b7ebb02538b7b7e996`

Can be got via the following command:

```shell
cometbft show-node-id --home <home-to-cometbft-config>
```

## Configuration

### persistent_peers

Peers to keeping persistent connections to. Typically used to specify the genesis node.

Configuration format:

```toml
persistent_peers = "node-id@ip:port,node-id1@ip1:port1"
```

Example:

```toml
persistent_peers = "2a41ce3551fdf4dfdd2100b7ebb02538b7b7e996@192.168.20.250:26656"
```

NOTE: `node-id` is the node id (lowercase id) for the persistent peer.
