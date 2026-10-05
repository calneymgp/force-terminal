// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

package blockcontroller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/shellexec"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func forceControllerFixture(t *testing.T) (context.Context, *waveobj.ForceAgentInstance) {
	t.Helper()
	dir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	wavebase.DataHome_VarCache = canonical
	if err := os.MkdirAll(filepath.Join(canonical, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	if err := filestore.InitFilestoreForTesting(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	v := &waveobj.ForceAgentInstance{OID: uuid.NewString(), TabID: uuid.NewString(), BlockID: uuid.NewString(), RootPath: canonical, Status: "prepared", Meta: waveobj.MetaMapType{}}
	if err := wstore.DBInsert(ctx, v); err != nil {
		t.Fatal(err)
	}
	block := &waveobj.Block{OID: v.BlockID, ParentORef: waveobj.MakeORef(waveobj.OType_Tab, v.TabID).String(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term", "force:agentinstanceid": v.OID}}
	if err := wstore.DBInsert(ctx, block); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DestroyBlockController(v.BlockID) })
	return ctx, v
}

func TestForceAgentRestoreAndForceResyncNeverLaunchOrRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("first agent target is macOS arm64")
	}
	ctx, v := forceControllerFixture(t)
	trace := filepath.Join(v.RootPath, "launch-count")
	executable := filepath.Join(v.RootPath, "fake agent")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf x >> \"$1\"\nprintf 'AGENT_READY\\n'\nIFS= read -r line\nprintf 'AGENT_INPUT:%s\\n' \"$line\"\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, force := range []bool{false, true} {
		if err := ResyncController(ctx, v.TabID, v.BlockID, nil, force); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatal("restoring an inert instance launched a process")
	}
	exited := make(chan ForceAgentProcessEvidence, 1)
	spec := ForceAgentLocalLaunchSpec{InstanceID: v.OID, AttemptID: uuid.NewString(), Generation: 1, Process: shellexec.LocalAgentProcSpec{Executable: executable, Args: []string{trace}, Cwd: v.RootPath}}
	evidence, err := LaunchForceAgentLocal(ctx, v.TabID, v.BlockID, spec, func(e ForceAgentProcessEvidence) { exited <- e })
	if err != nil {
		t.Fatal(err)
	}
	if evidence.ProcessID < 1 || evidence.Status != Status_Running || evidence.AttemptID != spec.AttemptID {
		t.Fatalf("launch returned no process evidence: %#v", evidence)
	}
	if err := DestroyBlockControllerFromClient(ctx, v.BlockID); err == nil {
		t.Fatal("generic destroy accepted a live Force agent")
	}
	if observed := ObserveForceAgentLocal(v.BlockID); observed == nil || observed.ProcessID != evidence.ProcessID || observed.Status != Status_Running {
		t.Fatal("generic destroy changed the live Force process")
	}
	if _, err := LaunchForceAgentLocal(ctx, v.TabID, v.BlockID, spec, nil); err == nil {
		t.Fatal("second launch was accepted while the agent was alive")
	}
	if err := ResyncController(ctx, v.TabID, v.BlockID, nil, true); err != nil {
		t.Fatal(err)
	}
	observed := ObserveForceAgentLocal(v.BlockID)
	if observed == nil || observed.ProcessID != evidence.ProcessID || observed.Status != Status_Running {
		t.Fatal("generic forced resync changed the live agent")
	}
	if err := SendInput(v.BlockID, &BlockInputUnion{TermSize: &waveobj.TermSize{Rows: 32, Cols: 100}, InputData: []byte("finish\n")}); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-exited:
		if result.Status != Status_Done || result.ExitCode == nil || *result.ExitCode != 7 || result.AttemptID != spec.AttemptID || result.Generation != 1 {
			t.Fatalf("wrong exit evidence: %#v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("agent exit was not observed")
	}
	count, err := os.ReadFile(trace)
	if err != nil || string(count) != "x" {
		t.Fatalf("expected one process, trace=%q err=%v", count, err)
	}
	_, output, err := filestore.WFS.ReadFile(ctx, v.BlockID, wavebase.BlockFile_Term)
	if err != nil || !strings.Contains(string(output), "AGENT_INPUT:finish") {
		t.Fatalf("same terminal block did not receive I/O: %q %v", output, err)
	}
	DestroyBlockController(v.BlockID)
	if err := ResyncController(ctx, v.TabID, v.BlockID, nil, true); err != nil {
		t.Fatal(err)
	}
	count, _ = os.ReadFile(trace)
	if string(count) != "x" {
		t.Fatal("restoration after process exit started another process")
	}
}

func TestForceAgentRestoreRejectsChangedBinding(t *testing.T) {
	ctx, v := forceControllerFixture(t)
	if err := ResyncController(ctx, v.TabID, v.BlockID, nil, false); err != nil {
		t.Fatal(err)
	}
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, v.BlockID)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a damaged persisted row, bypassing the generic metadata guard.
	block.Meta["force:agentinstanceid"] = uuid.NewString()
	if err := wstore.DBUpdate(ctx, block); err != nil {
		t.Fatal(err)
	}
	if err := ResyncController(ctx, v.TabID, v.BlockID, nil, true); err == nil {
		t.Fatal("changed agent binding was accepted")
	}
}

func TestForceAgentDurableBindingGuardsRemovedMarkerAfterRestart(t *testing.T) {
	ctx, v := forceControllerFixture(t)
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, v.BlockID)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a damaged persisted row, bypassing the generic metadata guard.
	delete(block.Meta, "force:agentinstanceid")
	block.Meta[waveobj.MetaKey_Controller] = "cmd"
	block.Meta[waveobj.MetaKey_Cmd] = "must-never-launch"
	if err := wstore.DBUpdate(ctx, block); err != nil {
		t.Fatal(err)
	}
	if err := ResyncController(ctx, v.TabID, v.BlockID, nil, true); err == nil {
		t.Fatal("durable agent with removed marker entered generic startup")
	}
	if err := DestroyBlockControllerFromClient(ctx, v.BlockID); err == nil {
		t.Fatal("generic destroy accepted a persisted Force binding with removed marker")
	}
	if controller := getController(v.BlockID); controller != nil {
		t.Fatal("invalid durable binding constructed a generic controller")
	}
}

type normalDestroyController struct{ stops int }

func (*normalDestroyController) Start(context.Context, waveobj.MetaMapType, *waveobj.RuntimeOpts, bool) error {
	return nil
}
func (c *normalDestroyController) Stop(bool, string, bool) { c.stops++ }
func (*normalDestroyController) GetRuntimeStatus() *BlockControllerRuntimeStatus {
	return &BlockControllerRuntimeStatus{ShellProcStatus: Status_Init}
}
func (*normalDestroyController) GetConnName() string              { return "" }
func (*normalDestroyController) SendInput(*BlockInputUnion) error { return nil }

func TestGenericDestroyStillStopsNormalController(t *testing.T) {
	ctx, _ := forceControllerFixture(t)
	blockID := uuid.NewString()
	if err := wstore.DBInsert(ctx, &waveobj.Block{OID: blockID, Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}); err != nil {
		t.Fatal(err)
	}
	controller := &normalDestroyController{}
	registerController(blockID, controller)
	if err := DestroyBlockControllerFromClient(ctx, blockID); err != nil {
		t.Fatal(err)
	}
	if controller.stops != 1 || getController(blockID) != nil {
		t.Fatalf("normal controller was not destroyed: stops=%d current=%T", controller.stops, getController(blockID))
	}
}

func TestForceAgentStopWaitsForExitEvidenceAndOutputDrain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("first agent target is macOS arm64")
	}
	ctx, v := forceControllerFixture(t)
	executable := filepath.Join(v.RootPath, "fake agent")
	childPIDFile := filepath.Join(v.RootPath, "child-pid")
	// This fixture cooperatively reaps its own child. Background shells handle
	// PTY hangup differently across OSes; a survivor must remain uncertain.
	script := "#!/bin/sh\n" +
		"/bin/sleep 30 </dev/null >/dev/null 2>&1 &\n" +
		"child=$!\nprintf '%s' \"$child\" > '" + childPIDFile + "'\n" +
		"trap 'kill \"$child\"; wait \"$child\"; printf FINAL_OUTPUT; exit 0' TERM\n" +
		"printf READY\nwhile :; do IFS= read -r line; done\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	evidenceSaved := make(chan ForceAgentProcessEvidence, 2)
	spec := ForceAgentLocalLaunchSpec{InstanceID: v.OID, AttemptID: uuid.NewString(), Generation: 1, Process: shellexec.LocalAgentProcSpec{Executable: executable, Cwd: v.RootPath}}
	if _, err := LaunchForceAgentLocal(ctx, v.TabID, v.BlockID, spec, func(e ForceAgentProcessEvidence) { evidenceSaved <- e }); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, output, _ := filestore.WFS.ReadFile(ctx, v.BlockID, wavebase.BlockFile_Term)
		if strings.Contains(string(output), "READY") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake agent did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	childPIDBytes, err := os.ReadFile(childPIDFile)
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(childPIDBytes)))
	if err != nil || childPID < 1 {
		t.Fatalf("invalid fixture child PID: %q %v", childPIDBytes, err)
	}
	if exists, err := process.PidExists(int32(childPID)); err != nil || !exists {
		t.Fatalf("fixture child was not alive before Stop: %v %v", exists, err)
	}
	getController(v.BlockID).Stop(true, Status_Done, false)
	select {
	case result := <-evidenceSaved:
		if result.Status != Status_Done || !result.ProcessGroupAbsent || result.ProcessGroupID != result.ProcessID {
			t.Fatalf("Stop did not confirm descendant group exit: %+v", result)
		}
	default:
		t.Fatal("Stop returned before evidence callback")
	}
	_, output, err := filestore.WFS.ReadFile(ctx, v.BlockID, wavebase.BlockFile_Term)
	if err != nil || !strings.Contains(string(output), "FINAL_OUTPUT") {
		t.Fatalf("Stop did not drain final output: %q %v", output, err)
	}
	if _, err := os.Stat(childPIDFile); err != nil {
		t.Fatalf("fake descendant was not launched: %v", err)
	}
}

func TestForceAgentReportsTerminalStorageFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("first agent target is macOS arm64")
	}
	ctx, v := forceControllerFixture(t)
	if err := ResyncController(ctx, v.TabID, v.BlockID, nil, false); err != nil {
		t.Fatal(err)
	}
	c := getController(v.BlockID).(*ForceAgentController)
	c.appendOutput = func(string, string, []byte) error { return fmt.Errorf("test-owned storage unavailable") }
	executable := filepath.Join(v.RootPath, "fake agent")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf OUTPUT\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	observations := make(chan ForceAgentProcessEvidence, 4)
	spec := ForceAgentLocalLaunchSpec{InstanceID: v.OID, AttemptID: uuid.NewString(), Generation: 1, Process: shellexec.LocalAgentProcSpec{Executable: executable, Cwd: v.RootPath}}
	if _, err := LaunchForceAgentLocal(ctx, v.TabID, v.BlockID, spec, func(e ForceAgentProcessEvidence) { observations <- e }); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case result := <-observations:
			if result.ErrorCode != "terminal_storage_failed" {
				t.Fatal("storage failure was silently discarded")
			}
			if result.Status == Status_Done {
				if c.GetRuntimeStatus().ForceAgentErrorCode != result.ErrorCode {
					t.Fatal("terminal failure was hidden from the renderer")
				}
				return
			}
		case <-deadline:
			t.Fatal("storage failure evidence timed out")
		}
	}
}
