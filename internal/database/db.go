package database

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/openmesh-network/core/internal/logger"
)

// Instance is the instance that holds the database connection
type Instance struct {
	Conn *badger.DB
}

func NewInstance() (*Instance, error) {
	i := &Instance{}
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))

	i.Conn = db
	if err != nil {
		logger.Fatalf("Opening database: %v", err)
	}
	return i, nil
}
