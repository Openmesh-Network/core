package verificationApp

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"log"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/dgraph-io/badger/v3"
	"openmesh.network/openmesh-core/internal/bft/types"

	crypt "github.com/cometbft/cometbft/proto/tendermint/crypto"

	"github.com/golang/protobuf/proto"
)

type KVStoreApplication struct {
	db           *badger.DB
	onGoingBlock *badger.Txn
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{db: db}
}
func (app *KVStoreApplication) Info(_ context.Context, info *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	return &abcitypes.ResponseInfo{}, nil
}

func (app *KVStoreApplication) Query(_ context.Context, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
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
func (app *KVStoreApplication) CheckTx(_ context.Context, check *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	code := app.isValid(check.Tx)
	return &abcitypes.ResponseCheckTx{Code: code}, nil
}
func (app *KVStoreApplication) InitChain(_ context.Context, chain *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	return &abcitypes.ResponseInitChain{}, nil
}

func (app *KVStoreApplication) PrepareProposal(_ context.Context, proposal *abcitypes.RequestPrepareProposal) (*abcitypes.ResponsePrepareProposal, error) {
	return &abcitypes.ResponsePrepareProposal{Txs: proposal.Txs}, nil
}
func (app *KVStoreApplication) ProcessProposal(_ context.Context, proposal *abcitypes.RequestProcessProposal) (*abcitypes.ResponseProcessProposal, error) {
	return &abcitypes.ResponseProcessProposal{Status: abcitypes.ResponseProcessProposal_ACCEPT}, nil
}

func (app *KVStoreApplication) FinalizeBlock(_ context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	var txs = make([]*abcitypes.ExecTxResult, len(req.Txs))
	var validatorupdates = make([]abcitypes.ValidatorUpdate, 0, len(req.Txs))
	app.onGoingBlock = app.db.NewTransaction(true)
	for i, tx := range req.Txs {
		if code := app.isValid(tx); code != 0 {
			log.Printf("Error: invalid transaction index %v", i)
			txs[i] = &abcitypes.ExecTxResult{Code: code}
		} else {
			var transaction types.Transaction
			hexString := string(tx)
			tx, _ = hex.DecodeString(hexString)
			err := proto.Unmarshal(tx, &transaction)
			if err != nil {
				log.Printf("Error unmarshaling transaction data:", err)
				txs[i] = &abcitypes.ExecTxResult{Code: 1}
			}

			switch transaction.Type {
			case types.TransactionType_NormalTransaction:
				normalData := &types.NormalTransactionData{}
				normalData = transaction.GetNormalData()
				log.Printf("Resource Transaction Data:", transaction)
				if normalData == nil {
					log.Printf("Error: Normal Data is nil %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				res := app.handleNormalTransaction(*normalData)
				if res != 0 {
					log.Printf("Error: Response from normal Data is not proper %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				txs[i] = &abcitypes.ExecTxResult{}
				log.Printf("Normal Transaction Data:", normalData)
			case types.TransactionType_VerificationTransaction:
				verificationData := &types.VerificationTransactionData{}
				verificationData = transaction.GetVerificationData()
				if err != nil {
					log.Printf("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				res := app.handleVerificationTransaction(*verificationData)
				log.Printf("Handle transaction recieved")
				if res != 0 {
					log.Printf("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				txs[i] = &abcitypes.ExecTxResult{}
				log.Printf("Verification Transaction Data:", verificationData)
			case types.TransactionType_ResourceTransaction:
				resourceData := &types.ResourceTransactionData{}
				resourceData = transaction.GetResourceData()
				log.Printf("Resource Transaction Data:", transaction)
				if err != nil {
					log.Printf("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				res := app.handleResourceTransaction(*resourceData)
				if res != 0 {
					log.Printf("Error: invalid transaction index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}
				txs[i] = &abcitypes.ExecTxResult{}
				log.Printf("Resource Transaction Data:", resourceData)

			case types.TransactionType_NodeRegistrationTransaction:
				registrationData := &types.NodeRegistrationTransactionData{}
				registrationData = transaction.GetNodeRegistrationData()
				log.Printf("Resource Transaction Data:", registrationData)
				publicKeyString := registrationData.GetNodeAddress()
				pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyString)
				if err != nil {
					log.Printf("Error decoding Base64:", err)

				}

				var publicKeyMessage = &crypt.PublicKey{
					Sum: &crypt.PublicKey_Ed25519{
						Ed25519: pubKeyBytes,
					},
				}

				if err != nil {
					log.Printf("Error marshalling PublicKey message:", err)

				}

				if err != nil {
					log.Printf("problem alert", err)
				}

				if err != nil {
					// Handle error, e.g., invalid public key format
					log.Printf("Error: invalid pubkey index %v", i)
					txs[i] = &abcitypes.ExecTxResult{Code: 1}
				}

				txs[i] = &abcitypes.ExecTxResult{}
				validatorup := &abcitypes.ValidatorUpdate{
					PubKey: *publicKeyMessage,
					Power:  10,
				}
				validatorupdates = append(validatorupdates, *validatorup)
				log.Printf("Resource Transaction Data:", registrationData)

			default:
				log.Printf("Unknown transaction type")
				txs[i] = &abcitypes.ExecTxResult{Code: code}
			}

		}
	}

	return &abcitypes.ResponseFinalizeBlock{
		TxResults:        txs,
		ValidatorUpdates: validatorupdates,
	}, nil
}

func (app KVStoreApplication) Commit(_ context.Context, commit *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	return &abcitypes.ResponseCommit{}, app.onGoingBlock.Commit()
}

func (app *KVStoreApplication) ListSnapshots(_ context.Context, snapshots *abcitypes.RequestListSnapshots) (*abcitypes.ResponseListSnapshots, error) {
	return &abcitypes.ResponseListSnapshots{}, nil
}

func (app *KVStoreApplication) OfferSnapshot(_ context.Context, snapshot *abcitypes.RequestOfferSnapshot) (*abcitypes.ResponseOfferSnapshot, error) {
	return &abcitypes.ResponseOfferSnapshot{}, nil
}

func (app *KVStoreApplication) LoadSnapshotChunk(_ context.Context, chunk *abcitypes.RequestLoadSnapshotChunk) (*abcitypes.ResponseLoadSnapshotChunk, error) {
	return &abcitypes.ResponseLoadSnapshotChunk{}, nil
}

func (app *KVStoreApplication) ApplySnapshotChunk(_ context.Context, chunk *abcitypes.RequestApplySnapshotChunk) (*abcitypes.ResponseApplySnapshotChunk, error) {
	return &abcitypes.ResponseApplySnapshotChunk{Result: abcitypes.ResponseApplySnapshotChunk_ACCEPT}, nil
}

func (app KVStoreApplication) ExtendVote(_ context.Context, extend *abcitypes.RequestExtendVote) (*abcitypes.ResponseExtendVote, error) {
	return &abcitypes.ResponseExtendVote{}, nil
}

func (app *KVStoreApplication) VerifyVoteExtension(_ context.Context, verify *abcitypes.RequestVerifyVoteExtension) (*abcitypes.ResponseVerifyVoteExtension, error) {
	return &abcitypes.ResponseVerifyVoteExtension{}, nil
}

/**
func (app *KVStoreApplication) ExecuteTransaction(tx []byte) uint32 {
	var transaction types.Transaction
	hexString := string(tx)
	tx, _ = hex.DecodeString(hexString)
	err := proto.Unmarshal(tx, &transaction)
	if err != nil {
		log.Printf("Error unmarshaling transaction data:", err)
		return 1
	}

	switch transaction.Type {
	case types.TransactionType_NormalTransaction:
		normalData := &types.NormalTransactionData{}
		normalData = transaction.GetNormalData()
		log.Printf("Resource Transaction Data:", transaction)
		if err != nil {
			log.Printf("Error unmarshaling normal transaction data:", err)
			return 1
		}
		res := app.handleNormalTransaction(*normalData)
		if res != 0{
			log.Printf("Error occured in executing transaction")
			return 1
		}
		return 0
		log.Printf("Normal Transaction Data:", normalData)
	case types.TransactionType_VerificationTransaction:
		verificationData := &types.VerificationTransactionData{}
		verificationData = transaction.GetVerificationData()
		if err != nil {
			log.Printf("Error unmarshaling verification transaction data:", err)
			return 1
		}
		res := app.handleVerificationTransaction(*verificationData)
		log.Printf("Handle transaction recieved")
		if res != 0{
			log.Printf("Error occured in executing transaction")
			return 1
		}
		return 0
		log.Printf("Verification Transaction Data:", verificationData)
	case types.TransactionType_ResourceTransaction:
		resourceData := &types.ResourceTransactionData{}
		resourceData = transaction.GetResourceData()
		log.Printf("Resource Transaction Data:", transaction)
		if err != nil {
			log.Printf("Error unmarshaling resource transaction data:", err)
			return 1
		}
		res := app.handleResourceTransaction(*resourceData)
		if res != 0{
			log.Printf("Error occured in executing transaction")
			return 1
		}
		return 0
		log.Printf("Resource Transaction Data:", resourceData)

	case types.TransactionType_NodeRegistrationTransaction:
		registrationData := &types.NodeRegistrationTransactionData{}
		registrationData = transaction.GetNodeRegistrationData()
		log.Printf("Resource Transaction Data:", registrationData)
		res := app.handleNodeRegistrationTransaction(*registrationData)
		if res != 0{
			log.Printf("Error occured in executing transaction")
			return 1
		}
		return 0
		log.Printf("Resource Transaction Data:", registrationData)

	default:
		log.Printf("Unknown transaction type")
		return 1
	}
	return 0
}**/

func (app *KVStoreApplication) handleNormalTransaction(tx types.NormalTransactionData) uint32 {
	return 0
}

func (app *KVStoreApplication) handleVerificationTransaction(tx types.VerificationTransactionData) uint32 {
	return 0
}

func (app *KVStoreApplication) handleResourceTransaction(tx types.ResourceTransactionData) uint32 {
	return 0
}

/**
func (app *KVStoreApplication) handleNodeRegistrationTransaction(tx types.NodeRegistrationTransactionData) types.NodeRegistrationTransactionData {
	log.Printf("Handle transaction recieved")
	return 0
}**/

func (app *KVStoreApplication) isValid(tx []byte) uint32 {
	// check format
	hexString := string(tx)
	log.Printf("the hex string is ", hexString)
	txo, err := hex.DecodeString(hexString)
	if err != nil {
		log.Printf("a major error occured", err)
	}
	log.Printf("the hex", txo)
	var transaction types.Transaction
	err = proto.Unmarshal(txo, &transaction)
	if err != nil {
		log.Printf("Error unmarshaling transaction data:", err)
		return 1
	}

	// Check the transaction type and handle accordingly
	switch transaction.Type {
	case types.TransactionType_NormalTransaction:
		normalData := &types.NormalTransactionData{}
		normalData = transaction.GetNormalData()
		if normalData == nil {
			log.Printf("Error unmarshaling normal transaction data:", err)
			return 1
		}
		return 0
		log.Printf("Normal Transaction Data:", normalData)
	case types.TransactionType_VerificationTransaction:
		verificationData := &types.VerificationTransactionData{}
		verificationData = transaction.GetVerificationData()
		if verificationData == nil {
			log.Printf("Error unmarshaling verification transaction data:", err)
			return 1
		}
		return 0
		log.Printf("Verification Transaction Data:", verificationData)
	case types.TransactionType_ResourceTransaction:
		resourceData := &types.ResourceTransactionData{}
		resourceData = transaction.GetResourceData()
		if resourceData == nil {
			log.Printf("Error unmarshaling resource transaction data:", err)
			return 1
		}
		return 0
		log.Printf("Resource Transaction Data:", resourceData)
	case types.TransactionType_NodeRegistrationTransaction:
		nodeRegistrationData := &types.NodeRegistrationTransactionData{}
		nodeRegistrationData = transaction.GetNodeRegistrationData()

		if nodeRegistrationData == nil {
			log.Printf("Error unmarshaling resource transaction data:", err)
			return 1
		}
		return 0
		log.Printf("Resource Transaction Data:", nodeRegistrationData)
	default:
		log.Printf("Unknown transaction type")
		return 1
	}
	return 1
}
