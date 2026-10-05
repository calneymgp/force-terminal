// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

package blockcontroller

import (
	"context"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/shellexec"
	"github.com/wavetermdev/waveterm/pkg/util/unixutil"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

const forceAgentInstanceMetaKey = "force:agentinstanceid"

type ForceAgentLocalLaunchSpec struct {
	InstanceID string
	AttemptID  string
	Generation int64
	Process    shellexec.LocalAgentProcSpec
}

// Evidence describes an observed process; it does not validate a provider's
// conversation identity. Attempt/generation correlate asynchronous exit events.
type ForceAgentProcessEvidence struct {
	AttemptID           string
	Generation          int64
	ProcessID           int
	ProcessStartTs      int64
	ProcessGroupID      int
	ProcessGroupStartTs int64
	SystemBootID        string
	ProcessGroupAbsent  bool
	StartedAt           int64
	FinishedAt          int64
	Status              string
	ExitCode            *int
	ErrorCode           string
}

type ForceAgentController struct {
	mu           sync.Mutex
	writeMu      sync.Mutex
	instanceID   string
	tabID        string
	blockID      string
	connection   string
	proc         *shellexec.ShellProc
	evidence     ForceAgentProcessEvidence
	version      int64
	appendOutput func(string, string, []byte) error
}

// Generic terminal restoration, including forced resync, is always inert.
func (c *ForceAgentController) Start(ctx context.Context, _ waveobj.MetaMapType, _ *waveobj.RuntimeOpts, _ bool) error {
	return ctx.Err()
}

func (c *ForceAgentController) GetConnName() string { return c.connection }

func (c *ForceAgentController) GetRuntimeStatus() *BlockControllerRuntimeStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	exitCode := 0
	if c.evidence.ExitCode != nil {
		exitCode = *c.evidence.ExitCode
	}
	return &BlockControllerRuntimeStatus{BlockId: c.blockID, Version: c.version, ShellProcStatus: c.evidence.Status, ShellProcConnName: c.connection, ShellProcExitCode: exitCode, ForceAgentErrorCode: c.evidence.ErrorCode}
}

func (c *ForceAgentController) publishStatus() {
	wps.Broker.Publish(wps.WaveEvent{Event: wps.Event_ControllerStatus, Scopes: []string{
		waveobj.MakeORef(waveobj.OType_Tab, c.tabID).String(), waveobj.MakeORef(waveobj.OType_Block, c.blockID).String(),
	}, Data: c.GetRuntimeStatus()})
}

func (c *ForceAgentController) Stop(graceful bool, _ string, _ bool) {
	c.mu.Lock()
	proc := c.proc
	evidence := c.evidence
	active := evidence.Status == Status_Running || evidence.Status == "uncertain"
	c.mu.Unlock()
	if proc == nil || !active {
		return
	}
	// Signal only the exact child handle. Never signal a numeric PGID from Stop:
	// Wait may reap the leader between an identity check and kill(-PGID), which
	// could direct the signal to a reused group. Closing the PTY after leader exit
	// delivers the normal hangup to foreground children; survivors stay uncertain.
	cmd := proc.Cmd.(shellexec.CmdWrap).Cmd
	if evidence.ProcessID != cmd.Process.Pid {
		return
	}
	if signal := unixutil.ParseSignal("TERM"); signal != nil {
		_ = cmd.Process.Signal(signal)
	} else {
		return
	}
	go func() {
		timer := time.NewTimer(DefaultGracefulKillWait)
		defer timer.Stop()
		select {
		case <-proc.DoneCh:
		case <-timer.C:
			_ = cmd.Process.Kill()
		}
	}()
	if graceful {
		<-proc.DoneCh
	}
}

func (c *ForceAgentController) SendInput(input *BlockInputUnion) error {
	if input == nil {
		return fmt.Errorf("invalid agent input")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.mu.Lock()
	proc, active := c.proc, c.evidence.Status == Status_Running
	c.mu.Unlock()
	if proc == nil || !active {
		return fmt.Errorf("agent is not connected; reconnect explicitly")
	}
	if input.TermSize != nil {
		if input.TermSize.Rows < 1 || input.TermSize.Cols < 1 || input.TermSize.Rows > 65535 || input.TermSize.Cols > 65535 {
			return fmt.Errorf("invalid agent terminal size")
		}
		if err := proc.Cmd.SetSize(input.TermSize.Cols, input.TermSize.Rows); err != nil {
			return fmt.Errorf("agent terminal resize failed")
		}
		if err := setTermSizeInDB(c.blockID, *input.TermSize); err != nil {
			return err
		}
	}
	if input.SigName != "" {
		return fmt.Errorf("agent signals are unavailable; use terminal interrupt")
	}
	if len(input.InputData) > 0 {
		if _, err := proc.Cmd.Write(input.InputData); err != nil {
			return fmt.Errorf("agent terminal input failed")
		}
	}
	return nil
}

func forceAgentBinding(ctx context.Context, tabID string, block *waveobj.Block) (*waveobj.ForceAgentInstance, error) {
	id := block.Meta.GetString(forceAgentInstanceMetaKey, "")
	if parsed, err := uuid.Parse(id); err != nil || parsed.String() != id {
		return nil, fmt.Errorf("invalid agent terminal binding")
	}
	v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, id)
	if err != nil || v.BlockID != block.OID || v.TabID != tabID || block.ParentORef != waveobj.MakeORef(waveobj.OType_Tab, tabID).String() {
		return nil, fmt.Errorf("agent terminal binding does not match its instance")
	}
	return v, nil
}

func forceAgentIDForBlock(ctx context.Context, blockID string) (string, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (string, error) {
		return tx.GetString("SELECT oid FROM db_forceagentinstance WHERE json_extract(data, '$.blockid') = ?", blockID), nil
	})
}

// Caller holds the block resync mutex. This guard precedes every destructive
// generic resync branch, so repeated terminal restart commands cannot kill an agent.
func resyncForceAgent(ctx context.Context, tabID string, block *waveobj.Block) (*ForceAgentController, error) {
	v, err := forceAgentBinding(ctx, tabID, block)
	if err != nil {
		return nil, err
	}
	if existing := getController(block.OID); existing != nil {
		c, ok := existing.(*ForceAgentController)
		if !ok || c.instanceID != v.OID || c.connection != v.Connection || c.tabID != tabID {
			return nil, fmt.Errorf("agent terminal has an incompatible controller")
		}
		return c, nil
	}
	c := &ForceAgentController{instanceID: v.OID, tabID: tabID, blockID: block.OID, connection: v.Connection,
		evidence: ForceAgentProcessEvidence{Status: Status_Init}, version: time.Now().UnixMilli(), appendOutput: HandleAppendBlockFile}
	registerController(block.OID, c)
	return c, nil
}

// LaunchForceAgentLocal is only called by the explicit, reserved service
// operation. ResyncController never reaches this function.
// Every error is returned before a process starts; after PTY start, the result
// contains process evidence and later failures go through the observer.
func LaunchForceAgentLocal(ctx context.Context, tabID, blockID string, spec ForceAgentLocalLaunchSpec, onEvidence func(ForceAgentProcessEvidence)) (ForceAgentProcessEvidence, error) {
	mu := getBlockResyncMutex(blockID)
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return ForceAgentProcessEvidence{}, err
	}
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, blockID)
	if err != nil {
		return ForceAgentProcessEvidence{}, err
	}
	c, err := resyncForceAgent(ctx, tabID, block)
	if err != nil {
		return ForceAgentProcessEvidence{}, err
	}
	if c.instanceID != spec.InstanceID || c.connection != "" || spec.Generation < 1 {
		return ForceAgentProcessEvidence{}, fmt.Errorf("invalid local agent launch binding")
	}
	if parsed, err := uuid.Parse(spec.AttemptID); err != nil || parsed.String() != spec.AttemptID {
		return ForceAgentProcessEvidence{}, fmt.Errorf("invalid agent attempt identity")
	}
	c.mu.Lock()
	if c.evidence.Status == Status_Running {
		c.mu.Unlock()
		return ForceAgentProcessEvidence{}, fmt.Errorf("agent process is already running")
	}
	if c.evidence.Status == "uncertain" {
		oldProc := c.proc
		oldGroupID := c.evidence.ProcessGroupID
		done := false
		if oldProc != nil {
			done, _ = oldProc.WaitNB()
		}
		state, groupErr := shellexec.InspectAgentProcessGroup(oldGroupID)
		if !done || groupErr != nil || state != shellexec.AgentProcessGroupAbsent {
			c.mu.Unlock()
			return ForceAgentProcessEvidence{}, fmt.Errorf("agent process group remains uncertain")
		}
		c.evidence.Status = Status_Done
	}
	fileCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	err = filestore.WFS.MakeFile(fileCtx, blockID, wavebase.BlockFile_Term, nil, wshrpc.FileOpts{MaxSize: DefaultTermMaxFileSize, Circular: true})
	if err != nil && err != fs.ErrExist {
		c.mu.Unlock()
		return ForceAgentProcessEvidence{}, fmt.Errorf("agent terminal storage is unavailable")
	}
	termSize := getTermSize(block)
	proc, err := shellexec.StartLocalAgentProc(spec.Process, termSize)
	if err != nil {
		c.mu.Unlock()
		return ForceAgentProcessEvidence{}, err
	}
	cmd := proc.Cmd.(shellexec.CmdWrap).Cmd
	var processStartTs int64
	if observed, err := process.NewProcess(int32(cmd.Process.Pid)); err == nil {
		processStartTs, _ = observed.CreateTime()
	}
	var systemBootID string
	if bootID, err := shellexec.CurrentAgentBootID(); err == nil {
		systemBootID = bootID
	}
	c.proc = proc
	c.evidence = ForceAgentProcessEvidence{AttemptID: spec.AttemptID, Generation: spec.Generation,
		ProcessID: cmd.Process.Pid, ProcessStartTs: processStartTs, ProcessGroupID: proc.AgentProcessGroupID,
		ProcessGroupStartTs: processStartTs, SystemBootID: systemBootID,
		StartedAt: time.Now().UnixMilli(), Status: Status_Running}
	c.version++
	evidence := c.evidence
	c.mu.Unlock()
	c.publishStatus()
	go c.monitor(proc, evidence, onEvidence)
	return evidence, nil
}

func ObserveForceAgentLocal(blockID string) *ForceAgentProcessEvidence {
	c, ok := getController(blockID).(*ForceAgentController)
	if !ok {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	evidence := c.evidence
	return &evidence
}

// Persistence failure must remain visible even when the object store cannot
// accept an error checkpoint. It never releases a lease or changes identity.
func ReportForceAgentPersistenceError(blockID, attemptID string, generation int64, code string) {
	c, ok := getController(blockID).(*ForceAgentController)
	if !ok {
		return
	}
	c.mu.Lock()
	if c.evidence.AttemptID != attemptID || c.evidence.Generation != generation {
		c.mu.Unlock()
		return
	}
	// The caller supplies a code, never an arbitrary error or prompt text.
	if code != "persistence_failed" {
		code = "persistence_failed"
	}
	c.evidence.ErrorCode = code
	c.version++
	c.mu.Unlock()
	c.publishStatus()
}

func (c *ForceAgentController) monitor(proc *shellexec.ShellProc, evidence ForceAgentProcessEvidence, onEvidence func(ForceAgentProcessEvidence)) {
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := proc.Cmd.Read(buf)
			if n > 0 {
				if err := c.appendOutput(c.blockID, wavebase.BlockFile_Term, buf[:n]); err != nil {
					c.mu.Lock()
					firstError := c.evidence.ErrorCode == ""
					c.evidence.ErrorCode = "terminal_storage_failed"
					c.version++
					current := c.evidence
					c.mu.Unlock()
					c.publishStatus()
					if firstError && onEvidence != nil {
						onEvidence(current)
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()
	waitErr := proc.Cmd.Wait()
	code := proc.Cmd.ExitCode()
	// Drain the PTY's final output before closing it. A retained descendant tty
	// cannot hold up shutdown or persistence indefinitely.
	select {
	case <-readDone:
	case <-time.After(250 * time.Millisecond):
	}
	proc.Cmd.Close()
	<-readDone
	groupAbsent := false
	groupError := "process_group_evidence_missing"
	if evidence.ProcessGroupID > 0 && evidence.ProcessGroupID == evidence.ProcessID &&
		evidence.ProcessGroupStartTs > 0 && evidence.ProcessGroupStartTs == evidence.ProcessStartTs {
		groupCheckDeadline := time.Now().Add(DefaultGracefulKillWait)
		for {
			state, err := shellexec.InspectAgentProcessGroup(evidence.ProcessGroupID)
			if err != nil {
				groupError = "process_group_check_failed"
			} else if state == shellexec.AgentProcessGroupAbsent {
				groupAbsent = true
				break
			} else {
				groupError = "process_group_members_remaining"
			}
			if time.Now().After(groupCheckDeadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if !groupAbsent {
		evidence.Status = "uncertain"
		evidence.ErrorCode = groupError
		c.mu.Lock()
		if c.evidence.AttemptID == evidence.AttemptID && c.evidence.Generation == evidence.Generation {
			if c.evidence.ErrorCode == "terminal_storage_failed" {
				evidence.ErrorCode = c.evidence.ErrorCode
			}
			c.evidence = evidence
			c.version++
		}
		c.mu.Unlock()
		c.publishStatus()
		if onEvidence != nil {
			onEvidence(evidence)
		}
		proc.SetWaitErrorAndSignalDone(waitErr)
		return
	}
	evidence.ProcessGroupAbsent = true
	evidence.Status, evidence.ExitCode, evidence.FinishedAt = Status_Done, &code, time.Now().UnixMilli()
	c.mu.Lock()
	if c.evidence.AttemptID == evidence.AttemptID {
		evidence.ErrorCode = c.evidence.ErrorCode
		c.evidence = evidence
		c.version++
	}
	c.mu.Unlock()
	if err := c.appendOutput(c.blockID, wavebase.BlockFile_Term, []byte(fmt.Sprintf("\r\n[agente encerrado: %d]\r\n", code))); err != nil {
		evidence.ErrorCode = "terminal_storage_failed"
		c.mu.Lock()
		c.evidence.ErrorCode = evidence.ErrorCode
		c.mu.Unlock()
	}
	c.publishStatus()
	if onEvidence != nil {
		onEvidence(evidence)
	}
	proc.SetWaitErrorAndSignalDone(waitErr)
}
