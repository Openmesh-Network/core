package config

import (
    "flag"
    "path/filepath"
    "strings"
)

var (
    fullPath string // Path (absolute/relative) and Name (default: config.yml) to config file
    Path     string // Path to config file
    Name     string // Name (without extension) of config file
)

func ParseFlags() {
    flag.StringVar(&fullPath, "config", "./config.yml", "Configuration file Name and Path")

    flag.Parse()

    // Setup Path and Name to configuration file
    fileName := filepath.Base(fullPath)
    Path = strings.TrimSuffix(fullPath, fileName)
    Name = strings.TrimSuffix(fileName, ".yml")
}
