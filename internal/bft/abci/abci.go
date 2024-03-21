package verificationApp

import (
	"bytes"
	"context"
	"fmt"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/dgraph-io/badger/v3"
	"google.golang.org/protobuf/proto"
	"log"
	// "math/rand"
	"github.com/openmesh-network/core/internal/bft/types"
)

type VerificationApp struct {
	db           *badger.DB
	onGoingBlock *badger.Txn
}

var _ abcitypes.Application = (*VerificationApp)(nil)

func (app *VerificationApp) FinalizeBlock(_ context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	var txs = make([]*abcitypes.ExecTxResult, len(req.Txs))

	app.onGoingBlock = app.db.NewTransaction(true)
	for i, tx := range req.Txs {
		if code := app.isValid(tx); code != 0 {
			log.Printf("Error: invalid transaction index %v", i)
			txs[i] = &abcitypes.ExecTxResult{Code: code}
		} else {
			parts := bytes.SplitN(tx, []byte("="), 2)
			key, value := parts[0], parts[1]
			log.Printf("Adding key %s with value %s", key, value)

			if err := app.onGoingBlock.Set(key, value); err != nil {
				log.Panicf("Error writing to database, unable to execute tx: %v", err)
			}

			log.Printf("Successfully added key %s with value %s", key, value)

			txs[i] = &abcitypes.ExecTxResult{}
		}
	}

	// Run callback that stops collecting the code, and sends result as a transaction.
	{
		//
	}

	// Determine new order in block deterministically.
	{
		//
	}

	return &abcitypes.ResponseFinalizeBlock{
		TxResults: txs,
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

func (app *VerificationApp) ExecuteTransaction(tx []byte) uint32 {
	var transaction types.Transaction
	err := proto.Unmarshal(tx, &transaction)
	if err != nil {
		fmt.Println("Error unmarshaling transaction data:", err)
		return 1
	}

	switch transaction.Type {
	case types.TransactionType_NormalTransaction:
		normalData := &types.NormalTransactionData{}
		normalData = transaction.GetNormalData()
		if err != nil {
			fmt.Println("Error unmarshaling normal transaction data:", err)
			return 1
		}
		res := app.handleNormalTransaction(*normalData)
		if res != 0 {
			fmt.Println("Error occured in executing transaction")
			return 1
		}
		return 0
		fmt.Println("Normal Transaction Data:", normalData)
	case types.TransactionType_VerificationTransaction:
		verificationData := &types.VerificationTransactionData{}
		verificationData = transaction.GetVerificationData()
		res := app.handleVerificationTransaction(*verificationData)
		if res != 0 {
			fmt.Println("Error occured in executing transaction")
			return 1
		}
		return 0
		fmt.Println("Verification Transaction Data:", verificationData)
	case types.TransactionType_ResourceTransaction:
		resourceData := &types.ResourceTransactionData{}
		err := transaction.GetResourceData()
		if err != nil {
			fmt.Println("Error unmarshaling resource transaction data:", err)
			return 1
		}
		res := app.handleResourceTransaction(*resourceData)
		if res != 0 {
			fmt.Println("Error occured in executing transaction")
			return 1
		}
		return 0
		fmt.Println("Resource Transaction Data:", resourceData)
	default:
		fmt.Println("Unknown transaction type")
		return 1
	}
	return 0
}

func (app *VerificationApp) isValid(tx []byte) uint32 {
	// check format
	var transaction types.Transaction
	err := proto.Unmarshal(tx, &transaction)
	if err != nil {
		fmt.Println("Error unmarshaling transaction data:", err)
		return 1
	}

	// Check the transaction type and handle accordingly
	switch transaction.Type {
	case types.TransactionType_NormalTransaction:
		normalData := &types.NormalTransactionData{}
		normalData = transaction.GetNormalData()
		if err != nil {
			fmt.Println("Error unmarshaling normal transaction data:", err)
			return 1
		}
		return 0
		fmt.Println("Normal Transaction Data:", normalData)
	case types.TransactionType_VerificationTransaction:
		verificationData := &types.VerificationTransactionData{}
		verificationData = transaction.GetVerificationData()
		if err != nil {
			fmt.Println("Error unmarshaling verification transaction data:", err)
			return 1
		}
		return 0
		fmt.Println("Verification Transaction Data:", verificationData)
	case types.TransactionType_ResourceTransaction:
		resourceData := &types.ResourceTransactionData{}
		resourceData = transaction.GetResourceData()
		if err != nil {
			fmt.Println("Error unmarshaling resource transaction data:", err)
			return 1
		}
		return 0
		fmt.Println("Resource Transaction Data:", resourceData)
	default:
		fmt.Println("Unknown transaction type")
		return 1
	}
	return 1
}

func (app *VerificationApp) CheckTx(_ context.Context, check *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	code := app.isValid(check.Tx)
	return &abcitypes.ResponseCheckTx{Code: code}, nil
}

func NewVerificationApp(db *badger.DB) *VerificationApp {
	return &VerificationApp{db: db}
}

func (app *VerificationApp) InitChain(_ context.Context, chain *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	return &abcitypes.ResponseInitChain{}, nil
}

func (app *VerificationApp) PrepareProposal(_ context.Context, proposal *abcitypes.RequestPrepareProposal) (*abcitypes.ResponsePrepareProposal, error) {
	return &abcitypes.ResponsePrepareProposal{Txs: proposal.Txs}, nil
}
func (app *VerificationApp) ProcessProposal(_ context.Context, proposal *abcitypes.RequestProcessProposal) (*abcitypes.ResponseProcessProposal, error) {
	return &abcitypes.ResponseProcessProposal{Status: abcitypes.ResponseProcessProposal_ACCEPT}, nil
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

func (app *VerificationApp) handleNormalTransaction(tx types.NormalTransactionData) uint32 {
	return 0
}

func (app *VerificationApp) handleVerificationTransaction(tx types.VerificationTransactionData) uint32 {
	return 0
}

func (app *VerificationApp) handleResourceTransaction(tx types.ResourceTransactionData) uint32 {
	return 0
}
func (app *VerificationApp) Info(_ context.Context, info *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	return &abcitypes.ResponseInfo{}, nil
}
