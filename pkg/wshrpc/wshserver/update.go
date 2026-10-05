// SPDX-License-Identifier: Apache-2.0
package wshserver

import (
	"context"
	"fmt"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/secretstore"
	"github.com/wavetermdev/waveterm/pkg/updateguard"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	"sort"
	"time"
)

// Only Electron coordinates an application update. A CLI renderer cannot
// provide authoritative cross-window readiness or trigger this flush barrier.
func requireUpdateCoordinator(ctx context.Context) error {
	if wshutil.GetRpcSourceFromContext(ctx) != "electron" {
		return fmt.Errorf("application update preparation requires Electron coordinator")
	}
	return nil
}

func (ws *WshServer) FlushForUpdateCommand(ctx context.Context, data wshrpc.CommandFlushForUpdateData) error {
	if err := requireUpdateCoordinator(ctx); err != nil {
		return err
	}
	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return flushUpdateStores(flushCtx, filestore.WFS.FlushForUpdate, secretstore.FlushForUpdate)
}

func flushUpdateStores(ctx context.Context, flushFiles, flushSecrets func(context.Context) error) error {
	if err := flushFiles(ctx); err != nil {
		return err
	}
	return flushSecrets(ctx)
}

func (ws *WshServer) GetUpdateBlockersCommand(ctx context.Context, data wshrpc.CommandGetUpdateBlockersData) ([]string, error) {
	if err := requireUpdateCoordinator(ctx); err != nil {
		return nil, err
	}
	return collectUpdateBlockers(ctx, data)
}

func collectUpdateBlockers(ctx context.Context, data wshrpc.CommandGetUpdateBlockersData) ([]string, error) {
	controllers := make([]updateguard.Controller, 0)
	for _, status := range blockcontroller.GetAllBlockControllerRuntimeStatuses() {
		block, err := wstore.DBMustGet[*waveobj.Block](ctx, status.BlockId)
		if err != nil {
			return nil, fmt.Errorf("cannot verify terminal %s: %w", status.BlockId, err)
		}
		shellState := ""
		if info := wstore.GetRTInfo(waveobj.MakeORef(waveobj.OType_Block, status.BlockId)); info != nil {
			shellState = info.ShellState
		}
		controllers = append(controllers, updateguard.Controller{BlockID: status.BlockId, Kind: block.Meta.GetString(waveobj.MetaKey_Controller, ""), State: status.ShellProcStatus, ShellState: shellState})
	}
	storedJobs, err := wstore.DBGetAllObjsByType[*waveobj.Job](ctx, waveobj.OType_Job)
	if err != nil {
		return nil, fmt.Errorf("cannot verify running jobs: %w", err)
	}
	jobs := make([]updateguard.Job, 0, len(storedJobs))
	for _, job := range storedJobs {
		jobs = append(jobs, updateguard.Job{ID: job.OID, BlockID: job.AttachedBlockId, Kind: job.JobKind, State: job.JobManagerStatus, PID: job.CmdPid, ExitTS: job.CmdExitTs})
	}
	reasons := updateguard.Blockers(controllers, jobs, data.VerifiedIdleBlocks)
	storedAgents, err := wstore.DBGetAllObjsByType[*waveobj.ForceAgentInstance](ctx, waveobj.OType_ForceAgentInstance)
	if err != nil {
		return nil, fmt.Errorf("cannot verify persisted agents: %w", err)
	}
	agents := make([]updateguard.Agent, 0, len(storedAgents))
	for _, agent := range storedAgents {
		agents = append(agents, updateguard.Agent{BlockID: agent.BlockID, Title: agent.TitleSnapshot,
			State: agent.Status, Phase: agent.OperationPhase, WriterLeaseKey: agent.WriterLeaseKey})
	}
	reasons = append(reasons, updateguard.AgentBlockers(agents)...)
	sort.Strings(reasons)
	return reasons, nil
}
