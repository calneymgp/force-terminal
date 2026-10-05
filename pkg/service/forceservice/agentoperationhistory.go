package forceservice

import (
	"context"
	"time"

	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

const (
	agentRequestOperation     = "operation"
	agentRequestLiveReconnect = "live-reconnect"
)

type agentRequestReceipt struct {
	InstanceID string `db:"instance_id"`
	RequestKey string `db:"request_key"`
	Intent     string `db:"intent"`
	Generation int64  `db:"generation"`
	Kind       string `db:"kind"`
}

func agentRequestReceiptInTx(tx *wstore.TxWrap, instanceID, requestKey string) (*agentRequestReceipt, error) {
	var rows []agentRequestReceipt
	tx.Select(&rows, "SELECT instance_id, request_key, intent, generation, kind FROM force_agent_request_ledger WHERE instance_id = ? AND request_key = ?", instanceID, requestKey)
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func agentRequestReceiptFor(ctx context.Context, instanceID, requestKey string) (*agentRequestReceipt, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*agentRequestReceipt, error) {
		return agentRequestReceiptInTx(tx, instanceID, requestKey)
	})
}

func recordAgentRequestInTx(tx *wstore.TxWrap, instanceID, requestKey, intent string, generation int64, kind string) {
	tx.Exec("INSERT INTO force_agent_request_ledger (instance_id, request_key, intent, generation, kind, accepted_at) VALUES (?, ?, ?, ?, ?, ?)", instanceID, requestKey, intent, generation, kind, time.Now().UnixMilli())
}

func resultForSupersededAgentRequest(v *waveobj.ForceAgentInstance, receipt *agentRequestReceipt) *ForceAgentOperationResult {
	rtn := resultForAgent(v)
	rtn.Operation = ForceAgentOperation{Generation: receipt.Generation, RequestKey: receipt.RequestKey, Intent: receipt.Intent, Phase: "superseded", Status: "superseded"}
	return rtn
}

func acceptLiveAgentReconnect(ctx context.Context, instanceID, requestKey string, expected *waveobj.ForceAgentInstance) (*waveobj.ForceAgentInstance, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*waveobj.ForceAgentInstance, error) {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), instanceID)
		if err != nil {
			return nil, err
		}
		if v.Generation != expected.Generation || v.AttemptID != expected.AttemptID || v.BlockID != expected.BlockID {
			return nil, ErrConflict
		}
		if receipt, err := agentRequestReceiptInTx(tx, instanceID, requestKey); err != nil {
			return nil, err
		} else if receipt != nil {
			if receipt.Intent != "reconnect" {
				return nil, ErrConflict
			}
			return v, nil
		}
		observed := blockcontroller.ObserveForceAgentLocal(v.BlockID)
		if observed == nil || observed.AttemptID != v.AttemptID || observed.Generation != v.Generation || observed.Status != blockcontroller.Status_Running {
			return nil, ErrOperationPending
		}
		recordAgentRequestInTx(tx, instanceID, requestKey, "reconnect", v.Generation, agentRequestLiveReconnect)
		return v, nil
	})
}
