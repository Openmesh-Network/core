package core

import (
	"log"
	"github.com/openmesh-network/core/internal/networking/p2p"
)

// Instance is the top-level instance
type Instance struct {
	pi *p2p.Instance
}

// NewInstance initialise an empty top-level instance
func NewInstance() *Instance {
	return &Instance{}
}

func (i *Instance) SetP2pInstance(pi *p2p.Instance) *Instance {
	i.pi = pi
	return i
}

// Start the top-level instance as well as all the low-level instances
func (i *Instance) Start() {
	err := i.pi.Start()
	if err != nil {
		log.Fatalf("Failed to start p2p instance: %s", err.Error())
	}
}

// Stop the top-level instance as well as all the low-level instances
func (i *Instance) Stop() {
	if err := i.pi.Stop(); err != nil {
		log.Printf("Failed to stop p2p instance: %s", err.Error())
	}
}
