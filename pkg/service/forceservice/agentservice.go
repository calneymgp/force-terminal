// SPDX-License-Identifier: Apache-2.0
package forceservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/tsgen/tsgenmeta"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wcore"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

var ErrWriterConflict = errors.New("agent destination already has a writer")
var ErrOperationPending = errors.New("agent operation is still pending")
var ErrOperationSuperseded = errors.New("agent operation request was superseded")
var ErrDestinationUnresolved = errors.New("agent destination cannot be resolved on this host")

type ForceAgentInstanceInput struct {
	ProjectID      string `json:"projectid"`
	ProjectVersion int    `json:"projectversion"`
	ProfileID      string `json:"profileid"`
	ProfileVersion int    `json:"profileversion"`
	TabID          string `json:"tabid"`
	CreationKey    string `json:"creationkey"`
}

func (svc *ForceService) CreateAgentInstance_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "input"}}
}

func validAgentID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}

func (svc *ForceService) CreateAgentInstance(ctx context.Context, input ForceAgentInstanceInput) (*waveobj.ForceAgentInstance, waveobj.UpdatesRtnType, error) {
	if !validAgentID(input.ProjectID) || !validAgentID(input.ProfileID) || !validAgentID(input.TabID) || input.ProjectVersion < 1 || input.ProfileVersion < 1 {
		return nil, nil, fmt.Errorf("invalid agent creation identity or version")
	}
	if input.CreationKey != "" && !validAgentID(input.CreationKey) {
		return nil, nil, fmt.Errorf("invalid creationkey")
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	retry := false
	instance, err := wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*waveobj.ForceAgentInstance, error) {
		if input.CreationKey != "" {
			existing, err := wstore.DBGet[*waveobj.ForceAgentInstance](tx.Context(), input.CreationKey)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				if existing.ProjectID != input.ProjectID || existing.ProjectVersion != input.ProjectVersion || existing.ProfileID != input.ProfileID || existing.ProfileVersion != input.ProfileVersion || existing.TabID != input.TabID {
					return nil, ErrConflict
				}
				retry = true
				return existing, nil
			}
		}
		project, err := wstore.DBMustGet[*waveobj.ForceProject](tx.Context(), input.ProjectID)
		if err != nil {
			return nil, err
		}
		profile, err := wstore.DBMustGet[*waveobj.ForceAgentProfile](tx.Context(), input.ProfileID)
		if err != nil {
			return nil, err
		}
		if project.Version != input.ProjectVersion || profile.Version != input.ProfileVersion || project.Archived || profile.Archived {
			return nil, ErrConflict
		}
		tab, err := wstore.DBMustGet[*waveobj.Tab](tx.Context(), input.TabID)
		if err != nil {
			return nil, err
		}
		if !validAgentID(tab.LayoutState) {
			return nil, fmt.Errorf("agent tab has no valid layout")
		}
		if _, err := wstore.DBMustGet[*waveobj.LayoutState](tx.Context(), tab.LayoutState); err != nil {
			return nil, err
		}
		id := input.CreationKey
		if id == "" {
			id = uuid.NewString()
		}
		blockID := uuid.NewString()
		now := time.Now().UnixMilli()
		hash := sha256.Sum256([]byte(profile.SystemPrompt))
		instance := &waveobj.ForceAgentInstance{
			OID: id, ProjectID: project.OID, ProjectVersion: project.Version,
			ProfileID: profile.OID, ProfileVersion: profile.Version,
			TabID: tab.OID, BlockID: blockID,
			TitleSnapshot: profile.Title, IconSnapshot: profile.Icon,
			PromptSnapshot: profile.SystemPrompt, PromptHash: hex.EncodeToString(hash[:]),
			Adapter: profile.Adapter, AdapterVersion: "contract-v1",
			Connection: project.Connection, RootPath: project.RootPath,
			IdentityEvidence: "unavailable", Status: "prepared", CreatedAt: now, UpdatedAt: now,
		}
		if profile.Adapter == "claude-code" {
			instance.AdapterVersion = ClaudeAdapterVersion
			instance.ClaudeSessionID = uuid.NewString()
			instance.IdentityEvidence = "requested"
		}
		// An empty controller is a deliberate restore guard: ResyncController
		// returns before constructing or starting any runtime controller.
		block := &waveobj.Block{OID: blockID, ParentORef: waveobj.MakeORef(waveobj.OType_Tab, tab.OID).String(), Meta: waveobj.MetaMapType{
			waveobj.MetaKey_View: "term", waveobj.MetaKey_DisplayName: profile.Title,
			waveobj.MetaKey_Connection: project.Connection, waveobj.MetaKey_CmdCwd: project.RootPath,
			waveobj.MetaKey_CmdRunOnStart: false, "force:agentinstanceid": id,
		}}
		if err := wstore.DBInsert(tx.Context(), instance); err != nil {
			return nil, err
		}
		if err := wstore.DBInsert(tx.Context(), block); err != nil {
			return nil, err
		}
		tab.BlockIds = append(tab.BlockIds, blockID)
		if err := wstore.DBUpdate(tx.Context(), tab); err != nil {
			return nil, err
		}
		if err := wcore.QueueLayoutActionForTab(tx.Context(), tab.OID, waveobj.LayoutActionData{ActionType: wcore.LayoutActionDataType_Insert, BlockId: blockID, Focused: true}); err != nil {
			return nil, err
		}
		return instance, nil
	})
	if err != nil {
		return nil, nil, err
	}
	if retry {
		return instance, nil, nil
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return instance, updates, nil
}

func (svc *ForceService) ListAgentInstances_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "projectID"}}
}

func (svc *ForceService) ListAgentInstances(ctx context.Context, projectID string) ([]*waveobj.ForceAgentInstance, error) {
	if !validAgentID(projectID) {
		return nil, fmt.Errorf("invalid project id")
	}
	type agentRow struct {
		OId     string
		Version int
		Data    []byte
	}
	all, err := wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) ([]*waveobj.ForceAgentInstance, error) {
		var rows []agentRow
		tx.Select(&rows, "SELECT oid, version, data FROM db_forceagentinstance WHERE json_extract(data, '$.projectid') = ?", projectID)
		result := make([]*waveobj.ForceAgentInstance, 0, len(rows))
		for _, row := range rows {
			decoded, err := waveobj.FromJson(row.Data)
			if err != nil {
				return nil, err
			}
			v := decoded.(*waveobj.ForceAgentInstance)
			v.Version = row.Version
			result = append(result, v)
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]*waveobj.ForceAgentInstance, 0)
	for _, v := range all {
		if v.Connection == "" && (v.WriterLeaseKey != "" || v.Status == "running") {
			mu := agentOperationLock(v.OID)
			mu.Lock()
			current, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
			if err == nil && current.WriterLeaseKey != "" {
				reconcileCtx := waveobj.ContextWithUpdates(ctx)
				err = reconcileAgentLocal(reconcileCtx, current)
				if errors.Is(err, ErrOperationPending) {
					err = nil
				}
				if updates := waveobj.ContextGetUpdatesRtn(reconcileCtx); len(updates) > 0 {
					wps.Broker.SendUpdateEvents(updates)
				}
			} else if err == nil && current.Status == "running" && blockcontroller.ObserveForceAgentLocal(current.BlockID) == nil {
				err = markAgentUncertain(ctx, current.OID, current.AttemptID, current.Generation, "process_evidence_missing")
			}
			if err == nil {
				current, err = wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
			}
			mu.Unlock()
			if err != nil {
				return nil, err
			}
			v = current
		}
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt == result[j].CreatedAt {
			return result[i].OID < result[j].OID
		}
		return result[i].CreatedAt < result[j].CreatedAt
	})
	return result, nil
}

// operationReservation is persisted before a future launcher may run.
type operationReservation struct {
	Generation     int64
	PriorAttemptID string
}

type agentDestination struct {
	key           string
	hostIdentity  string
	canonicalRoot string
}

// canonicalDestination only resolves local roots. Remote roots require a
// host-side resolver in the future adapter; an unverified alias cannot lease.
func canonicalDestination(v *waveobj.ForceAgentInstance) (agentDestination, error) {
	if v.Connection != "" {
		return agentDestination{}, ErrDestinationUnresolved
	}
	if !filepath.IsAbs(v.RootPath) {
		return agentDestination{}, ErrDestinationUnresolved
	}
	path, err := filepath.EvalSymlinks(v.RootPath)
	if err != nil {
		return agentDestination{}, fmt.Errorf("resolve agent root: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return agentDestination{}, ErrDestinationUnresolved
	}
	// A .git directory or worktree marker file identifies the checkout even
	// when the configured root is nested. Only inspect the marker type; its
	// contents may refer to private Git metadata and are never needed here.
	for dir := path; ; dir = filepath.Dir(dir) {
		marker, err := os.Lstat(filepath.Join(dir, ".git"))
		if err == nil {
			if !marker.IsDir() && !marker.Mode().IsRegular() {
				return agentDestination{}, ErrDestinationUnresolved
			}
			path = dir
			break
		}
		if !os.IsNotExist(err) {
			return agentDestination{}, ErrDestinationUnresolved
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		return agentDestination{}, ErrDestinationUnresolved
	}
	identity := fmt.Sprintf("local:%s:%d", host, os.Getuid())
	root := filepath.Clean(path)
	sum := sha256.Sum256([]byte(identity + "\x00" + root))
	return agentDestination{key: hex.EncodeToString(sum[:]), hostIdentity: identity, canonicalRoot: root}, nil
}

func rootsOverlap(left, right string) bool {
	if left == right {
		return true
	}
	for _, pair := range [][2]string{{left, right}, {right, left}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func reserveOperation(ctx context.Context, instanceID, requestKey, intent string) (operationReservation, error) {
	if !validAgentID(instanceID) || !validAgentID(requestKey) || (intent != "start" && intent != "reconnect" && intent != "new-session") {
		return operationReservation{}, fmt.Errorf("invalid operation request")
	}
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (operationReservation, error) {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), instanceID)
		if err != nil {
			return operationReservation{}, err
		}
		if v.OperationRequestKey == requestKey {
			if v.OperationIntent != intent {
				return operationReservation{}, ErrConflict
			}
			return operationReservation{Generation: v.Generation, PriorAttemptID: v.PriorAttemptID}, nil
		}
		if receipt, err := agentRequestReceiptInTx(tx, instanceID, requestKey); err != nil {
			return operationReservation{}, err
		} else if receipt != nil {
			if receipt.Intent != intent {
				return operationReservation{}, ErrConflict
			}
			return operationReservation{}, ErrOperationSuperseded
		}
		if v.OperationPhase == "reserved" || v.OperationPhase == "launch_requested" || v.Status == "running" || v.Status == "disconnected" || v.Status == "uncertain" {
			return operationReservation{}, ErrOperationPending
		}
		if v.Adapter != "claude-code" {
			return operationReservation{}, fmt.Errorf("agent adapter execution is unavailable")
		}
		destination, err := canonicalDestination(v)
		if err != nil {
			return operationReservation{}, err
		}
		for _, root := range tx.SelectStrings("SELECT canonical_root FROM force_agent_writer_lease WHERE host_identity = ?", destination.hostIdentity) {
			if rootsOverlap(root, destination.canonicalRoot) {
				return operationReservation{}, ErrWriterConflict
			}
		}
		next := v.Generation + 1
		tx.Exec("INSERT INTO force_agent_writer_lease (destination_key, host_identity, canonical_root, instance_id, generation) VALUES (?, ?, ?, ?, ?)", destination.key, destination.hostIdentity, destination.canonicalRoot, v.OID, next)
		recordAgentRequestInTx(tx, instanceID, requestKey, intent, next, agentRequestOperation)
		prior := v.AttemptID
		v.PriorAttemptID, v.OperationRequestKey, v.OperationIntent, v.OperationPhase = prior, requestKey, intent, "reserved"
		if intent == "new-session" && v.CurrentSessionLaunched {
			v.PriorClaudeSessionID = v.ClaudeSessionID
			v.ClaudeSessionID = uuid.NewString()
			v.IdentityEvidence = "requested"
			v.CurrentSessionLaunched = false
		}
		v.Generation, v.WriterLeaseKey, v.Status, v.UpdatedAt = next, destination.key, "pending", time.Now().UnixMilli()
		if err := wstore.DBUpdate(tx.Context(), v); err != nil {
			return operationReservation{}, err
		}
		return operationReservation{Generation: next, PriorAttemptID: prior}, nil
	})
}

// releaseWriterLease requires source-backed confirmation that the execution
// attempt has exited. A lost connection cannot use this path.
func releaseWriterLease(ctx context.Context, instanceID string, generation int64, confirmedExit bool) error {
	if !confirmedExit {
		return fmt.Errorf("confirmed process exit required")
	}
	return wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), instanceID)
		if err != nil {
			return err
		}
		if v.Generation != generation || v.WriterLeaseKey == "" {
			return ErrConflict
		}
		key := v.WriterLeaseKey
		owner := tx.GetString("SELECT instance_id FROM force_agent_writer_lease WHERE destination_key = ? AND generation = ?", key, generation)
		if owner != v.OID {
			return ErrConflict
		}
		tx.Exec("DELETE FROM force_agent_writer_lease WHERE destination_key = ? AND instance_id = ? AND generation = ?", key, v.OID, generation)
		v.WriterLeaseKey, v.OperationPhase, v.Status, v.UpdatedAt = "", "", "exited", time.Now().UnixMilli()
		return wstore.DBUpdate(tx.Context(), v)
	})
}
