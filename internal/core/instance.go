package core

import (
    "openmesh.network/openmesh-core/internal/database"
    "openmesh.network/openmesh-core/internal/logger"
    "openmesh.network/openmesh-core/internal/networking/p2p"
)

// Instance is the top-level instance
type Instance struct {
    pi *p2p.Instance
    DB *database.Instance
}

// NewInstance initialise an empty top-level instance
func NewInstance() *Instance {
    return &Instance{}
}

func (i *Instance) SetP2pInstance(pi *p2p.Instance) *Instance {
    i.pi = pi
    return i
}

func (i *Instance) SetDBInstance(db *database.Instance) *Instance {
    i.DB = db
    return i
}

// Start the top-level instance as well as all the low-level instances
func (i *Instance) Start() {
    err := i.pi.Start()
    if err != nil {
        logger.Fatalf("Failed to start p2p instance: %s", err.Error())
    }

    err = i.DB.Start()
    if err != nil {
        i.DB.Stop()
        logger.Fatalf("Failed to establish PostgreSQL connection: %s", err.Error())
    }
}

// Stop the top-level instance as well as all the low-level instances
func (i *Instance) Stop() {
    if err := i.pi.Stop(); err != nil {
        logger.Errorf("Failed to stop p2p instance: %s", err.Error())
    }

    if err := i.DB.Stop(); err != nil {
        logger.Errorf("Failed to stop PostgreSQL connection: %s", err.Error())
    }
}
