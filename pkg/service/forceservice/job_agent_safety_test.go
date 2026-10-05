package forceservice

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/jobcontroller"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func forceJobGuardFixture(t *testing.T) (context.Context, *waveobj.ForceAgentInstance) {
	t.Helper()
	ctx, svc, project, profile, tab := agentFixture(t, t.TempDir())
	instance, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{
		ProjectID: project.OID, ProjectVersion: project.Version,
		ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, instance
}

func TestGenericJobAttachCannotReplaceForceTerminal(t *testing.T) {
	for _, removeMarker := range []bool{false, true} {
		name := "with-marker"
		if removeMarker {
			name = "persistent-binding-only"
		}
		t.Run(name, func(t *testing.T) {
			ctx, instance := forceJobGuardFixture(t)
			if removeMarker {
				if err := wstore.DBUpdateFn(ctx, instance.BlockID, func(block *waveobj.Block) {
					delete(block.Meta, "force:agentinstanceid")
				}); err != nil {
					t.Fatal(err)
				}
			}
			block, err := wstore.DBMustGet[*waveobj.Block](ctx, instance.BlockID)
			if err != nil {
				t.Fatal(err)
			}
			job := &waveobj.Job{OID: uuid.NewString(), Connection: "fixture-unconnected", Meta: waveobj.MetaMapType{}}
			if err := wstore.DBInsert(ctx, job); err != nil {
				t.Fatal(err)
			}
			if err := jobcontroller.AttachJobToBlock(ctx, job.OID, block.OID); !errors.Is(err, wstore.ErrForceAgentProtected) {
				t.Fatalf("generic attachment was not rejected: %v", err)
			}
			gotBlock, err := wstore.DBMustGet[*waveobj.Block](ctx, block.OID)
			if err != nil {
				t.Fatal(err)
			}
			gotJob, err := wstore.DBMustGet[*waveobj.Job](ctx, job.OID)
			if err != nil {
				t.Fatal(err)
			}
			if gotBlock.JobId != "" || gotBlock.Version != block.Version || gotJob.AttachedBlockId != "" || gotJob.Version != job.Version {
				t.Fatal("rejected attachment changed block or job")
			}

			ordinary := &waveobj.Block{OID: uuid.NewString(), ParentORef: waveobj.MakeORef(waveobj.OType_Tab, instance.TabID).String(), Meta: waveobj.MetaMapType{"view": "term"}}
			if err := wstore.DBInsert(ctx, ordinary); err != nil {
				t.Fatal(err)
			}
			if err := jobcontroller.AttachJobToBlock(ctx, job.OID, ordinary.OID); err != nil {
				t.Fatalf("ordinary attachment regressed: %v", err)
			}
			gotBlock, err = wstore.DBMustGet[*waveobj.Block](ctx, ordinary.OID)
			if err != nil {
				t.Fatal(err)
			}
			gotJob, err = wstore.DBMustGet[*waveobj.Job](ctx, job.OID)
			if err != nil {
				t.Fatal(err)
			}
			if gotBlock.JobId != job.OID || gotJob.AttachedBlockId != ordinary.OID {
				t.Fatal("ordinary block/job association was not persisted")
			}
		})
	}
}

func TestGenericJobStartRejectsForceBlockBeforeConnectionOrPersistence(t *testing.T) {
	ctx, instance := forceJobGuardFixture(t)
	jobID, err := jobcontroller.StartJob(ctx, jobcontroller.StartJobParams{
		ConnName: "fixture-unconnected", JobKind: jobcontroller.JobKind_Task,
		Cmd: "never-executed", BlockId: instance.BlockID,
	})
	if !errors.Is(err, wstore.ErrForceAgentProtected) || jobID != "" {
		t.Fatalf("expected protection before connection check: id=%q err=%v", jobID, err)
	}
	jobs, err := wstore.DBGetAllObjsByType[*waveobj.Job](ctx, waveobj.OType_Job)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatal("rejected start persisted a job")
	}
}
