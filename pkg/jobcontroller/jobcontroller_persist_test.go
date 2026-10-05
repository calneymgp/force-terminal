package jobcontroller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func jobPersistenceFixture(t *testing.T) context.Context {
	t.Helper()
	previousDataDir := wavebase.DataHome_VarCache
	wavebase.DataHome_VarCache = t.TempDir()
	t.Cleanup(func() { wavebase.DataHome_VarCache = previousDataDir })
	if err := os.MkdirAll(filepath.Join(wavebase.DataHome_VarCache, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	return context.Background()
}

func TestJobPersistenceRollsBackIfDestinationBecameForce(t *testing.T) {
	ctx := jobPersistenceFixture(t)
	block := &waveobj.Block{OID: uuid.NewString(), Meta: waveobj.MetaMapType{"view": "term"}}
	if err := wstore.DBInsert(ctx, block); err != nil {
		t.Fatal(err)
	}
	instance := &waveobj.ForceAgentInstance{OID: uuid.NewString(), BlockID: block.OID}
	if err := wstore.DBInsert(ctx, instance); err != nil {
		t.Fatal(err)
	}
	job := &waveobj.Job{OID: uuid.NewString(), AttachedBlockId: block.OID, JobManagerStatus: JobManagerStatus_Init}
	if err := persistJobAndAttach(ctx, job); !errors.Is(err, wstore.ErrForceAgentProtected) {
		t.Fatalf("changed destination was not rejected: %v", err)
	}
	got, err := wstore.DBGet[*waveobj.Job](ctx, job.OID)
	if err != nil || got != nil {
		t.Fatalf("rejected job remained in the database: %v", err)
	}
	gotBlock, err := wstore.DBMustGet[*waveobj.Block](ctx, block.OID)
	if err != nil || gotBlock.JobId != "" || gotBlock.Version != block.Version {
		t.Fatalf("rejected persistence changed the terminal: %v", err)
	}
}

func TestJobPersistenceKeepsOrdinaryAttachmentAndUnattachedJobs(t *testing.T) {
	ctx := jobPersistenceFixture(t)
	oldJob := &waveobj.Job{OID: uuid.NewString()}
	block := &waveobj.Block{OID: uuid.NewString(), JobId: oldJob.OID, Meta: waveobj.MetaMapType{"view": "term"}}
	oldJob.AttachedBlockId = block.OID
	if err := wstore.DBInsert(ctx, block); err != nil {
		t.Fatal(err)
	}
	if err := wstore.DBInsert(ctx, oldJob); err != nil {
		t.Fatal(err)
	}
	job := &waveobj.Job{OID: uuid.NewString(), AttachedBlockId: block.OID}
	if err := persistJobAndAttach(ctx, job); err != nil {
		t.Fatal(err)
	}
	gotBlock, err := wstore.DBMustGet[*waveobj.Block](ctx, block.OID)
	if err != nil {
		t.Fatal(err)
	}
	gotJob, err := wstore.DBMustGet[*waveobj.Job](ctx, job.OID)
	if err != nil {
		t.Fatal(err)
	}
	gotOldJob, err := wstore.DBMustGet[*waveobj.Job](ctx, oldJob.OID)
	if err != nil {
		t.Fatal(err)
	}
	if gotBlock.JobId != job.OID || gotJob.AttachedBlockId != block.OID || gotOldJob.AttachedBlockId != "" {
		t.Fatal("ordinary replacement did not persist both associations")
	}
	unattached := &waveobj.Job{OID: uuid.NewString()}
	if err := persistJobAndAttach(ctx, unattached); err != nil {
		t.Fatal(err)
	}
	gotJob, err = wstore.DBMustGet[*waveobj.Job](ctx, unattached.OID)
	if err != nil || gotJob.AttachedBlockId != "" {
		t.Fatalf("unattached job regressed: %v", err)
	}
}

func TestJobPersistenceRollsBackMissingBlock(t *testing.T) {
	ctx := jobPersistenceFixture(t)
	job := &waveobj.Job{OID: uuid.NewString(), AttachedBlockId: uuid.NewString()}
	if err := persistJobAndAttach(ctx, job); err == nil {
		t.Fatal("missing attachment target was accepted")
	}
	got, err := wstore.DBGet[*waveobj.Job](ctx, job.OID)
	if err != nil || got != nil {
		t.Fatalf("failed attachment left an orphan job: %v", err)
	}
}
