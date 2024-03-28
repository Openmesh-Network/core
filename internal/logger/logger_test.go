package logger

import (
	"testing"

	"github.com/openmesh-network/core/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestInitLogger(t *testing.T) {
	config.Path = "../../"
	config.Name = "config"
	config.ParseConfig(config.Path, true)
	// This initialises a production logger and print JSON-styled logs in the console
	InitLogger()
	Infof("This is test logger: %s", "INFO level")
	Errorf("This is also test logger: %s", "ERROR level")
	err := SyncAll()
	assert.NoError(t, err)
}
