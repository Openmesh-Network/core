package config

import (
	"github.com/spf13/viper"
	"log"
	"strings"
)

// Config is a global variable that hold all the configurations need by the whole project
// NOTE: It must be initialised via ParseConfig() before use
var Config config

// config is the configuration structure for the whole Openmesh Core project
type config struct {
	P2P P2pConfig `yaml:"p2p"`
	BFT BFTConfig `yaml:"bft"`
	Log LogConfig `yaml:"log"`
	DB  DBConfig  `yaml:"db"`
}

// P2pConfig is the configuration for libp2p-related instances
type P2pConfig struct {
	Addr      string `yaml:"addr"`      // libp2p listening address (default: 0.0.0.0)
	Port      int    `yaml:"port"`      // libp2p listening port
	GroupName string `yaml:"groupName"` // Name used for discovering nodes via mDNS
	PeerLimit int    `yaml:"peerLimit"` // Max number of peers this node can establish connection to
}

// DBConfig is the configuration for database connection and operation
type DBConfig struct {
	Username string `yaml:"username"` // Username for the specific database to be connected
	Password string `yaml:"password"` // Password for the specific database to be connected
	Port     int    `yaml:"port"`     // Database connection port
	DBName   string `yaml:"dbName"`   // Name for the database used
	URL      string `yaml:"URL"`      // Database connection URL
}

// BFTConfig is the configuration for using CometBFT
type BFTConfig struct {
	HomeDir string `yaml:"homeDir"` // Path to CometBFT config
}

// LogConfig is the configuration for zap logger
type LogConfig struct {
	Development bool           `yaml:"development"`   // Development logger has DEBUG level and is more human-friendly
	Encoding    string         `yaml:"encoding"`      // Default: JSON for production
	InfoConfig  InfoLogConfig  `mapstructure:"info"`  // Sub-config for info-level logs
	ErrorConfig ErrorLogConfig `mapstructure:"error"` // Sub-config for error-level logs
}

type InfoLogConfig struct {
	FileName   string `yaml:"fileName"`   // Name and path to the info log
	MaxSize    int    `yaml:"maxSize"`    // Megabytes
	MaxAge     int    `yaml:"maxAge"`     // Days
	MaxBackups int    `yaml:"maxBackups"` // How much old info log files to retain
	ToStdout   bool   `yaml:"toStdout"`   // Log to stdout (except file) or not
	ToFile     bool   `yaml:"toFile"`     // Log to file or not
}

type ErrorLogConfig struct {
	FileName   string `yaml:"fileName"`   // Name and path to the error log
	MaxSize    int    `yaml:"maxSize"`    // Megabytes
	MaxAge     int    `yaml:"maxAge"`     // Days
	MaxBackups int    `yaml:"maxBackups"` // How much old error log files to retain
	ToStderr   bool   `yaml:"toStderr"`   // Log to stderr (except file) or not
	ToFile     bool   `yaml:"toFile"`     // Log to file or not
}

type CollectorConfig struct {
	ApiKeys map[string]string `yaml:"apiKeys"` // API keys for each authenticated source
}

// ParseConfig parses the yml configuration file and initialise the Config variable
func ParseConfig(configAtCompileTime string, allowRuntimeConfigFile bool) {
	coreConf := viper.New()

	if Name == "" {
		r := strings.NewReader(configAtCompileTime)
		if err := coreConf.ReadConfig(r); err != nil {
			// This should NEVER run! We can't allow faulty configs to be compiled to the executable.
			panic(err)
		}
	} else if allowRuntimeConfigFile {
		coreConf.AddConfigPath(Path)
		coreConf.SetConfigName(Name)
		coreConf.SetConfigType("yaml")
		if err := coreConf.ReadInConfig(); err != nil {
			log.Fatalf("Failed to read the configuration: %s", err.Error())
		}
	} else {
		log.Fatalf("No config file found in compiled binary and no runtime config file allowed.")
	}

	if err := coreConf.Unmarshal(&Config); err != nil {
		log.Fatalf("Failed to parse the configuration: %s", err.Error())
	}
}
