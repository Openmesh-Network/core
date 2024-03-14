# Package Config

This directory is for supporting the configuration functionalities.

## Add New Configurations

**Step 1**: Specify a configuration `struct` anywhere like the struct below:

```go
// P2pConfig is the configuration for libp2p-related instances
type P2pConfig struct {
    Addr      string `yaml:"addr"`      // libp2p listening address (default: 0.0.0.0)
    Port      int    `yaml:"port"`      // libp2p listening port
    GroupName string `yaml:"groupName"` // Name used for discovering nodes via mDNS
    PeerLimit int    `yaml:"peerLimit"` // Max number of peers this node can establish connection to
}
```

Requirements:

1. This struct must be exported.
2. All the fields in the struct must be exported.
3. Use the `yaml` tag to specify the key for this config in the yml config file.

**Step 2**: Add this struct to the `config` struct:

```go
// config is the configuration structure for the whole Openmesh Core project
type config struct {
    P2P P2pConfig `yaml:"p2p"`
}
```

Requirements:

1. The field for this struct must be exported.
2. Use the `yaml` tag to specify the key for this config in the yml config file.

**Step 3**: Add configurations in `config.yml` (default configuration file) or other customised configuration file.

```yaml
p2p:
  # 0.0.0.0 = localhost
  addr: 0.0.0.0
  # 0 = random port
  port: 0
  groupName: xnode
  peerLimit: 50
```

**Step 4**: Retrieve your configuration value from the `config.Config` global variable:

```go
limit := config.Config.P2P.PeerLimit
```
