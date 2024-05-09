package bft

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"

	cfg "github.com/cometbft/cometbft/config"
	cmtflags "github.com/cometbft/cometbft/libs/cli/flags"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	bftp2p "github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	"github.com/cometbft/cometbft/types"
	"google.golang.org/protobuf/proto"

	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/dgraph-io/badger/v3"
	"github.com/openmesh-network/core/collector"
	"github.com/openmesh-network/core/config"
	abci "github.com/openmesh-network/core/internal/bft/abci"
	otypes "github.com/openmesh-network/core/internal/bft/types"
	log "github.com/openmesh-network/core/internal/logger"
	"github.com/spf13/viper"
)

// Instance is the CometBFT instance
type Instance struct {
	Config     *cfg.Config
	Addr       []byte
	BftNode    *nm.Node
	Collector  *collector.CollectorInstance
	app        *abci.VerificationApp
	collector  *collector.CollectorInstance
	FullPubKey []byte
}

// NewInstance initialise a CometBFT instance use the config specified
func NewInstance(db *badger.DB, collector *collector.CollectorInstance) (*Instance, error) {
	conf := cfg.DefaultConfig()
	homeDir := config.Config.BFT.HomeDir
	conf.SetRoot(homeDir)

	log.Info("Loaded config: ", homeDir)

	// Parse CometBFT config
	bftConf := viper.New()
	// TODO XXX: Embed this into the executable instead of loading it from filesystem.
	bftConf.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))
	if err := bftConf.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := bftConf.Unmarshal(conf); err != nil {
		return nil, err
	}
	if err := conf.ValidateBasic(); err != nil {
		return nil, err
	}

	pv := privval.LoadFilePV(
		conf.PrivValidatorKeyFile(),
		conf.PrivValidatorStateFile(),
	)
	publicKey, err := pv.GetPubKey()
	if err != nil {
		panic(err)
	}

	app := abci.NewVerificationApp(publicKey.Bytes(), db)

	nodeKey, err := bftp2p.LoadNodeKey(conf.NodeKeyFile())
	if err != nil {
		return nil, err
	}

	log := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))
	log, err = cmtflags.ParseLogLevel(conf.LogLevel, log, cfg.DefaultLogLevel)
	if err != nil {
		return nil, err
	}

	// Create CometBFT node
	node, err := nm.NewNode(
		conf,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		nm.DefaultGenesisDocProviderFunc(conf),
		cfg.DefaultDBProvider,
		nm.DefaultMetricsProvider(conf.Instrumentation),
		log,
	)

	if err != nil {
		return nil, err
	}

	temp := sha256.Sum256(publicKey.Bytes())
	InstancePub := publicKey.Bytes()
	return &Instance{Config: conf, BftNode: node, app: app, collector: collector, Addr: temp[:20], FullPubKey: InstancePub}, nil
}

// Start the CometBFT node
func (inst *Instance) Start(ctx context.Context) {
	eventBus := inst.BftNode.EventBus()

	base64AddrString := base64.StdEncoding.EncodeToString(inst.FullPubKey)

	newBlock, err := eventBus.Subscribe(ctx, "mainId", types.EventQueryValidBlock)
	if err != nil {
		panic(err)
	}

	registered := false
	registerSentTransaction := false

	// Event handler
	go func() {
		for {
			select {
			case <-ctx.Done():
				eventBus.UnsubscribeAll(ctx, "mainId")
				return
			case <-newBlock.Canceled():
				eventBus.UnsubscribeAll(ctx, "mainId")
				return
			case <-newBlock.Out():
				env, err := inst.BftNode.ConfigureRPC()
				if err != nil {
					panic(err)
				}

				// If not connected, then connect!
				if !registered {
					res, err := env.Status(&rpctypes.Context{})
					if err != nil {
						panic(err)
					}

					// Now you can access the fields of the ResultStatus struct
					log.Warn("Latest block height: ", res.SyncInfo.LatestBlockHeight)

					if res.SyncInfo.LatestBlockHeight > 0 {

						validatorSet, err := env.StateStore.LoadValidators(res.SyncInfo.LatestBlockHeight)
						if err != nil {
							panic(err)
						}

						temp := sha256.Sum256(inst.FullPubKey)
						addr := temp[:20]

						if validatorSet.HasAddress(addr) {
							log.Info("Turns out we're registered!")
							registered = true
						} else {
							if !registerSentTransaction {
								transactionMessage := otypes.Transaction{
									Owner:     base64AddrString,
									Signature: "",
									Type:      *otypes.TransactionType_NodeRegistrationTransaction.Enum(),
									Data: &otypes.Transaction_NodeRegistrationData{
										NodeRegistrationData: &otypes.NodeRegistrationTransactionData{
											NodeAddress:     base64AddrString,
											NodeAttestation: "",
											NodeSignature:   "",
										},
									},
								}

								transactionBytes, err := proto.Marshal(&transactionMessage)
								if err != nil {
									panic(err)
								}

								transaction := types.Tx(transactionBytes[:])

								log.Debug("Pushing registration transaction with hash: ", sha256.Sum256(transactionBytes))
								_, err = env.BroadcastTxAsync(&rpctypes.Context{}, transaction)

								if err != nil {
									log.Error("Failed to push registration transaction: ", err)
									// panic(err)
								} else {
									log.Debug("Succesfully pushed registration transaction!")
									registerSentTransaction = true
								}
							}
						}
					}
				} else {

					requests := inst.app.GetRequestsDue()
					requestsNext := inst.app.GetRequestsDueNext()

					log.Info("Requests for now: ", requests)
					log.Info("Requests for later: ", requestsNext)

					var summaries []collector.Summary
					if requests != nil {
						if config.Config.BFT.MockTransactions {
							// Mock transactions of a similar format.
						} else {
							summaries = inst.collector.SubmitRequests(requests, requestsNext, time.Now().Add(-time.Second))
						}
					} else {
						log.Debug("No requests this block :(")
					}

					transactionPushedCount := 0
					for i := range summaries {

						s := summaries[i]
						r := requests[i]

						// Format as a transactionMessage
						transactionMessage := otypes.Transaction{
							Owner:     base64AddrString,
							Signature: strconv.Itoa(i),
							Type:      *otypes.TransactionType_VerificationTransaction.Enum(),
						}

						// Build some dataset.

						transactionMessage.Data = &otypes.Transaction_VerificationData{
							VerificationData: &otypes.VerificationTransactionData{
								// XXX: Decide how we're handling attestations.
								Attestation: "",
								// XXX: Need to decide how we're building the cids.
								// There's a tradeoff between blockchain size and download speed.
								// Make a fake but plausible CID
								// - It's currently a CID per byte.
								Cid:        s.DataHashes[0].String(),
								Datasource: r.Source.Name + "-" + r.Source.Topics[r.Topic],
								// XXX: Should this be the time it started being recorded or ended?
								Timestamp: time.Now().Unix(),
							},
						}

						transactionBytes, err := proto.Marshal(&transactionMessage)
						if err != nil {
							panic(err)
						}

						transaction := types.Tx(transactionBytes[:])

						env, err := inst.BftNode.ConfigureRPC()

						if err != nil {
							fmt.Println(transaction)
							panic(err)
						}

						_, err = env.BroadcastTxAsync(&rpctypes.Context{}, transaction)

						if err != nil {
							log.Error("Couldn't push transaction, reason: ", err)
							// panic(err)
						} else {
							transactionPushedCount++
						}
					}

					log.Debug("Pushed ", transactionPushedCount, "/", len(summaries))
				}
			}
		}
	}()

	go func() {
		// This blocks:
		if err := inst.BftNode.Start(); err != nil {
			log.Fatalf("Failed to start CometBFT node: %s", err.Error())
		}
	}()
}

// Stop the CometBFT node
func (i *Instance) Stop() error {
	err := i.BftNode.Stop()
	i.BftNode.Wait()
	return err
}
