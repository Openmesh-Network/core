package verificationApp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"math/rand"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/dgraph-io/badger/v3"
	"google.golang.org/protobuf/proto"

	// "math/rand"
	crypt "github.com/cometbft/cometbft/proto/tendermint/crypto"
	"github.com/openmesh-network/core/collector"
	"github.com/openmesh-network/core/config"
	"github.com/openmesh-network/core/internal/bft/types"
	log "github.com/openmesh-network/core/internal/logger"
)

type VerificationApp struct {
	db                         *badger.DB
	onGoingBlock               *badger.Txn
	publicKey                  []byte
	validatorPrioritiesCurrent [][]collector.Request
	validatorPrioritiesNext    [][]collector.Request
	validatorFreeThisRound     []bool
	votesCurrent               []abcitypes.VoteInfo
	votesNext                  []abcitypes.VoteInfo
}

const VALIDATOR_PREALLOCATED_COUNT = 2000

var _ abcitypes.Application = (*VerificationApp)(nil)

func findAddressInPriorities(publicKey []byte, votes []abcitypes.VoteInfo, priorities [][]collector.Request) []collector.Request {
	for i := range priorities {
		validator := votes[i].GetValidator()

		temp := sha256.Sum256(publicKey)
		addr := temp[:20]
		log.Info(validator.Address, addr)

		if bytes.Equal(validator.Address, addr) {
			log.Info("Found priorities for node.")

			return priorities[i]
		}
	}

	return nil
}

func (app *VerificationApp) GetRequestsDue() []collector.Request {
	return findAddressInPriorities(app.publicKey, app.votesCurrent, app.validatorPrioritiesCurrent)
}
func (app *VerificationApp) GetRequestsDueNext() []collector.Request {
	return findAddressInPriorities(app.publicKey, app.votesNext, app.validatorPrioritiesNext)
}

func (app *VerificationApp) InitChain(_ context.Context, chain *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	return &abcitypes.ResponseInitChain{}, nil
}

func (app *VerificationApp) PrepareProposal(_ context.Context, proposal *abcitypes.RequestPrepareProposal) (*abcitypes.ResponsePrepareProposal, error) {

	// Only accept transactions that fit in the correct order?

	// Will currently accept all transactions.

	return &abcitypes.ResponsePrepareProposal{Txs: proposal.Txs}, nil
}
func (app *VerificationApp) ProcessProposal(_ context.Context, proposal *abcitypes.RequestProcessProposal) (*abcitypes.ResponseProcessProposal, error) {

	// Supposedly it's bad for performance to reject crappy blocks.
	// I think we should be a strict as possible, and give death penalty to misbehaving nodes basically.

	return &abcitypes.ResponseProcessProposal{Status: abcitypes.ResponseProcessProposal_ACCEPT}, nil
}

func (app *VerificationApp) FinalizeBlock(_ context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	var txs = make([]*abcitypes.ExecTxResult, len(req.Txs))

	app.onGoingBlock = app.db.NewTransaction(true)
	var validatorupdates = make([]abcitypes.ValidatorUpdate, 0, len(req.Txs))

	for i, tx := range req.Txs {
		if code := app.isValid(tx); code != 0 {
			// log.Error("Error: invalid transaction index %v", i)
			txs[i] = &abcitypes.ExecTxResult{Code: code}
		} else {
			var transaction types.Transaction
			err := proto.Unmarshal(tx, &transaction)

			if err != nil {
				log.Error("Error unmarshaling transaction data:", err)
				txs[i] = &abcitypes.ExecTxResult{Code: 1}
			}

			switch transaction.Type {
			case types.TransactionType_NormalTransaction:
				normalData := &types.NormalTransactionData{}
				normalData = transaction.GetNormalData()
				// log.Debug("Resource Transaction Data:", transaction)
				if normalData == nil {
					log.Error("Error: Normal Data is nil %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				res := app.handleNormalTransaction(*normalData)
				if res != 0 {
					log.Error("Error: Response from normal Data is not proper %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				txs[i] = &abcitypes.ExecTxResult{}
				// log.Debug("Normal Transaction Data:", normalData)
			case types.TransactionType_VerificationTransaction:
				verificationData := &types.VerificationTransactionData{}
				verificationData = transaction.GetVerificationData()
				if err != nil {
					log.Error("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				res := app.handleVerificationTransaction(*verificationData)
				// log.Debug("Handle transaction recieved")
				if res != 0 {
					log.Error("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				txs[i] = &abcitypes.ExecTxResult{}
				// log.Debug("Verification Transaction Data:", verificationData)
			case types.TransactionType_ResourceTransaction:
				resourceData := &types.ResourceTransactionData{}
				resourceData = transaction.GetResourceData()
				// log.Debug("Resource Transaction Data:", transaction)
				if err != nil {
					log.Error("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				res := app.handleResourceTransaction(*resourceData)
				if res != 0 {
					log.Error("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				txs[i] = &abcitypes.ExecTxResult{}
				// log.Debug("Resource Transaction Data:", resourceData)

			case types.TransactionType_NodeRegistrationTransaction:
				registrationData := &types.NodeRegistrationTransactionData{}
				registrationData = transaction.GetNodeRegistrationData()

				publicKeyString := registrationData.GetNodeAddress()
				pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyString)
				if err != nil {
					log.Error("Error decoding Base64:", err)

				}

				var publicKeyMessage = &crypt.PublicKey{
					Sum: &crypt.PublicKey_Ed25519{
						Ed25519: pubKeyBytes,
					},
				}

				if err != nil {
					log.Error("Error marshalling PublicKey message:", err)
				}

				if err != nil {
					log.Debug("problem alert", err)
				}

				if err != nil {
					// Handle error, e.g., invalid public key format
					log.Error("Error: invalid pubkey index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}

				txs[i] = &abcitypes.ExecTxResult{}
				validatorup := &abcitypes.ValidatorUpdate{
					PubKey: *publicKeyMessage,
					Power:  10,
				}
				validatorupdates = append(validatorupdates, *validatorup)
				log.Debug("Node succesfully registered: ", len(validatorupdates))
				// log.Debug("Node Registration Transaction Data:", registrationData)

			default:
				log.Error("Unknown transaction type")
				txs[i] = &abcitypes.ExecTxResult{Code: code}
			}

		}
	}

	// Select sources pseudo-randomly.
	if !config.Config.BFT.SkipSourceSelection {
		// Turn hash to 64 bit integer to use as rand seed.
		var r *rand.Rand
		{
			var seed int64
			hashPrevious := req.GetHash()
			for i := range hashPrevious {
				seed ^= int64(hashPrevious[i])
				seed <<= 8
			}
			r = rand.New(rand.NewSource(seed))
		}

		app.validatorPrioritiesCurrent = app.validatorPrioritiesCurrent[:0]
		app.validatorPrioritiesCurrent = append(app.validatorPrioritiesCurrent, app.validatorPrioritiesNext...)

		app.votesCurrent = app.votesCurrent[:0]
		app.votesCurrent = append(app.votesCurrent, app.votesNext...)

		app.votesNext = req.DecidedLastCommit.Votes

		// Not sure what the right number of rounds is :shrug:. Chosing arbitrarily.
		roundAmount := 1
		validatorCount := len(app.votesNext)

		if validatorCount > VALIDATOR_PREALLOCATED_COUNT {
			// XXX: Handle more intelligently.
			app.validatorFreeThisRound = make([]bool, validatorCount)
			app.validatorPrioritiesNext = make([][]collector.Request, validatorCount)
		} else {
			// Go through voters and pick set that voted.
			app.validatorFreeThisRound = app.validatorFreeThisRound[:validatorCount]
			app.validatorPrioritiesNext = app.validatorPrioritiesNext[:validatorCount]
		}

		for i := range app.validatorPrioritiesNext {
			app.validatorPrioritiesNext[i] = make([]collector.Request, 0, roundAmount)
		}
		log.Info("Started source selection.")

		// NOTE(Tom): This algorithm gives earlier sources higher priority.
		for round := 0; round < roundAmount && len(app.validatorPrioritiesNext) > 0; round++ {
			for i := range app.validatorFreeThisRound {
				app.validatorFreeThisRound[i] = true
			}

			r.Shuffle(len(app.validatorPrioritiesNext), func(i, j int) {
				{
					temp := app.validatorFreeThisRound[j]
					app.validatorFreeThisRound[j] = app.validatorFreeThisRound[i]
					app.validatorFreeThisRound[i] = temp
				}

				{
					temp := app.validatorPrioritiesNext[j]
					app.validatorPrioritiesNext[j] = app.validatorPrioritiesNext[i]
					app.validatorPrioritiesNext[i] = temp
				}
			})

			for i := range collector.Sources {
				for j := range collector.Sources[i].Topics {
					for k := range app.validatorFreeThisRound {

						if app.validatorFreeThisRound[k] {

							// Make sure that they're not already assigned to this source.
							alreadyAssigned := false
							for _, req := range app.validatorPrioritiesNext[k] {
								if req.Source.Name == collector.Sources[i].Name && req.Topic == j {
									alreadyAssigned = true
								}
							}

							if !alreadyAssigned {
								app.validatorFreeThisRound[k] = false

								req := collector.Request{
									Source: collector.Sources[i],
									Topic:  j,
								}
								app.validatorPrioritiesNext[k] = append(app.validatorPrioritiesNext[k], req)

								break
							}
						}
					}
				}
			}
		}

		// Need to have this info available somewhere...

		// XXX: Remove this from finalizeblock? Remove from abci?
		// Only run this when requested maybe?

		log.Info("Done sorting preferences, writting our requests.")
	}

	return &abcitypes.ResponseFinalizeBlock{
		TxResults:        txs,
		ValidatorUpdates: validatorupdates,
	}, nil
}

func (app *VerificationApp) Query(_ context.Context, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	resp := abcitypes.ResponseQuery{Key: req.Data}

	dbErr := app.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(req.Data)
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			resp.Log = "key does not exist"
			return nil
		}

		return item.Value(func(val []byte) error {
			resp.Log = "exists"
			resp.Value = val
			return nil
		})
	})
	if dbErr != nil {
		log.Panicf("Error reading database, unable to execute query: %v", dbErr)
	}
	return &resp, nil
}

func (app *VerificationApp) isValid(tx []byte) uint32 {
	// check format
	var transaction types.Transaction
	err := proto.Unmarshal(tx, &transaction)
	// log.Debug("the tx type is", transaction.Type)
	if err != nil {
		log.Error("Error unmarshaling transaction data:", err)
		return 1
	}

	// Check the transaction type and handle accordingly
	switch transaction.Type {
	case types.TransactionType_NormalTransaction:
		// normalData := &types.NormalTransactionData{}
		// normalData = transaction.GetNormalData()
		// log.Info("Normal Transaction Data:", normalData)
		return 0
	case types.TransactionType_VerificationTransaction:
		// verificationData := &types.VerificationTransactionData{}
		// verificationData = transaction.GetVerificationData()
		// log.Info("Verification Transaction Data:", verificationData)
		return 0
	case types.TransactionType_ResourceTransaction:
		// resourceData := &types.ResourceTransactionData{}
		// resourceData = transaction.GetResourceData()
		// log.Info("Resource Transaction Data:", resourceData)
		return 0
	case types.TransactionType_NodeRegistrationTransaction:
		nodeRegistrationData := &types.NodeRegistrationTransactionData{}
		nodeRegistrationData = transaction.GetNodeRegistrationData()

		if nodeRegistrationData == nil {
			log.Error("Error unmarshaling resource transaction data:", err)
			return 1
		}
		// log.Debug("Resource Transaction Data:", nodeRegistrationData)
		return 0

	default:
		log.Error("Unknown transaction type")
		return 1
	}
}

func (app *VerificationApp) CheckTx(_ context.Context, check *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	code := app.isValid(check.Tx)

	// XXX: Skip all this if the type is not verification!

	{
		// Check against stored transactions and see if there's a higher priority transaction already stored.
		priority := 0
		priorityStoredHighest := 0

		if priority < priorityStoredHighest {
			// Transaction is not valid, a higher priority exists already.
			code = 1
		}
	}

	return &abcitypes.ResponseCheckTx{Code: code}, nil
}

func NewVerificationApp(publicKey []byte, db *badger.DB) *VerificationApp {
	return &VerificationApp{
		publicKey:                  publicKey,
		votesNext:                  make([]abcitypes.VoteInfo, 0, 100),
		votesCurrent:               make([]abcitypes.VoteInfo, 0, 100),
		validatorPrioritiesCurrent: make([][]collector.Request, 0, VALIDATOR_PREALLOCATED_COUNT),
		validatorPrioritiesNext:    make([][]collector.Request, 0, VALIDATOR_PREALLOCATED_COUNT),
		validatorFreeThisRound:     make([]bool, 0, VALIDATOR_PREALLOCATED_COUNT),
		db:                         db}
}

/**
func (app *KVStoreApplication) handleNodeRegistrationTransaction(tx types.NodeRegistrationTransactionData) types.NodeRegistrationTransactionData {
	log.Printf("Handle transaction recieved")
	return 0
}**/

func (app *VerificationApp) handleNormalTransaction(tx types.NormalTransactionData) uint32 {
	return 0
}

func (app *VerificationApp) handleVerificationTransaction(tx types.VerificationTransactionData) uint32 {
	return 0
}

func (app *VerificationApp) handleResourceTransaction(tx types.ResourceTransactionData) uint32 {
	return 0
}

func (app VerificationApp) Commit(_ context.Context, commit *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	return &abcitypes.ResponseCommit{}, app.onGoingBlock.Commit()
}

func (app *VerificationApp) ListSnapshots(_ context.Context, snapshots *abcitypes.RequestListSnapshots) (*abcitypes.ResponseListSnapshots, error) {
	return &abcitypes.ResponseListSnapshots{}, nil
}

func (app *VerificationApp) OfferSnapshot(_ context.Context, snapshot *abcitypes.RequestOfferSnapshot) (*abcitypes.ResponseOfferSnapshot, error) {
	return &abcitypes.ResponseOfferSnapshot{}, nil
}

func (app *VerificationApp) LoadSnapshotChunk(_ context.Context, chunk *abcitypes.RequestLoadSnapshotChunk) (*abcitypes.ResponseLoadSnapshotChunk, error) {
	return &abcitypes.ResponseLoadSnapshotChunk{}, nil
}

func (app *VerificationApp) ApplySnapshotChunk(_ context.Context, chunk *abcitypes.RequestApplySnapshotChunk) (*abcitypes.ResponseApplySnapshotChunk, error) {
	return &abcitypes.ResponseApplySnapshotChunk{Result: abcitypes.ResponseApplySnapshotChunk_ACCEPT}, nil
}

func (app VerificationApp) ExtendVote(_ context.Context, extend *abcitypes.RequestExtendVote) (*abcitypes.ResponseExtendVote, error) {
	return &abcitypes.ResponseExtendVote{}, nil
}

func (app *VerificationApp) VerifyVoteExtension(_ context.Context, verify *abcitypes.RequestVerifyVoteExtension) (*abcitypes.ResponseVerifyVoteExtension, error) {
	return &abcitypes.ResponseVerifyVoteExtension{}, nil
}

func (app *VerificationApp) Info(_ context.Context, info *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	return &abcitypes.ResponseInfo{}, nil
}
