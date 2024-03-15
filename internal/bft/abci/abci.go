package verificationApp

import (
    "context"
    "database/sql"
    "log"
    "openmesh.network/openmesh-core/internal/bft/types"

    abcitypes "github.com/cometbft/cometbft/abci/types"
    "github.com/golang/protobuf/proto"
)

type KVStoreApplication struct {
    db           *sql.DB
    onGoingBlock *sql.Tx
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication(db *sql.DB) *KVStoreApplication {
    return &KVStoreApplication{db: db}
}
func (app *KVStoreApplication) Info(_ context.Context, info *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
    return &abcitypes.ResponseInfo{}, nil
}

func (app *KVStoreApplication) Query(_ context.Context, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
    resp := abcitypes.ResponseQuery{Key: req.Data}

    // Start a new transaction
    tx, err := app.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // Execute a SELECT query to retrieve the value corresponding to the key
    row := tx.QueryRow("SELECT tablename FROM nodemeta WHERE datasource = $1", req.Data)

    // Scan the value from the row
    var value []byte
    switch err := row.Scan(&value); err {
    case nil:
        resp.Log = "exists"
        resp.Value = value
    case sql.ErrNoRows:
        resp.Log = "key does not exist"
    default:
        log.Panicf("Error reading database: %v", err)
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

    // Start a new transaction
    temptx, err := app.db.Begin()
    if err != nil {
        log.Panicf("Error starting transaction: %v", err)
    }
    defer temptx.Rollback()

    for i, tx := range req.Txs {
        // Assuming the format of the transaction is "key=value"
        var transaction types.Transaction
        if err := proto.Unmarshal(tx, &transaction); err != nil {
            log.Printf("Failed to unmarshal transaction: %v", err)
            txs[i] = &abcitypes.ExecTxResult{Code: 1} // You may need to define error codes based on your application logic
            continue
        }

        // Check if the key-value pair is valid (optional)
        if app.isValid(tx) != 0 {
            log.Printf("Error: invalid transaction index %v", i)
            txs[i] = &abcitypes.ExecTxResult{Code: 1} // You may need to define error codes based on your application logic
            continue
        }

        if transaction.GetType() == types.TransactionType_VerificationTransaction {
            log.Printf("Verification transaction foundd the tx")
            _, err := temptx.Exec("INSERT INTO nodemeta (datasource, tablename) VALUES ($1, $2)", transaction.GetSignature(), transaction.GetSignature())
            if err != nil {
                log.Panicf("Error writing to database, unable to execute tx: %v", err)
            }

            log.Printf("Successfully added key %s with value %s", transaction.GetSignature(), transaction.GetSignature())
        }

        // Execute the SQL INSERT query

        txs[i] = &abcitypes.ExecTxResult{} // Mark transaction as successful
    }

    // Commit the transaction
    if err := temptx.Commit(); err != nil {
        log.Panicf("Error committing transaction: %v", err)
    }

    return &abcitypes.ResponseFinalizeBlock{
        TxResults: txs,
    }, nil
}

func (app KVStoreApplication) Commit(_ context.Context, commit *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
    // Since PostgreSQL automatically commits transactions, we don't need to manually commit here
    // We can just return an empty ResponseCommit
    return &abcitypes.ResponseCommit{}, nil
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
    log.Panicf("Extend vote is called")
    return &abcitypes.ResponseExtendVote{VoteExtension: []byte("ok")}, nil

}

func (app *KVStoreApplication) VerifyVoteExtension(_ context.Context, verify *abcitypes.RequestVerifyVoteExtension) (*abcitypes.ResponseVerifyVoteExtension, error) {
    log.Panicf("Extend vote is called")
    return &abcitypes.ResponseVerifyVoteExtension{Status: abcitypes.ResponseVerifyVoteExtension_ACCEPT}, nil
}

func (app *KVStoreApplication) isValid(tx []byte) uint32 {
    // check format
    var transaction types.Transaction
    if err := proto.Unmarshal(tx, &transaction); err != nil {
        log.Printf("Failed to unmarshal transaction: %v", err)
        return 1
    }

    return 0
}
