package forceservice

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func agentFixture(t *testing.T, root string) (context.Context, *ForceService, *waveobj.ForceProject, *waveobj.ForceAgentProfile, *waveobj.Tab) {
	t.Helper()
	ctx := catalogStore(t)
	svc := &ForceService{}
	project, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Project", Icon: "folder", RootPath: root})
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := svc.SaveProfile(ctx, ForceProfileInput{Title: "Builder", Icon: "robot", SystemPrompt: "Build safely", Adapter: "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	layout := &waveobj.LayoutState{OID: uuid.NewString()}
	if err := wstore.DBInsert(ctx, layout); err != nil {
		t.Fatal(err)
	}
	tab := &waveobj.Tab{OID: uuid.NewString(), Name: "Agents", LayoutState: layout.OID}
	if err := wstore.DBInsert(ctx, tab); err != nil {
		t.Fatal(err)
	}
	return ctx, svc, project, profile, tab
}

func TestAgentCreateReopenKeepsSnapshotAndInertBlock(t *testing.T) {
	root := t.TempDir()
	ctx, svc, project, profile, tab := agentFixture(t, root)
	input := ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID, CreationKey: uuid.NewString()}
	first, updates, err := svc.CreateAgentInstance(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 4 {
		t.Fatalf("updates=%d want instance, block, tab and layout", len(updates))
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	second, retryUpdates, err := svc.CreateAgentInstance(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(retryUpdates) != 0 || first.OID != second.OID || first.BlockID != second.BlockID || first.ClaudeSessionID != second.ClaudeSessionID {
		t.Fatalf("retry changed instance: first=%+v second=%+v updates=%+v", first, second, retryUpdates)
	}
	if _, err := uuid.Parse(second.ClaudeSessionID); err != nil {
		t.Fatalf("bad Claude UUID: %v", err)
	}
	if second.PromptSnapshot != "Build safely" || second.ProfileVersion != 1 || second.IdentityEvidence != "requested" || second.Generation != 0 || second.Status != "prepared" {
		t.Fatalf("bad snapshot/state: %+v", second)
	}
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, second.BlockID)
	if err != nil {
		t.Fatal(err)
	}
	if block.Meta.GetString(waveobj.MetaKey_Controller, "") != "" || block.JobId != "" || block.Meta.GetString(waveobj.MetaKey_View, "") != "term" || block.Meta.GetString("force:agentinstanceid", "") != second.OID {
		t.Fatalf("block is launchable or not linked: %+v", block)
	}
	if err := blockcontroller.ResyncController(ctx, tab.OID, block.OID, nil, false); err != nil {
		t.Fatalf("inert block failed controller restore: %v", err)
	}
	gotTab, err := wstore.DBMustGet[*waveobj.Tab](ctx, tab.OID)
	if err != nil || len(gotTab.BlockIds) != 1 || gotTab.BlockIds[0] != block.OID {
		t.Fatalf("tab block link: %+v %v", gotTab, err)
	}
	gotLayout, err := wstore.DBMustGet[*waveobj.LayoutState](ctx, tab.LayoutState)
	if err != nil || gotLayout.PendingBackendActions == nil || len(*gotLayout.PendingBackendActions) != 1 || (*gotLayout.PendingBackendActions)[0].ActionType != "insert" || (*gotLayout.PendingBackendActions)[0].BlockId != block.OID || !(*gotLayout.PendingBackendActions)[0].Focused {
		t.Fatalf("block omitted from layout: %+v %v", gotLayout, err)
	}
	if block.Meta.GetString(waveobj.MetaKey_CmdCwd, "") != project.RootPath || block.Meta.GetBool(waveobj.MetaKey_CmdRunOnStart, true) || block.Meta.GetString(waveobj.MetaKey_Connection, "missing") != "" {
		t.Fatalf("inert terminal metadata incomplete: %+v", block.Meta)
	}
	instances, err := svc.ListAgentInstances(ctx, project.OID)
	if err != nil || len(instances) != 1 {
		t.Fatalf("list=%+v err=%v", instances, err)
	}
	w, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(w, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["otype"] != waveobj.OType_ForceAgentInstance || wire["version"] == nil || wire["prompthash"] == "" {
		t.Fatalf("wire=%+v", wire)
	}
}

func TestAgentReservationCanonicalAliasAndConcurrentWriters(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	ctx, svc, project, profile, tab := agentFixture(t, root)
	aliasProject, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Alias", Icon: "folder", RootPath: alias})
	if err != nil {
		t.Fatal(err)
	}
	makeAgent := func(projectID string, version int) *waveobj.ForceAgentInstance {
		t.Helper()
		v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: projectID, ProjectVersion: version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	a, b := makeAgent(project.OID, project.Version), makeAgent(aliasProject.OID, aliasProject.Version)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{a.OID, b.OID} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := reserveOperation(ctx, id, uuid.NewString(), "start")
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrWriterConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected reservation error: %v", err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
}

func TestAgentCreateRejectsStaleArchivedAndMissingCatalog(t *testing.T) {
	ctx, svc, project, profile, tab := agentFixture(t, t.TempDir())
	base := ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID}
	stale := base
	stale.ProfileVersion++
	if _, _, err := svc.CreateAgentInstance(ctx, stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale profile: %v", err)
	}
	missing := base
	missing.ProjectID = uuid.NewString()
	if _, _, err := svc.CreateAgentInstance(ctx, missing); !errors.Is(err, wstore.ErrNotFound) {
		t.Fatalf("missing project: %v", err)
	}
	if _, err := svc.ArchiveProfile(ctx, profile.OID, profile.Version, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateAgentInstance(ctx, base); err == nil {
		t.Fatal("archived profile accepted")
	}
	if n, err := wstore.DBGetCount[*waveobj.ForceAgentInstance](ctx); err != nil || n != 0 {
		t.Fatalf("partial instances=%d err=%v", n, err)
	}
}

func TestAgentCreateRejectsTabWithoutLayoutAtomically(t *testing.T) {
	ctx, svc, project, profile, _ := agentFixture(t, t.TempDir())
	tab := &waveobj.Tab{OID: uuid.NewString(), Name: "No layout"}
	if err := wstore.DBInsert(ctx, tab); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID}); err == nil {
		t.Fatal("tab without layout accepted")
	}
	if n, err := wstore.DBGetCount[*waveobj.ForceAgentInstance](ctx); err != nil || n != 0 {
		t.Fatalf("partial instance=%d err=%v", n, err)
	}
	if n, err := wstore.DBGetCount[*waveobj.Block](ctx); err != nil || n != 0 {
		t.Fatalf("partial block=%d err=%v", n, err)
	}
}

func TestAgentReserveConflictsAndMonotonicGeneration(t *testing.T) {
	root := t.TempDir()
	ctx, svc, project, profile, tab := agentFixture(t, root)
	makeAgent := func() *waveobj.ForceAgentInstance {
		t.Helper()
		v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	a, b := makeAgent(), makeAgent()
	first, err := reserveOperation(ctx, a.OID, uuid.NewString(), "start")
	if err != nil || first.Generation != 1 {
		t.Fatalf("reserve first=%+v err=%v", first, err)
	}
	if _, err := reserveOperation(ctx, b.OID, uuid.NewString(), "start"); !errors.Is(err, ErrWriterConflict) {
		t.Fatalf("conflict=%v", err)
	}
	if _, err := reserveOperation(ctx, a.OID, uuid.NewString(), "reconnect"); !errors.Is(err, ErrOperationPending) {
		t.Fatalf("pending=%v", err)
	}
	if err := releaseWriterLease(ctx, a.OID, first.Generation, false); err == nil {
		t.Fatal("unconfirmed release accepted")
	}
	if err := releaseWriterLease(ctx, a.OID, first.Generation, true); err != nil {
		t.Fatal(err)
	}
	second, err := reserveOperation(ctx, a.OID, uuid.NewString(), "reconnect")
	if err != nil || second.Generation != 2 {
		t.Fatalf("generation=%+v err=%v", second, err)
	}
	if _, err := reserveOperation(ctx, b.OID, uuid.NewString(), "start"); !errors.Is(err, ErrWriterConflict) {
		t.Fatalf("lease not held: %v", err)
	}
}

func TestAgentReserveNewSessionChangesClaudeUUIDOnlyOnExplicitIntent(t *testing.T) {
	ctx, svc, project, profile, tab := agentFixture(t, t.TempDir())
	v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
	if err != nil {
		t.Fatal(err)
	}
	original := v.ClaudeSessionID
	first, err := reserveOperation(ctx, v.OID, uuid.NewString(), "start")
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseWriterLease(ctx, v.OID, first.Generation, true); err != nil {
		t.Fatal(err)
	}
	if err := wstore.DBUpdateFn[*waveobj.ForceAgentInstance](ctx, v.OID, func(v *waveobj.ForceAgentInstance) { v.WasLaunched, v.CurrentSessionLaunched = true, true }); err != nil {
		t.Fatal(err)
	}
	second, err := reserveOperation(ctx, v.OID, uuid.NewString(), "new-session")
	if err != nil {
		t.Fatal(err)
	}
	got, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Generation != 2 || got.ClaudeSessionID == original || got.PriorClaudeSessionID != original || got.IdentityEvidence != "requested" {
		t.Fatalf("new session checkpoint: %+v", got)
	}
}

func TestAgentRemoteRegistrationStaysInertAndReservationFailsClosed(t *testing.T) {
	ctx := catalogStore(t)
	svc := &ForceService{}
	project, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Remote", Icon: "folder", Connection: "unreachable.example.invalid", RootPath: "/srv/project"})
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := svc.SaveProfile(ctx, ForceProfileInput{Title: "Remote agent", Icon: "robot", Adapter: "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	layout := &waveobj.LayoutState{OID: uuid.NewString()}
	if err := wstore.DBInsert(ctx, layout); err != nil {
		t.Fatal(err)
	}
	tab := &waveobj.Tab{OID: uuid.NewString(), Name: "Agents", LayoutState: layout.OID}
	if err := wstore.DBInsert(ctx, tab); err != nil {
		t.Fatal(err)
	}
	v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
	if err != nil {
		t.Fatalf("remote registration tried to reach host: %v", err)
	}
	if _, err := reserveOperation(ctx, v.OID, uuid.NewString(), "start"); !errors.Is(err, ErrDestinationUnresolved) {
		t.Fatalf("unresolved remote reservation: %v", err)
	}
	if n, err := wstore.DBGetCount[*waveobj.ForceAgentInstance](ctx); err != nil || n != 1 {
		t.Fatalf("registration lost after failed reserve: count=%d err=%v", n, err)
	}
}

func TestAgentIndependentRootsReserveIndependently(t *testing.T) {
	parent := t.TempDir()
	firstRoot, secondRoot := filepath.Join(parent, "checkout-a"), filepath.Join(parent, "checkout-b")
	for _, root := range []string{firstRoot, secondRoot} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, svc, project, profile, tab := agentFixture(t, firstRoot)
	other, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Other checkout", Icon: "folder", RootPath: secondRoot})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*waveobj.ForceProject{project, other} {
		v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: p.OID, ProjectVersion: p.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := reserveOperation(ctx, v.OID, uuid.NewString(), "start"); err != nil {
			t.Fatalf("independent root %s: %v", p.RootPath, err)
		}
	}
}

func TestAgentOverlappingRootsConflictAcrossConcurrentReservations(t *testing.T) {
	for _, marker := range []string{"none", "git-directory", "git-file"} {
		t.Run(marker, func(t *testing.T) {
			parent := t.TempDir()
			child := filepath.Join(parent, "nested:checkout")
			if err := os.Mkdir(child, 0700); err != nil {
				t.Fatal(err)
			}
			switch marker {
			case "git-directory":
				if err := os.Mkdir(filepath.Join(parent, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
			case "git-file":
				if err := os.WriteFile(filepath.Join(parent, ".git"), []byte("gitdir: /elsewhere\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, svc, project, profile, tab := agentFixture(t, parent)
			childProject, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Nested", Icon: "folder", RootPath: child})
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, p := range []*waveobj.ForceProject{project, childProject} {
				v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: p.OID, ProjectVersion: p.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, v.OID)
			}
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for _, id := range ids {
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					_, err := reserveOperation(ctx, id, uuid.NewString(), "start")
					results <- err
				}(id)
			}
			wg.Wait()
			close(results)
			success, conflict := 0, 0
			for err := range results {
				if err == nil {
					success++
				} else if errors.Is(err, ErrWriterConflict) {
					conflict++
				} else {
					t.Fatalf("reservation error: %v", err)
				}
			}
			if success != 1 || conflict != 1 {
				t.Fatalf("overlapping roots: success=%d conflict=%d", success, conflict)
			}
		})
	}
}
