package database

import (
    "github.com/stretchr/testify/assert"
    "openmesh.network/openmesh-core/internal/config"
    "openmesh.network/openmesh-core/internal/logger"
    "testing"
)

func setup() {
    config.Path = "../../"
    config.Name = "config"
    config.ParseConfig()
    // This initialises a production logger and print JSON-styled logs in the console
    logger.InitLogger()
}

func teardown() {
    logger.SyncAll()
}

func TestNewInstance(t *testing.T) {
    setup()
    defer teardown()

    ins, err := NewInstance()
    if ins.Conn != nil {
        defer ins.Stop()
    }
    assert.NoError(t, err)
    err = ins.Start()
    assert.NoError(t, err)
}
