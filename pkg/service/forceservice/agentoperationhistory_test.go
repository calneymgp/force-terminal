package forceservice

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	dbfs "github.com/wavetermdev/waveterm/db"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/util/migrateutil"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func finishHistoryAttempt(t *testing.T, ctx context.Context, blockID, instanceID string) {
	t.Helper()
	if err := blockcontroller.SendInput(blockID, &blockcontroller.BlockInputUnion{InputData: []byte("exit\n")}); err != nil {
		t.Fatal(err)
	}
	waitAgentStatus(t, ctx, instanceID, "exited")
}

func TestAgentHistoricalNewSessionKeyNeverLaunchesAgain(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	if _, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	finishHistoryAttempt(t, ctx, v.BlockID, v.OID)
	keyA := uuid.NewString()
	a, _, err := svc.StartNewSession(ctx, v.OID, keyA)
	if err != nil {
		t.Fatal(err)
	}
	finishHistoryAttempt(t, ctx, v.BlockID, v.OID)
	keyB := uuid.NewString()
	b, _, err := svc.StartNewSession(ctx, v.OID, keyB)
	if err != nil {
		t.Fatal(err)
	}
	finishHistoryAttempt(t, ctx, v.BlockID, v.OID)
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	replay, updates, err := svc.StartNewSession(ctx, v.OID, keyA)
	if err != nil || len(updates) != 0 || replay == nil {
		t.Fatalf("historical replay: %+v updates=%+v err=%v", replay, updates, err)
	}
	if replay.Operation.RequestKey != keyA || replay.Operation.Intent != "new-session" || replay.Operation.Generation != a.Operation.Generation || replay.Operation.Phase != "superseded" || replay.Operation.Status != "superseded" || replay.Instance.ClaudeSessionID != b.Instance.ClaudeSessionID || replay.Instance.Generation != b.Operation.Generation {
		t.Fatalf("historical replay changed conversation or lost receipt: %+v", replay)
	}
	if _, _, err := svc.ReconnectAgent(ctx, v.OID, keyA); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong-intent replay=%v", err)
	}
	if _, err := reserveOperation(ctx, v.OID, keyA, "new-session"); !errors.Is(err, ErrOperationSuperseded) {
		t.Fatalf("direct reserve reused historical key: %v", err)
	}
	if count, err := os.ReadFile(trace); err != nil || string(count) != "xxx" {
		t.Fatalf("historical replay launched CLI: count=%q err=%v", count, err)
	}
}

func TestAgentLiveReconnectReceiptSurvivesExitAndReopen(t *testing.T) {
	ctx, svc, v, trace, _ := runtimeFixture(t)
	started, _, err := svc.StartAgent(ctx, v.OID, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	live, _, err := svc.ReconnectAgent(ctx, v.OID, key)
	if err != nil || live.Operation.Generation != started.Operation.Generation {
		t.Fatalf("live reconnect: %+v %v", live, err)
	}
	finishHistoryAttempt(t, ctx, v.BlockID, v.OID)
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	replay, updates, err := svc.ReconnectAgent(ctx, v.OID, key)
	if err != nil || len(updates) != 0 || replay == nil {
		t.Fatalf("replayed live reconnect: %+v updates=%+v err=%v", replay, updates, err)
	}
	if replay.Operation.RequestKey != key || replay.Operation.Intent != "reconnect" || replay.Operation.Generation != started.Operation.Generation || replay.Operation.Phase != "superseded" || replay.Operation.Status != "superseded" || replay.Instance.ClaudeSessionID != v.ClaudeSessionID {
		t.Fatalf("live receipt replay lost context: %+v", replay)
	}
	if count, err := os.ReadFile(trace); err != nil || string(count) != "x" {
		t.Fatalf("live reconnect replay launched CLI: count=%q err=%v", count, err)
	}
}

func TestAgentCanonicalDestinationRejectsSymlinkGitMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nested"), filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalDestination(&waveobj.ForceAgentInstance{RootPath: filepath.Join(root, "nested")}); !errors.Is(err, ErrDestinationUnresolved) {
		t.Fatalf("symlink .git marker accepted: %v", err)
	}
}

func TestAgentRequestLedgerMigrationBackfillsLatestAcceptedKey(t *testing.T) {
	ctx, svc, project, profile, tab := agentFixture(t, t.TempDir())
	v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	v.OperationRequestKey, v.OperationIntent, v.Generation = key, "start", 4
	if err := wstore.DBUpdate(ctx, v); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", wstore.GetDBName())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m, err := migrateutil.MakeMigrate("wstore", db, dbfs.WStoreMigrationFS, "migrations-wstore")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	receipt, err := agentRequestReceiptFor(ctx, v.OID, key)
	if err != nil || receipt == nil || receipt.Generation != 4 || receipt.Intent != "start" || receipt.Kind != agentRequestOperation {
		t.Fatalf("version 13 latest request was not backfilled: %+v %v", receipt, err)
	}
}
