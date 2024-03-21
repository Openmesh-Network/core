package logger

import (
    "github.com/stretchr/testify/assert"
    "github.com/openmesh-network/core/internal/config"
    "testing"
)

func TestInitLogger(t *testing.T) {
    config.Path = "../../"
    config.Name = "config"
    config.ParseConfig()
    // This initialises a production logger and print JSON-styled logs in the console
    InitLogger()
    Infof("This is test logger: %s", "INFO level")
    Errorf("This is also test logger: %s", "ERROR level")
    err := SyncAll()
    assert.NoError(t, err)
}
