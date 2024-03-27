package core

import (
    "context"
    "errors"
    "github.com/openmesh-network/core/internal/bft"
    "github.com/openmesh-network/core/internal/config"
    "github.com/openmesh-network/core/internal/database"
    "github.com/openmesh-network/core/internal/logger"
    "github.com/openmesh-network/core/internal/networking/p2p"
    "net/http"
    "time"
)

// Instance is the top-level instance
type Instance struct {
    pi  *p2p.Instance
    DB  *database.Instance
    BFT *bft.Instance
    srv *http.Server
}

// NewInstance initialise an empty top-level instance
func NewInstance() *Instance {
    ins := &Instance{}
    ins.srv = &http.Server{}
    return ins
}

func (i *Instance) SetP2pInstance(pi *p2p.Instance) *Instance {
    i.pi = pi
    return i
}

func (i *Instance) SetDBInstance(db *database.Instance) *Instance {
    i.DB = db
    return i
}

func (i *Instance) SetBFTInstance(bft *bft.Instance) *Instance {
    i.BFT = bft
    return i
}

// Start the top-level instance as well as all the low-level instances
func (i *Instance) Start() {
    err := i.pi.Start()
    if err != nil {
        logger.Fatalf("Failed to start p2p instance: %s", err.Error())
    }

    i.BFT.Start()

    // Initialise pprof http server
    if config.Config.PProf.Enabled {
        i.srv.Addr = config.Config.PProf.Addr
        logger.Infof("Successfully initialised pprof http server on %s", config.Config.PProf.Addr)
        go func() {
            if err := i.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
                logger.Fatalf("Failed to start pprof http server: %s", err.Error())
            }
        }()
    }
}

// Stop the top-level instance as well as all the low-level instances
func (i *Instance) Stop() {
    if err := i.pi.Stop(); err != nil {
        logger.Errorf("Failed to stop p2p instance: %s", err.Error())
    }

    if err := i.BFT.Stop(); err != nil {
        logger.Errorf("Failed to stop CometBFT instance: %s", err.Error())
    }

    // Shutdown the pprof http server
    if config.Config.PProf.Enabled {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
        defer cancel()
        if err := i.srv.Shutdown(shutdownCtx); err != nil {
            logger.Errorf("Failed to shutdown pprof server: %v\n", err)
        }
    }
}
