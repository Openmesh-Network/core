package config

import (
    "github.com/spf13/viper"
    "log"
)

// Config is a global variable that hold all the configurations need by the whole project
// NOTE: It must be initialised via ParseConfig() before use
var Config config

// config is the configuration structure for the whole Openmesh Core project
type config struct {
    P2P P2pConfig `yaml:"p2p"`
}

// P2pConfig is the configuration for libp2p-related instances
type P2pConfig struct {
    Addr      string `yaml:"addr"`      // libp2p listening address (default: 0.0.0.0)
    Port      int    `yaml:"port"`      // libp2p listening port
    GroupName string `yaml:"groupName"` // Name used for discovering nodes via mDNS
    PeerLimit int    `yaml:"peerLimit"` // Max number of peers this node can establish connection to
}

// ParseConfig parses the yml configuration file and initialise the Config variable
func ParseConfig() {
    viper.AddConfigPath(Path)
    viper.SetConfigName(Name)
    viper.SetConfigType("yaml")

    if err := viper.ReadInConfig(); err != nil {
        log.Fatalf("Failed to read the configuration: %s", err.Error())
    }
    if err := viper.Unmarshal(&Config); err != nil {
        log.Fatalf("Failed to parse the configuration: %s", err.Error())
    }
}
