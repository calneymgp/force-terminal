// SPDX-License-Identifier: Apache-2.0
package forceservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/shellexec"
	"github.com/wavetermdev/waveterm/pkg/tsgen/tsgenmeta"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

var ErrAgentUncertain = errors.New("agent process state is uncertain")
var ErrAgentUnavailable = errors.New("agent operation unavailable")

// This override is package-private and only set by controlled tests. Public
// RPC inputs cannot choose a process path or command line.
var agentCLIPathForTest string

var agentOperationLocks sync.Map

// One private prompt belongs to one owned attempt. Keep it while any member of
// the PTY group may still be active, then remove it on confirmed reconciliation
// even if the controller's original exit callback already returned uncertain.
var agentPromptCleanups sync.Map

func cleanupAgentPrompt(attemptID string) {
	if cleanup, ok := agentPromptCleanups.LoadAndDelete(attemptID); ok {
		_ = cleanup.(func() error)()
	}
}

func releaseAgentWriterAfterConfirmedExit(ctx context.Context, v *waveobj.ForceAgentInstance) error {
	if err := releaseWriterLease(ctx, v.OID, v.Generation, true); err != nil {
		return err
	}
	cleanupAgentPrompt(v.AttemptID)
	return nil
}

func agentOperationLock(id string) *sync.Mutex {
	lock, _ := agentOperationLocks.LoadOrStore(id, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

type ForceAgentOperation struct {
	Generation int64  `json:"generation"`
	RequestKey string `json:"requestkey"`
	Intent     string `json:"intent"`
	Phase      string `json:"phase"`
	Status     string `json:"status"`
	AttemptID  string `json:"attemptid,omitempty"`
}

type ForceAgentOperationResult struct {
	Instance        *waveobj.ForceAgentInstance `json:"instance"`
	Operation       ForceAgentOperation         `json:"operation"`
	TerminalBlockID string                      `json:"terminalblockid"`
}

func resultForAgent(v *waveobj.ForceAgentInstance) *ForceAgentOperationResult {
	return &ForceAgentOperationResult{Instance: v, TerminalBlockID: v.BlockID, Operation: ForceAgentOperation{
		Generation: v.Generation, RequestKey: v.OperationRequestKey, Intent: v.OperationIntent,
		Phase: v.OperationPhase, Status: v.Status, AttemptID: v.AttemptID,
	}}
}

func (svc *ForceService) StartAgent_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "instanceID", "requestKey"}}
}
func (svc *ForceService) ReconnectAgent_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "instanceID", "requestKey"}}
}
func (svc *ForceService) StartNewSession_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "instanceID", "requestKey"}}
}

func (svc *ForceService) StartAgent(ctx context.Context, instanceID, requestKey string) (*ForceAgentOperationResult, waveobj.UpdatesRtnType, error) {
	return svc.runAgentOperation(ctx, instanceID, requestKey, "start")
}
func (svc *ForceService) ReconnectAgent(ctx context.Context, instanceID, requestKey string) (*ForceAgentOperationResult, waveobj.UpdatesRtnType, error) {
	return svc.runAgentOperation(ctx, instanceID, requestKey, "reconnect")
}
func (svc *ForceService) StartNewSession(ctx context.Context, instanceID, requestKey string) (*ForceAgentOperationResult, waveobj.UpdatesRtnType, error) {
	return svc.runAgentOperation(ctx, instanceID, requestKey, "new-session")
}

func currentAgentContext() (string, string, error) {
	u, err := user.Current()
	if err != nil || u.Uid == "" {
		return "", "", ErrDestinationUnresolved
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", ErrDestinationUnresolved
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "", "", ErrDestinationUnresolved
	}
	material := host + "\x00" + u.Uid + "\x00" + home + "\x00" + os.Getenv("HOME") + "\x00" + os.Getenv("CLAUDE_CONFIG_DIR")
	sum := sha256.Sum256([]byte(material))
	return u.Uid, hex.EncodeToString(sum[:]), nil
}

func agentExecutionRoot(v *waveobj.ForceAgentInstance) (string, error) {
	if v.Connection != "" {
		return "", ErrDestinationUnresolved
	}
	root, err := filepath.EvalSymlinks(v.RootPath)
	if err != nil {
		return "", ErrDestinationUnresolved
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", ErrDestinationUnresolved
	}
	return filepath.Clean(root), nil
}

func (svc *ForceService) runAgentOperation(ctx context.Context, instanceID, requestKey, intent string) (*ForceAgentOperationResult, waveobj.UpdatesRtnType, error) {
	if !validAgentID(instanceID) || !validAgentID(requestKey) {
		return nil, nil, fmt.Errorf("invalid agent operation identity")
	}
	mu := agentOperationLock(instanceID)
	mu.Lock()
	defer mu.Unlock()
	v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, instanceID)
	if err != nil {
		return nil, nil, err
	}
	if v.OperationRequestKey == requestKey {
		if v.OperationIntent != intent {
			return nil, nil, ErrConflict
		}
		return resultForAgent(v), nil, nil
	}
	receipt, err := agentRequestReceiptFor(ctx, instanceID, requestKey)
	if err != nil {
		return nil, nil, err
	}
	if receipt != nil {
		if receipt.Intent != intent {
			return nil, nil, ErrConflict
		}
		if receipt.Kind == agentRequestLiveReconnect && receipt.Generation == v.Generation {
			if observed := blockcontroller.ObserveForceAgentLocal(v.BlockID); observed != nil && observed.AttemptID == v.AttemptID && observed.Generation == v.Generation && observed.Status == blockcontroller.Status_Running {
				rtn := resultForAgent(v)
				rtn.Operation = ForceAgentOperation{Generation: receipt.Generation, RequestKey: requestKey, Intent: intent, Phase: "reattached", Status: "running", AttemptID: v.AttemptID}
				return rtn, nil, nil
			}
		}
		return resultForSupersededAgentRequest(v, receipt), nil, nil
	}
	if v.Adapter != "claude-code" {
		return nil, nil, fmt.Errorf("%w: adapter execution unavailable", ErrAgentUnavailable)
	}
	if v.Connection != "" {
		return nil, nil, fmt.Errorf("%w: remote execution unavailable", ErrAgentUnavailable)
	}
	if intent == "reconnect" {
		if observed := blockcontroller.ObserveForceAgentLocal(v.BlockID); observed != nil && observed.AttemptID == v.AttemptID && observed.Generation == v.Generation && observed.Status == blockcontroller.Status_Running {
			accepted, err := acceptLiveAgentReconnect(ctx, instanceID, requestKey, v)
			if err != nil {
				return nil, nil, err
			}
			rtn := resultForAgent(accepted)
			rtn.Operation = ForceAgentOperation{Generation: accepted.Generation, RequestKey: requestKey, Intent: intent, Phase: "reattached", Status: "running", AttemptID: accepted.AttemptID}
			return rtn, nil, nil
		}
	}
	if v.AdapterVersion != "contract-v1" && v.AdapterVersion != ClaudeAdapterVersion {
		return nil, nil, fmt.Errorf("%w: adapter version mismatch", ErrAgentUnavailable)
	}
	if err := reconcileAgentLocal(ctx, v); err != nil {
		return nil, nil, err
	}
	v, err = wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, instanceID)
	if err != nil {
		return nil, nil, err
	}
	if v.Status == "running" || v.Status == "uncertain" || v.OperationPhase == "launch_requested" {
		return nil, nil, ErrAgentUncertain
	}
	if intent == "start" && v.CurrentSessionLaunched {
		return nil, nil, fmt.Errorf("%w: use reconnect for prior conversation", ErrAgentUnavailable)
	}
	if intent == "reconnect" && !v.CurrentSessionLaunched {
		return nil, nil, fmt.Errorf("%w: no prior execution", ErrAgentUnavailable)
	}
	if intent == "new-session" && !v.WasLaunched {
		return nil, nil, fmt.Errorf("%w: no prior execution", ErrAgentUnavailable)
	}
	if v.WriterLeaseKey != "" {
		return nil, nil, ErrOperationPending
	}
	root, err := agentExecutionRoot(v)
	if err != nil {
		return nil, nil, err
	}
	dest, err := canonicalDestination(v)
	if err != nil {
		return nil, nil, err
	}
	userID, contextHash, err := currentAgentContext()
	if err != nil {
		return nil, nil, err
	}
	if v.WasLaunched && (v.ExecutionRoot == "" || v.ExecutionLeaseRoot == "" || v.CLIHistoryContext == "") {
		return nil, nil, fmt.Errorf("%w: saved execution context missing", ErrDestinationUnresolved)
	}
	if v.ExecutionRoot != "" && (v.ExecutionRoot != root || v.ExecutionLeaseRoot != dest.canonicalRoot || v.CLIHistoryContext != contextHash) {
		return nil, nil, fmt.Errorf("%w: execution destination or history changed", ErrDestinationUnresolved)
	}
	mode := ClaudeLaunchStart
	if intent == "reconnect" {
		mode = ClaudeLaunchResume
	}
	if _, err := reserveOperation(ctx, instanceID, requestKey, intent); err != nil {
		return nil, nil, err
	}
	attemptID := uuid.NewString()
	ctx = waveobj.ContextWithUpdates(ctx)
	v, err = wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*waveobj.ForceAgentInstance, error) {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), instanceID)
		if err != nil {
			return nil, err
		}
		if v.OperationRequestKey != requestKey || v.OperationPhase != "reserved" {
			return nil, ErrConflict
		}
		v.AttemptID, v.OperationPhase, v.ExecutionRoot, v.ExecutionLeaseRoot, v.CLIHistoryContext = attemptID, "launch_requested", root, dest.canonicalRoot, contextHash
		v.LocalPID, v.LocalProcessStartTs, v.AttemptStartedAt, v.AttemptFinishedAt, v.AttemptExitCode = 0, 0, 0, 0, nil
		v.ErrorCode, v.UpdatedAt = "", time.Now().UnixMilli()
		if err := wstore.DBUpdate(tx.Context(), v); err != nil {
			return nil, err
		}
		return v, nil
	})
	if err != nil {
		return nil, nil, err
	}
	// The checkpoint above is durable before private staging or process spawn.
	prepared, err := prepareClaudeLaunch(ctx, ClaudeLaunchSpec{SessionID: v.ClaudeSessionID, PromptSnapshot: v.PromptSnapshot, PromptHash: v.PromptHash,
		Connection: v.Connection, UserContext: userID, Cwd: root, Mode: mode, CLIPath: agentCLIPathForTest})
	if err != nil {
		return failedKnownNoSpawn(ctx, v.OID, attemptID, v.Generation, "prepare_failed", err)
	}
	launch := blockcontroller.ForceAgentLocalLaunchSpec{InstanceID: v.OID, AttemptID: attemptID, Generation: v.Generation,
		Process: shellexec.LocalAgentProcSpec{Executable: prepared.Argv[0], Args: prepared.Argv[1:], Cwd: root}}
	agentPromptCleanups.Store(attemptID, prepared.Cleanup)
	onEvidence := func(e blockcontroller.ForceAgentProcessEvidence) {
		if e.Status == blockcontroller.Status_Done {
			defer cleanupAgentPrompt(e.AttemptID)
		}
		mu.Lock()
		defer mu.Unlock()
		if err := persistAgentEvidence(context.Background(), v.OID, e); err != nil {
			_ = markAgentUncertain(context.Background(), v.OID, attemptID, v.Generation, "persistence_failed")
			blockcontroller.ReportForceAgentPersistenceError(v.BlockID, attemptID, v.Generation, "persistence_failed")
		}
	}
	evidence, err := blockcontroller.LaunchForceAgentLocal(ctx, v.TabID, v.BlockID, launch, onEvidence)
	if err != nil {
		cleanupAgentPrompt(attemptID)
		// LaunchForceAgentLocal currently reports every error before successful
		// PTY spawn; its successful return is the process evidence boundary.
		return failedKnownNoSpawn(ctx, v.OID, attemptID, v.Generation, "launch_failed", err)
	}
	if err := persistAgentEvidence(ctx, v.OID, evidence); err != nil {
		_ = markAgentUncertain(ctx, v.OID, attemptID, v.Generation, "persistence_failed")
		blockcontroller.ReportForceAgentPersistenceError(v.BlockID, attemptID, v.Generation, "persistence_failed")
		return nil, nil, err
	}
	updated, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		return nil, nil, err
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return resultForAgent(updated), updates, nil
}

func failedKnownNoSpawn(ctx context.Context, id, attempt string, generation int64, code string, cause error) (*ForceAgentOperationResult, waveobj.UpdatesRtnType, error) {
	updates, err := abortUnlaunchedAgent(ctx, id, attempt, generation, code)
	if err != nil {
		return nil, nil, errors.Join(cause, err)
	}
	v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return resultForAgent(v), updates, nil
}

func abortUnlaunchedAgent(ctx context.Context, id, attempt string, generation int64, code string) (waveobj.UpdatesRtnType, error) {
	ctx = waveobj.ContextWithUpdates(ctx)
	err := wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), id)
		if err != nil {
			return err
		}
		if v.AttemptID != attempt || v.Generation != generation || v.AttemptStartedAt != 0 || v.LocalPID != 0 {
			return ErrConflict
		}
		tx.Exec("DELETE FROM force_agent_writer_lease WHERE destination_key = ? AND instance_id = ? AND generation = ?", v.WriterLeaseKey, id, generation)
		v.WriterLeaseKey, v.OperationPhase, v.ErrorCode, v.UpdatedAt = "", "prepare_failed", code, time.Now().UnixMilli()
		if v.OperationIntent == "reconnect" {
			v.Status = "resume_failed"
		} else {
			v.Status = "prepared"
		}
		return wstore.DBUpdate(tx.Context(), v)
	})
	if err != nil {
		return nil, err
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return updates, nil
}

func markAgentUncertain(ctx context.Context, id, attempt string, generation int64, code string) error {
	ctx = waveobj.ContextWithUpdates(ctx)
	err := wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), id)
		if err != nil {
			return err
		}
		if v.AttemptID != attempt || v.Generation != generation {
			return ErrConflict
		}
		if v.Status == "uncertain" && v.ErrorCode == code {
			return nil
		}
		v.Status, v.ErrorCode, v.UpdatedAt = "uncertain", code, time.Now().UnixMilli()
		return wstore.DBUpdate(tx.Context(), v)
	})
	if err == nil {
		if updates := waveobj.ContextGetUpdatesRtn(ctx); len(updates) > 0 {
			wps.Broker.SendUpdateEvents(updates)
		}
	}
	return err
}

func persistAgentEvidence(ctx context.Context, id string, e blockcontroller.ForceAgentProcessEvidence) error {
	ctx = waveobj.ContextWithUpdates(ctx)
	err := wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), id)
		if err != nil {
			return err
		}
		if v.Generation != e.Generation || v.AttemptID != e.AttemptID {
			return ErrConflict
		}
		if e.ProcessID > 0 {
			v.LocalPID = e.ProcessID
		}
		if e.ProcessStartTs > 0 {
			v.LocalProcessStartTs = e.ProcessStartTs
		}
		if e.ProcessGroupID > 0 {
			v.LocalProcessGroupID = e.ProcessGroupID
		}
		if e.ProcessGroupStartTs > 0 {
			v.LocalProcessGroupStartTs = e.ProcessGroupStartTs
		}
		if e.SystemBootID != "" {
			v.LocalBootID = e.SystemBootID
		}
		if e.StartedAt > 0 {
			v.AttemptStartedAt = e.StartedAt
			v.WasLaunched = true
			v.CurrentSessionLaunched = true
		}
		if e.ErrorCode != "" {
			v.ErrorCode = e.ErrorCode
		}
		if e.Status == "uncertain" {
			v.Status = "uncertain"
			if e.ErrorCode != "" {
				v.ErrorCode = e.ErrorCode
			}
		} else if e.Status == blockcontroller.Status_Done {
			if !e.ProcessGroupAbsent || e.ProcessGroupID < 1 || e.ProcessGroupID != e.ProcessID ||
				e.ProcessGroupStartTs < 1 || e.ProcessGroupStartTs != e.ProcessStartTs {
				v.Status, v.ErrorCode = "uncertain", "process_group_exit_unproven"
				v.UpdatedAt = time.Now().UnixMilli()
				return wstore.DBUpdate(tx.Context(), v)
			}
			v.AttemptFinishedAt, v.AttemptExitCode = e.FinishedAt, e.ExitCode
			v.Status, v.OperationPhase = "exited", ""
			if e.ExitCode != nil && *e.ExitCode != 0 {
				if v.OperationIntent == "reconnect" {
					v.Status, v.ErrorCode = "resume_failed", "resume_exit_nonzero"
				} else {
					v.ErrorCode = "process_exit_nonzero"
				}
			}
			if v.WriterLeaseKey != "" {
				owner := tx.GetString("SELECT instance_id FROM force_agent_writer_lease WHERE destination_key = ? AND generation = ?", v.WriterLeaseKey, v.Generation)
				if owner != v.OID {
					return ErrConflict
				}
				tx.Exec("DELETE FROM force_agent_writer_lease WHERE destination_key = ? AND instance_id = ? AND generation = ?", v.WriterLeaseKey, v.OID, v.Generation)
				v.WriterLeaseKey = ""
			}
		} else if e.Status == blockcontroller.Status_Running {
			v.Status, v.OperationPhase = "running", "running"
		}
		v.UpdatedAt = time.Now().UnixMilli()
		return wstore.DBUpdate(tx.Context(), v)
	})
	if err == nil {
		wps.Broker.SendUpdateEvents(waveobj.ContextGetUpdatesRtn(ctx))
	}
	return err
}

func reconcileAgentLocal(ctx context.Context, v *waveobj.ForceAgentInstance) error {
	if v.WriterLeaseKey == "" {
		return nil
	}
	if v.OperationPhase == "reserved" {
		return releaseAgentWriterAfterConfirmedExit(ctx, v)
	}
	if observed := blockcontroller.ObserveForceAgentLocal(v.BlockID); observed != nil && observed.AttemptID == v.AttemptID && observed.Generation == v.Generation {
		if observed.Status == blockcontroller.Status_Done {
			err := persistAgentEvidence(ctx, v.OID, *observed)
			if err == nil && observed.ProcessGroupAbsent {
				cleanupAgentPrompt(observed.AttemptID)
			}
			return err
		}
		if observed.Status == blockcontroller.Status_Running {
			return ErrOperationPending
		}
	}
	if v.LocalPID < 1 || v.LocalProcessStartTs < 1 || v.LocalProcessGroupID < 1 ||
		v.LocalProcessGroupID != v.LocalPID || v.LocalProcessGroupStartTs < 1 ||
		v.LocalProcessGroupStartTs != v.LocalProcessStartTs {
		return markAgentUncertain(ctx, v.OID, v.AttemptID, v.Generation, "process_group_evidence_missing")
	}
	if v.LocalBootID != "" {
		currentBootID, err := shellexec.CurrentAgentBootID()
		if err == nil && currentBootID != "" && currentBootID != v.LocalBootID {
			// A reboot destroys every local process group from the previous boot.
			return releaseAgentWriterAfterConfirmedExit(ctx, v)
		}
	}
	groupState, err := shellexec.InspectAgentProcessGroup(v.LocalProcessGroupID)
	if err != nil {
		return markAgentUncertain(ctx, v.OID, v.AttemptID, v.Generation, "process_group_check_failed")
	}
	if groupState == shellexec.AgentProcessGroupPresent {
		return markAgentUncertain(ctx, v.OID, v.AttemptID, v.Generation, "live_process_group_without_terminal")
	}
	if groupState != shellexec.AgentProcessGroupAbsent {
		return markAgentUncertain(ctx, v.OID, v.AttemptID, v.Generation, "process_group_check_failed")
	}
	// ESRCH confirms that no process remains in the recorded group. Its
	// conversation UUID remains the only resume target.
	return releaseAgentWriterAfterConfirmedExit(ctx, v)
}
