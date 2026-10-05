package forceservice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/shellexec"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func runtimeFixture(t *testing.T) (context.Context, *ForceService, *waveobj.ForceAgentInstance, string, string) {
	t.Helper()
	root := t.TempDir()
	ctx, svc, project, profile, tab := agentFixture(t, root)
	if err := filestore.InitFilestoreForTesting(); err != nil {
		t.Fatal(err)
	}
	v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { blockcontroller.DestroyBlockController(v.BlockID) })
	trace := filepath.Join(root, "launch-count")
	args := filepath.Join(root, "argv")
	cli := filepath.Join(root, "claude")
	promptTrace := filepath.Join(root, "prompt-trace")
	script := "#!/bin/sh\nprintf x >> '" + trace + "'\nprintf '%s\\n' \"$@\" > '" + args + "'\nprev=''\nresume=0\nfor arg in \"$@\"; do\n  if [ \"$arg\" = '--resume' ]; then resume=1; fi\n  if [ \"$prev\" = '--append-system-prompt-file' ]; then cat \"$arg\" >> '" + promptTrace + "'; printf '\\n--end--\\n' >> '" + promptTrace + "'; fi\n  prev=\"$arg\"\ndone\nif [ \"$resume\" = 1 ] && [ -f '" + filepath.Join(root, "fail-resume") + "' ]; then exit 7; fi\nprintf READY\\n\nIFS= read -r line\nexit 0\n"
	if err := os.WriteFile(cli, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := agentCLIPathForTest
	agentCLIPathForTest = cli
	t.Cleanup(func() { agentCLIPathForTest = old })
	return ctx, svc, v, trace, args
}

func TestAgentRuntimeReopenResumesExactSessionAndFrozenPrompt(t *testing.T) {
	ctx, svc, v, trace, argvPath := runtimeFixture(t)
	first, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
	profile, err := wstore.DBMustGet[*waveobj.ForceAgentProfile](ctx, v.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SaveProfile(ctx, ForceProfileInput{ID: profile.OID, ExpectedVersion: profile.Version, Title: profile.Title, Icon: profile.Icon, Adapter: profile.Adapter, SystemPrompt: "Changed later"}); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if second.Instance.OID != v.OID || second.TerminalBlockID != first.TerminalBlockID || second.Instance.ClaudeSessionID != v.ClaudeSessionID || second.Operation.Generation != 2 {
		t.Fatalf("resume changed identity: %+v", second)
	}
	waitFileCount(t, filepath.Join(v.RootPath, "prompt-trace"), "Build safely", 2)
	argv, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(argv), "--resume\n"+v.ClaudeSessionID+"\n") || strings.Contains(string(argv), "--session-id") {
		t.Fatalf("wrong resume argv: %q", argv)
	}
	prompt, err := os.ReadFile(filepath.Join(v.RootPath, "prompt-trace"))
	if err != nil || strings.Count(string(prompt), "Build safely") != 2 || strings.Contains(string(prompt), "Changed later") {
		t.Fatalf("prompt snapshot not frozen: %q %v", prompt, err)
	}
	count, err := os.ReadFile(trace)
	if err != nil || string(count) != "xx" {
		t.Fatalf("launch count=%q err=%v", count, err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func waitAgentStatus(t *testing.T, ctx context.Context, id, status string) *waveobj.ForceAgentInstance {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status == status {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("status remained %q, wanted %q: %+v", v.Status, status, v)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitFileCount(t *testing.T, path, needle string, count int) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil && strings.Count(string(data), needle) >= count {
			return string(data)
		}
		if time.Now().After(deadline) {
			t.Fatalf("file %s did not contain %d copies of %q: %q %v", path, count, needle, data, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAgentRuntimeDoubleClickUsesOneProcessAndBlock(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	key := uuid.NewString()
	results := make(chan *ForceAgentOperationResult, 2)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, _, err := svc.StartAgent(ctx, v.OID, key)
			results <- result
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	for result := range results {
		if result.Instance.OID != v.OID || result.TerminalBlockID != v.BlockID || result.Operation.Generation != 1 {
			t.Fatalf("duplicate operation: %+v", result)
		}
	}
	waitFileCount(t, trace, "x", 1)
	count, err := os.ReadFile(trace)
	if err != nil || string(count) != "x" {
		t.Fatalf("launch trace=%q err=%v", count, err)
	}
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err == nil {
		t.Fatal("new click restarted running process")
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	got := waitAgentStatus(t, ctx, v.OID, "exited")
	if got.Generation != 1 || got.LocalPID < 1 || !got.WasLaunched || got.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("bad exit checkpoint: %+v", got)
	}
}

func TestAgentRuntimeReconnectReusesConfirmedLiveProcess(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	first, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	second, updates, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString())
	if err != nil || len(updates) != 0 {
		t.Fatalf("live reconnect: %+v updates=%+v err=%v", second, updates, err)
	}
	if second.Operation.Generation != first.Operation.Generation || second.Operation.AttemptID != first.Operation.AttemptID || second.TerminalBlockID != first.TerminalBlockID || second.Instance.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("live process was replaced: %+v", second)
	}
	waitFileCount(t, trace, "x", 1)
	count, _ := os.ReadFile(trace)
	if string(count) != "x" {
		t.Fatalf("live reconnect launched duplicate: %q", count)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeKnownPreSpawnFailureReleasesLease(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if err := wstore.DBUpdateFn[*waveobj.Block](ctx, v.BlockID, func(b *waveobj.Block) {
		b.RuntimeOpts = &waveobj.RuntimeOpts{TermSize: waveobj.TermSize{Rows: -1, Cols: 80}}
	}); err != nil {
		t.Fatal(err)
	}
	failedResult, updates, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil || failedResult == nil || failedResult.Instance.ErrorCode != "launch_failed" || len(updates) == 0 {
		t.Fatalf("known launch failure: %+v updates=%+v err=%v", failedResult, updates, err)
	}
	got, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if got.WriterLeaseKey != "" || got.Status != "prepared" || got.WasLaunched || got.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("known pre-spawn failure held lease: %+v", got)
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("pre-spawn failure launched CLI: %v", err)
	}
}

func TestAgentRuntimeNewSessionPreSpawnFailureRetriesSamePreparedUUID(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
	cli := agentCLIPathForTest
	agentCLIPathForTest = filepath.Join(v.RootPath, "missing", "claude")
	failedResult, updates, err := svc.StartNewSession(ctx, v.OID, uuid.NewString())
	if err != nil || failedResult == nil || failedResult.Instance.ErrorCode != "prepare_failed" || len(updates) == 0 {
		t.Fatalf("known new-session failure: %+v updates=%+v err=%v", failedResult, updates, err)
	}
	prepared, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ClaudeSessionID == v.ClaudeSessionID || prepared.CurrentSessionLaunched || prepared.WriterLeaseKey != "" {
		t.Fatalf("new session not prepared safely: %+v", prepared)
	}
	agentCLIPathForTest = cli
	result, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil || result.Instance.ClaudeSessionID != prepared.ClaudeSessionID {
		t.Fatalf("start retry rotated prepared UUID: %+v %v", result, err)
	}
	waitFileCount(t, trace, "x", 2)
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeFailedPreparationPreservesUUIDAndAllowsRetry(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	cli := agentCLIPathForTest
	agentCLIPathForTest = filepath.Join(v.RootPath, "missing", "claude")
	failedResult, updates, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil || failedResult == nil || failedResult.Instance.ErrorCode != "prepare_failed" || len(updates) == 0 {
		t.Fatalf("known preparation failure: %+v updates=%+v err=%v", failedResult, updates, err)
	}
	failed, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.ClaudeSessionID != v.ClaudeSessionID || failed.WasLaunched || failed.WriterLeaseKey != "" || failed.Status != "prepared" {
		t.Fatalf("failed preparation changed identity or held lease: %+v", failed)
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("failed preparation launched CLI: %v", err)
	}
	agentCLIPathForTest = cli
	result, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil || result.Instance.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("retry failed: %+v %v", result, err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeUnknownLaunchCheckpointDoesNotDuplicate(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	reserved, err := reserveOperation(ctx, v.OID, uuid.NewString(), "start")
	if err != nil {
		t.Fatal(err)
	}
	if err := wstore.DBUpdateFn[*waveobj.ForceAgentInstance](ctx, v.OID, func(v *waveobj.ForceAgentInstance) {
		v.AttemptID = uuid.NewString()
		v.OperationPhase = "launch_requested"
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); !errors.Is(err, ErrAgentUncertain) {
		t.Fatalf("unknown checkpoint result: %v", err)
	}
	got, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Generation != reserved.Generation || got.Status != "uncertain" || got.ClaudeSessionID != v.ClaudeSessionID || got.WriterLeaseKey == "" {
		t.Fatalf("unknown checkpoint mutated: %+v", got)
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("unknown checkpoint launched CLI: %v", err)
	}
}

func TestAgentRuntimeSecondInstanceCannotWriteSameRoot(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	first, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: v.ProjectID, ProjectVersion: v.ProjectVersion, ProfileID: v.ProfileID, ProfileVersion: v.ProfileVersion, TabID: v.TabID})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.StartAgent(ctx, second.OID, uuid.NewString()); !errors.Is(err, ErrWriterConflict) {
		t.Fatalf("second writer: %v", err)
	}
	waitFileCount(t, trace, "x", 1)
	count, _ := os.ReadFile(trace)
	if string(count) != "x" || first.TerminalBlockID == second.BlockID {
		t.Fatalf("second launch or shared block: trace=%q first=%+v second=%+v", count, first, second)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeLeaderExitKeepsLeaseUntilProcessGroupGone(t *testing.T) {
	ctx, svc, v, _, argvPath := runtimeFixture(t)
	trace := filepath.Join(v.RootPath, "descendant-writes")
	stopFile := filepath.Join(v.RootPath, "stop-descendant")
	parentExitFile := filepath.Join(v.RootPath, "parent-exit")
	readyFile := filepath.Join(v.RootPath, "parent-ready")
	cliDir := filepath.Join(v.RootPath, "fake-cli")
	if err := os.Mkdir(cliDir, 0700); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(cliDir, "claude")
	t.Cleanup(func() { _ = os.WriteFile(stopFile, nil, 0600) })
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > '" + argvPath + "'\n" +
		"(trap '' HUP TERM; while [ ! -e '" + stopFile + "' ]; do printf c >> '" + trace + "'; /bin/sleep 0.02; done) </dev/null >/dev/null 2>&1 &\n" +
		"printf ready > '" + readyFile + "'\n" +
		"while [ ! -e '" + parentExitFile + "' ]; do /bin/sleep 0.02; done\n" +
		"exit 0\n"
	if err := os.WriteFile(cli, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	oldCLI := agentCLIPathForTest
	agentCLIPathForTest = cli
	t.Cleanup(func() { agentCLIPathForTest = oldCLI })
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	waitFileCount(t, trace, "c", 1)
	waitFileCount(t, readyFile, "ready", 1)
	if err := os.WriteFile(parentExitFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	uncertain := waitAgentStatus(t, ctx, v.OID, "uncertain")
	argvBytes, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatal(err)
	}
	var privatePrompt string
	args := strings.Split(strings.TrimSpace(string(argvBytes)), "\n")
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--append-system-prompt-file" {
			privatePrompt = args[i+1]
		}
	}
	if privatePrompt == "" {
		t.Fatal("fake CLI did not receive its private prompt file")
	}
	if _, err := os.Stat(privatePrompt); err != nil {
		t.Fatalf("private prompt removed while descendants remained: %v", err)
	}
	if uncertain.WriterLeaseKey == "" || uncertain.LocalPID < 1 || uncertain.LocalProcessGroupID != uncertain.LocalPID ||
		uncertain.LocalProcessGroupStartTs != uncertain.LocalProcessStartTs || uncertain.LocalBootID == "" {
		t.Fatalf("leader exit did not retain complete group evidence and lease: %+v", uncertain)
	}
	second, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{
		ProjectID: v.ProjectID, ProjectVersion: v.ProjectVersion,
		ProfileID: v.ProfileID, ProfileVersion: v.ProfileVersion, TabID: v.TabID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.StartAgent(ctx, second.OID, uuid.NewString()); !errors.Is(err, ErrWriterConflict) {
		t.Fatalf("second writer started while old group survived: %v", err)
	}
	before, err := os.ReadFile(trace)
	if err != nil || len(before) == 0 {
		t.Fatalf("descendant did not continue writing after its CLI parent exited: %q %v", before, err)
	}
	if err := os.WriteFile(stopFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		state, err := shellexec.InspectAgentProcessGroup(uncertain.LocalProcessGroupID)
		if err == nil && state == shellexec.AgentProcessGroupAbsent {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("owned process group did not disappear: state=%v err=%v", state, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Reconciliation now has kernel evidence that the original group is absent;
	// it can release the lease and resume the same saved session UUID.
	if _, err := svc.ListAgentInstances(ctx, v.ProjectID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(privatePrompt); !os.IsNotExist(err) {
		t.Fatalf("private prompt retained after confirmed group absence: %v", err)
	}
	if err := os.Remove(parentExitFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(readyFile); err != nil {
		t.Fatal(err)
	}
	resumed, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatalf("confirmed group absence did not reconcile: %v", err)
	}
	waitFileCount(t, readyFile, "ready", 1)
	if err := os.WriteFile(parentExitFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if resumed.Instance.ClaudeSessionID != v.ClaudeSessionID || resumed.Operation.Generation != uncertain.Generation+1 {
		t.Fatalf("reconciliation changed the session identity: %+v", resumed)
	}
	completed := waitAgentStatus(t, ctx, v.OID, "exited")
	if completed.WriterLeaseKey != "" {
		t.Fatalf("completed group retained writer lease: %+v", completed)
	}
}

func TestAgentRuntimeNewSessionRequiresConfirmedExitAndChangesOnlyConversation(t *testing.T) {
	ctx, svc, v, trace, argvPath := runtimeFixture(t)
	if _, _, err := svc.StartNewSession(ctx, v.OID, uuid.NewString()); err == nil {
		t.Fatal("new session before first launch accepted")
	}
	_, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.StartNewSession(ctx, v.OID, uuid.NewString()); err == nil {
		t.Fatal("new session while live accepted")
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
	second, _, err := svc.StartNewSession(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if second.Instance.ClaudeSessionID == v.ClaudeSessionID || second.Instance.PriorClaudeSessionID != v.ClaudeSessionID || second.TerminalBlockID != v.BlockID {
		t.Fatalf("new session identity: %+v", second)
	}
	waitFileCount(t, trace, "x", 2)
	argv, err := os.ReadFile(argvPath)
	if err != nil || !strings.Contains(string(argv), "--session-id\n"+second.Instance.ClaudeSessionID+"\n") || strings.Contains(string(argv), "--resume") {
		t.Fatalf("new session argv=%q err=%v", argv, err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeRefusesResumeWithoutSavedExecutionContext(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if err := wstore.DBUpdateFn[*waveobj.ForceAgentInstance](ctx, v.OID, func(v *waveobj.ForceAgentInstance) {
		v.WasLaunched, v.CurrentSessionLaunched, v.Status = true, true, "exited"
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString()); !errors.Is(err, ErrDestinationUnresolved) {
		t.Fatalf("missing saved context was accepted: %v", err)
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("unverified resume launched CLI: %v", err)
	}
}

func TestAgentRuntimeChangedHistoryContextBlocksResume(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	finished := waitAgentStatus(t, ctx, v.OID, "exited")
	if finished.CLIHistoryContext == "" || strings.Contains(finished.CLIHistoryContext, v.RootPath) {
		t.Fatalf("history context was absent or persisted as plaintext: %+v", finished)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "other-history"))
	if _, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString()); !errors.Is(err, ErrDestinationUnresolved) {
		t.Fatalf("changed history accepted: %v", err)
	}
	waitFileCount(t, trace, "x", 1)
	count, _ := os.ReadFile(trace)
	if string(count) != "x" {
		t.Fatalf("changed history caused launch: %q", count)
	}
}

func TestAgentRuntimeReservedCheckpointBeforeSpawnCanBeRetried(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if _, err := reserveOperation(ctx, v.OID, uuid.NewString(), "start"); err != nil {
		t.Fatal(err)
	}
	result, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil || result.Operation.Generation != 2 || result.Instance.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("reserved checkpoint retry: %+v %v", result, err)
	}
	waitFileCount(t, trace, "x", 1)
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func checkpointLostController(t *testing.T, ctx context.Context, v *waveobj.ForceAgentInstance, pid int, startTs int64) {
	t.Helper()
	_, contextHash, err := currentAgentContext()
	if err != nil {
		t.Fatal(err)
	}
	root, err := agentExecutionRoot(v)
	if err != nil {
		t.Fatal(err)
	}
	dest, err := canonicalDestination(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reserveOperation(ctx, v.OID, uuid.NewString(), "start"); err != nil {
		t.Fatal(err)
	}
	bootID, err := shellexec.CurrentAgentBootID()
	if err != nil {
		t.Fatal(err)
	}
	if err := wstore.DBUpdateFn[*waveobj.ForceAgentInstance](ctx, v.OID, func(current *waveobj.ForceAgentInstance) {
		current.AttemptID = uuid.NewString()
		current.OperationPhase, current.Status = "running", "running"
		current.WasLaunched, current.CurrentSessionLaunched = true, true
		current.LocalPID, current.LocalProcessStartTs = pid, startTs
		current.LocalProcessGroupID, current.LocalProcessGroupStartTs = pid, startTs
		current.LocalBootID = bootID
		current.ExecutionRoot, current.ExecutionLeaseRoot, current.CLIHistoryContext = root, dest.canonicalRoot, contextHash
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRuntimeConfirmedAbsentPIDResumesExactUUID(t *testing.T) {
	ctx, svc, v, trace, argvPath := runtimeFixture(t)
	checkpointLostController(t, ctx, v, 2147483647, 1)
	result, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation.Generation != 2 || result.Instance.ClaudeSessionID != v.ClaudeSessionID || result.TerminalBlockID != v.BlockID {
		t.Fatalf("resume identity: %+v", result)
	}
	waitFileCount(t, trace, "x", 1)
	argv, err := os.ReadFile(argvPath)
	if err != nil || !strings.Contains(string(argv), "--resume\n"+v.ClaudeSessionID+"\n") {
		t.Fatalf("resume argv=%q err=%v", argv, err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeLivePIDWithoutPTYRemainsUncertain(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	proc, err := process.NewProcess(int32(pgid))
	if err != nil {
		t.Fatal(err)
	}
	started, err := proc.CreateTime()
	if err != nil {
		t.Fatal(err)
	}
	checkpointLostController(t, ctx, v, pgid, started)
	if _, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString()); !errors.Is(err, ErrAgentUncertain) {
		t.Fatalf("live orphan process: %v", err)
	}
	got, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "uncertain" || got.WriterLeaseKey == "" || got.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("unsafe orphan state: %+v", got)
	}
	// A missing reboot identity cannot override a live process-group result.
	if err := wstore.DBUpdateFn[*waveobj.ForceAgentInstance](ctx, v.OID, func(current *waveobj.ForceAgentInstance) {
		current.LocalBootID = ""
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString()); !errors.Is(err, ErrAgentUncertain) {
		t.Fatalf("unknown boot identity plus live group did not remain uncertain: %v", err)
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("uncertain process caused launch: %v", err)
	}
}

func TestAgentListReconcilesLostLocalControllerWithoutLaunching(t *testing.T) {
	for _, state := range []string{"alive", "absent"} {
		t.Run(state, func(t *testing.T) {
			ctx, svc, v, trace, _ := runtimeFixture(t)
			pid, started := 2147483647, int64(1)
			if state == "alive" {
				pgid, err := syscall.Getpgid(os.Getpid())
				if err != nil {
					t.Fatal(err)
				}
				proc, err := process.NewProcess(int32(pgid))
				if err != nil {
					t.Fatal(err)
				}
				pid, started = pgid, mustProcessStart(t, proc)
			}
			checkpointLostController(t, ctx, v, pid, started)
			if err := wstore.InitWStore(); err != nil {
				t.Fatal(err)
			}
			listed, err := svc.ListAgentInstances(ctx, v.ProjectID)
			if err != nil || len(listed) != 1 {
				t.Fatalf("list=%+v err=%v", listed, err)
			}
			want := "exited"
			if state == "alive" {
				want = "uncertain"
			}
			if listed[0].Status != want || (listed[0].WriterLeaseKey != "") != (state == "alive") {
				t.Fatalf("lost controller state=%+v want=%s", listed[0], want)
			}
			version := listed[0].Version
			again, err := svc.ListAgentInstances(ctx, v.ProjectID)
			if err != nil || len(again) != 1 || again[0].Version != version {
				t.Fatalf("list generated repeat update: first=%+v again=%+v err=%v", listed, again, err)
			}
			if _, err := os.Stat(trace); !os.IsNotExist(err) {
				t.Fatalf("listing launched fake CLI: %v", err)
			}
		})
	}
}

func mustProcessStart(t *testing.T, proc *process.Process) int64 {
	t.Helper()
	started, err := proc.CreateTime()
	if err != nil {
		t.Fatal(err)
	}
	return started
}

func TestAgentRuntimeFailedResumeKeepsUUIDAndCanRetry(t *testing.T) {
	ctx, svc, v, trace, argvPath := runtimeFixture(t)
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
	cli := agentCLIPathForTest
	agentCLIPathForTest = filepath.Join(v.RootPath, "missing", "claude")
	failedResult, updates, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString())
	if err != nil || failedResult == nil || failedResult.Instance.ErrorCode != "prepare_failed" || len(updates) == 0 {
		t.Fatalf("known resume failure: %+v updates=%+v err=%v", failedResult, updates, err)
	}
	failed, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.ClaudeSessionID != v.ClaudeSessionID || failed.WriterLeaseKey != "" || failed.Status != "resume_failed" {
		t.Fatalf("failed resume mutated conversation: %+v", failed)
	}
	waitFileCount(t, trace, "x", 1)
	count, _ := os.ReadFile(trace)
	if string(count) != "x" {
		t.Fatalf("failed resume fell back to start: %q", count)
	}
	agentCLIPathForTest = cli
	result, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString())
	if err != nil || result.Instance.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("resume retry: %+v %v", result, err)
	}
	waitFileCount(t, trace, "x", 2)
	argv, err := os.ReadFile(argvPath)
	if err != nil || !strings.Contains(string(argv), "--resume\n"+v.ClaudeSessionID+"\n") {
		t.Fatalf("retry argv=%q err=%v", argv, err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
}

func TestAgentRuntimeNonzeroResumeExitPreservesExactConversation(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := blockcontroller.SendInput(v.BlockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, v.OID, "exited")
	if err := os.WriteFile(filepath.Join(v.RootPath, "fail-resume"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ReconnectAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	failed := waitAgentStatus(t, ctx, v.OID, "resume_failed")
	if failed.ClaudeSessionID != v.ClaudeSessionID || failed.AttemptExitCode == nil || *failed.AttemptExitCode != 7 || failed.WriterLeaseKey != "" || failed.ErrorCode != "resume_exit_nonzero" {
		t.Fatalf("nonzero resume evidence: %+v", failed)
	}
	waitFileCount(t, trace, "x", 2)
	count, _ := os.ReadFile(trace)
	if string(count) != "xx" {
		t.Fatalf("nonzero resume started a new conversation: %q", count)
	}
}
