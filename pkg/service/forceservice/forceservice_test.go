// SPDX-License-Identifier: Apache-2.0
package forceservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

type catalogEventClient struct {
	ctx    context.Context
	events []wps.WaveEvent
	err    error
}

type catalogRecordingClient struct {
	events []wps.WaveEvent
}

func (client *catalogRecordingClient) SendEvent(_ string, event wps.WaveEvent) {
	client.events = append(client.events, event)
}

func (client *catalogEventClient) SendEvent(_ string, event wps.WaveEvent) {
	client.events = append(client.events, event)
	update, ok := event.Data.(waveobj.WaveObjUpdate)
	if !ok {
		client.err = fmt.Errorf("unexpected event data type %T", event.Data)
		return
	}
	_, client.err = wstore.DBMustGet[*waveobj.ForceProject](client.ctx, update.OID)
}

func catalogStore(t *testing.T) context.Context {
	t.Helper()
	dataDir := t.TempDir()
	wavebase.DataHome_VarCache = dataDir
	if err := os.MkdirAll(filepath.Join(dataDir, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	return context.Background()
}

func inputWithCreationKey(t *testing.T, input any, key string, output any) {
	t.Helper()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	encodedKey, err := json.Marshal(key)
	if err != nil {
		t.Fatal(err)
	}
	fields["creationkey"] = encodedKey
	encoded, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, output); err != nil {
		t.Fatal(err)
	}
}

func projectInputWithCreationKey(t *testing.T, input ForceProjectInput, key string) ForceProjectInput {
	t.Helper()
	var output ForceProjectInput
	inputWithCreationKey(t, input, key, &output)
	return output
}

func profileInputWithCreationKey(t *testing.T, input ForceProfileInput, key string) ForceProfileInput {
	t.Helper()
	var output ForceProfileInput
	inputWithCreationKey(t, input, key, &output)
	return output
}

func TestCreationKeyRetryAfterReopenReturnsExistingWithoutUpdates(t *testing.T) {
	ctx := catalogStore(t)
	svc := &ForceService{}
	client := &catalogRecordingClient{}
	previousClient := wps.Broker.GetClient()
	wps.Broker.SetClient(client)
	wps.Broker.Subscribe("force-catalog-idempotency-test", wps.SubscriptionRequest{Event: wps.Event_WaveObjUpdate, AllScopes: true})
	t.Cleanup(func() {
		wps.Broker.UnsubscribeAll("force-catalog-idempotency-test")
		wps.Broker.SetClient(previousClient)
	})
	projectKey := "11111111-1111-4111-8111-111111111111"
	profileKey := "22222222-2222-4222-8222-222222222222"
	projectInput := projectInputWithCreationKey(t, ForceProjectInput{
		Name: "Retry project", Icon: "folder", RootPath: t.TempDir(),
	}, projectKey)
	profileInput := profileInputWithCreationKey(t, ForceProfileInput{
		Title: "Retry profile", Icon: "robot", SystemPrompt: "Keep this profile", Adapter: "codex",
	}, profileKey)

	createdProject, createdProjectUpdates, err := svc.SaveProject(ctx, projectInput)
	if err != nil || createdProject.OID != projectKey || createdProject.Version != 1 || len(createdProjectUpdates) != 1 {
		t.Fatalf("initial keyed project create: project=%+v updates=%+v err=%v", createdProject, createdProjectUpdates, err)
	}
	createdProfile, createdProfileUpdates, err := svc.SaveProfile(ctx, profileInput)
	if err != nil || createdProfile.OID != profileKey || createdProfile.Version != 1 || len(createdProfileUpdates) != 1 {
		t.Fatalf("initial keyed profile create: profile=%+v updates=%+v err=%v", createdProfile, createdProfileUpdates, err)
	}
	if len(client.events) != 2 {
		t.Fatalf("initial creates emitted %d events, want 2", len(client.events))
	}
	projectUpdatedAt, profileUpdatedAt := createdProject.UpdatedAt, createdProfile.UpdatedAt

	if err := wstore.InitWStore(); err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	retriedProject, projectUpdates, err := svc.SaveProject(ctx, projectInput)
	if err != nil || retriedProject.OID != projectKey || retriedProject.Version != 1 || retriedProject.UpdatedAt != projectUpdatedAt || len(projectUpdates) != 0 {
		t.Fatalf("project retry did not return the original unchanged object: project=%+v updates=%+v err=%v", retriedProject, projectUpdates, err)
	}
	retriedProfile, profileUpdates, err := svc.SaveProfile(ctx, profileInput)
	if err != nil || retriedProfile.OID != profileKey || retriedProfile.Version != 1 || retriedProfile.UpdatedAt != profileUpdatedAt || len(profileUpdates) != 0 {
		t.Fatalf("profile retry did not return the original unchanged object: profile=%+v updates=%+v err=%v", retriedProfile, profileUpdates, err)
	}
	if len(client.events) != 2 {
		t.Fatalf("retries emitted events: got %d total, want only the 2 initial creates", len(client.events))
	}
	catalog, err := svc.GetCatalog(ctx)
	if err != nil || len(catalog.Projects) != 1 || len(catalog.Profiles) != 1 {
		t.Fatalf("retries duplicated catalog entries: catalog=%+v err=%v", catalog, err)
	}
}

func TestCreationKeyRejectsChangedOrArchivedEntriesWithoutOverwriting(t *testing.T) {
	ctx := catalogStore(t)
	svc := &ForceService{}
	projectKey := "33333333-3333-4333-8333-333333333333"
	profileKey := "44444444-4444-4444-8444-444444444444"
	projectInput := projectInputWithCreationKey(t, ForceProjectInput{
		Name: "Original project", Icon: "folder", RootPath: t.TempDir(),
	}, projectKey)
	profileInput := profileInputWithCreationKey(t, ForceProfileInput{
		Title: "Original profile", Icon: "robot", SystemPrompt: "Original prompt", Adapter: "codex",
	}, profileKey)
	project, _, err := svc.SaveProject(ctx, projectInput)
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := svc.SaveProfile(ctx, profileInput)
	if err != nil {
		t.Fatal(err)
	}
	changedProject := projectInput
	changedProject.Name = "Changed project"
	if _, _, err := svc.SaveProject(ctx, changedProject); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed project retry error = %v, want ErrConflict", err)
	}
	changedProfile := profileInput
	changedProfile.SystemPrompt = "Changed prompt"
	if _, _, err := svc.SaveProfile(ctx, changedProfile); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed profile retry error = %v, want ErrConflict", err)
	}
	catalog, err := svc.GetCatalog(ctx)
	if err != nil || len(catalog.Projects) != 1 || len(catalog.Profiles) != 1 || catalog.Projects[0].Name != "Original project" || catalog.Profiles[0].SystemPrompt != "Original prompt" {
		t.Fatalf("changed retries altered or duplicated entries: catalog=%+v err=%v", catalog, err)
	}
	if _, err := svc.ArchiveProject(ctx, project.OID, project.Version, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ArchiveProfile(ctx, profile.OID, profile.Version, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SaveProject(ctx, projectInput); !errors.Is(err, ErrConflict) {
		t.Fatalf("archived project retry error = %v, want ErrConflict", err)
	}
	if _, _, err := svc.SaveProfile(ctx, profileInput); !errors.Is(err, ErrConflict) {
		t.Fatalf("archived profile retry error = %v, want ErrConflict", err)
	}
	catalog, err = svc.GetCatalog(ctx)
	if err != nil || len(catalog.Projects) != 1 || len(catalog.Profiles) != 1 || !catalog.Projects[0].Archived || !catalog.Profiles[0].Archived {
		t.Fatalf("archived retry altered or duplicated entries: catalog=%+v err=%v", catalog, err)
	}
}

func TestCreationKeyMustBeUUIDAndCannotBeUsedForEdit(t *testing.T) {
	ctx := catalogStore(t)
	svc := &ForceService{}
	root := t.TempDir()
	project, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Original project", Icon: "folder", RootPath: root})
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := svc.SaveProfile(ctx, ForceProfileInput{Title: "Original profile", Icon: "robot", Adapter: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SaveProject(ctx, projectInputWithCreationKey(t, ForceProjectInput{
		Name: "Invalid key", Icon: "folder", RootPath: root,
	}, "not-a-uuid")); err == nil {
		t.Fatal("invalid project creation key was accepted")
	}
	if _, _, err := svc.SaveProfile(ctx, profileInputWithCreationKey(t, ForceProfileInput{
		Title: "Invalid key", Icon: "robot", Adapter: "codex",
	}, "not-a-uuid")); err == nil {
		t.Fatal("invalid profile creation key was accepted")
	}
	projectEdit := projectInputWithCreationKey(t, ForceProjectInput{
		ID: project.OID, ExpectedVersion: project.Version, Name: "Changed project", Icon: "folder", RootPath: root,
	}, "55555555-5555-4555-8555-555555555555")
	if _, _, err := svc.SaveProject(ctx, projectEdit); err == nil {
		t.Fatal("project edit with creation key was accepted")
	}
	profileEdit := profileInputWithCreationKey(t, ForceProfileInput{
		ID: profile.OID, ExpectedVersion: profile.Version, Title: "Changed profile", Icon: "robot", Adapter: "codex",
	}, "66666666-6666-4666-8666-666666666666")
	if _, _, err := svc.SaveProfile(ctx, profileEdit); err == nil {
		t.Fatal("profile edit with creation key was accepted")
	}
	catalog, err := svc.GetCatalog(ctx)
	if err != nil || len(catalog.Projects) != 1 || len(catalog.Profiles) != 1 || catalog.Projects[0].Name != "Original project" || catalog.Profiles[0].Title != "Original profile" {
		t.Fatalf("invalid creation keys changed catalog: catalog=%+v err=%v", catalog, err)
	}
}

func TestCatalogPersistsAcrossReopenAndKeepsArchived(t *testing.T) {
	ctx := catalogStore(t)
	svc := &ForceService{}
	localRoot := t.TempDir()
	inputs := []ForceProjectInput{
		{Name: "DevOps", Icon: "server", RootPath: localRoot},
		{Name: "ETL", Icon: "database", Connection: "etl@example.com", RootPath: "/srv/data"},
		{Name: "Marketing", Icon: "bullhorn", Connection: "marketing.example.com:2222", RootPath: "~/campaigns"},
	}
	var projectIDs []string
	for _, input := range inputs {
		project, updates, err := svc.SaveProject(ctx, input)
		if err != nil || project == nil || project.Version != 1 || len(updates) != 1 {
			t.Fatalf("save project: project=%+v updates=%+v err=%v", project, updates, err)
		}
		projectIDs = append(projectIDs, project.OID)
	}
	var profileIDs []string
	for _, title := range []string{"DevOps", "ETL", "Marketing"} {
		profile, updates, err := svc.SaveProfile(ctx, ForceProfileInput{
			Title: title, Icon: "robot", SystemPrompt: "Act as " + title, Adapter: "codex",
		})
		if err != nil || profile == nil || profile.Version != 1 || len(updates) != 1 {
			t.Fatalf("save profile: profile=%+v updates=%+v err=%v", profile, updates, err)
		}
		profileIDs = append(profileIDs, profile.OID)
	}
	if updates, err := svc.ArchiveProject(ctx, projectIDs[0], 1, true); err != nil || len(updates) != 1 {
		t.Fatalf("archive project: updates=%+v err=%v", updates, err)
	}
	if updates, err := svc.ArchiveProfile(ctx, profileIDs[0], 1, true); err != nil || len(updates) != 1 {
		t.Fatalf("archive profile: updates=%+v err=%v", updates, err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	catalog, err := (&ForceService{}).GetCatalog(ctx)
	if err != nil || len(catalog.Projects) != 3 || len(catalog.Profiles) != 3 {
		t.Fatalf("reopened catalog: %+v err=%v", catalog, err)
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("encode catalog: %v", err)
	}
	var wire struct {
		Projects []map[string]any `json:"projects"`
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	for _, project := range wire.Projects {
		if project["otype"] != waveobj.OType_ForceProject || project["version"] == nil {
			t.Fatalf("project wire object incomplete: %+v", project)
		}
	}
	for _, profile := range wire.Profiles {
		if profile["otype"] != waveobj.OType_ForceAgentProfile || profile["version"] == nil {
			t.Fatalf("profile wire object incomplete: %+v", profile)
		}
	}
	for _, project := range catalog.Projects {
		if project.OID == projectIDs[0] && (!project.Archived || project.Version != 2) {
			t.Fatalf("archived project changed after reopen: %+v", project)
		}
	}
	for _, profile := range catalog.Profiles {
		if profile.OID == profileIDs[0] && (!profile.Archived || profile.Version != 2 || profile.SystemPrompt != "Act as DevOps") {
			t.Fatalf("archived profile changed after reopen: %+v", profile)
		}
	}
	if _, err := svc.ArchiveProject(ctx, projectIDs[0], 2, false); err != nil {
		t.Fatalf("reactivate project: %v", err)
	}
	if _, err := svc.ArchiveProfile(ctx, profileIDs[0], 2, false); err != nil {
		t.Fatalf("reactivate profile: %v", err)
	}
	catalog, err = svc.GetCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var reactivatedProject, reactivatedProfile bool
	for _, project := range catalog.Projects {
		if project.OID == projectIDs[0] {
			reactivatedProject = !project.Archived && project.Version == 3
		}
	}
	for _, profile := range catalog.Profiles {
		if profile.OID == profileIDs[0] {
			reactivatedProfile = !profile.Archived && profile.Version == 3
		}
	}
	if !reactivatedProject || !reactivatedProfile {
		t.Fatalf("catalog still archived after reactivation: %+v err=%v", catalog, err)
	}
}

func TestCatalogRejectsConflictsAndInvalidInputs(t *testing.T) {
	ctx := catalogStore(t)
	svc := &ForceService{}
	root := t.TempDir()
	project, _, err := svc.SaveProject(ctx, ForceProjectInput{Name: "Original", Icon: "folder", RootPath: root})
	if err != nil {
		t.Fatal(err)
	}
	updated, _, err := svc.SaveProject(ctx, ForceProjectInput{ID: project.OID, ExpectedVersion: project.Version, Name: "Updated", Icon: "folder", RootPath: root})
	if err != nil || updated.Version != 2 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if _, _, err := svc.SaveProject(ctx, ForceProjectInput{ID: project.OID, ExpectedVersion: 1, Name: "Stale", Icon: "folder", RootPath: root}); err == nil {
		t.Fatal("stale edit accepted")
	}
	if _, err := svc.ArchiveProject(ctx, project.OID, 1, true); err == nil {
		t.Fatal("stale archive accepted")
	}
	profile, _, err := svc.SaveProfile(ctx, ForceProfileInput{Title: "DevOps", Icon: "robot", Adapter: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ArchiveProfile(ctx, profile.OID, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SaveProfile(ctx, ForceProfileInput{ID: profile.OID, ExpectedVersion: 1, Title: "Stale", Icon: "robot", Adapter: "codex"}); err == nil {
		t.Fatal("stale profile edit accepted")
	}
	invalidProjects := []ForceProjectInput{
		{Name: "", Icon: "folder", RootPath: root},
		{Name: "Bad", Icon: "other", RootPath: root},
		{Name: "Missing", Icon: "folder", RootPath: filepath.Join(root, "missing")},
		{Name: "File", Icon: "folder", RootPath: filepath.Join(root, "file")},
		{Name: "Bad SSH", Icon: "folder", Connection: "ssh://host", RootPath: "/srv"},
		{Name: "Bad password", Icon: "folder", Connection: "user:password@host", RootPath: "/srv"},
		{Name: "Bad local alias", Icon: "folder", Connection: "local", RootPath: "/srv"},
		{Name: "Bad WSL", Icon: "folder", Connection: "wsl://ubuntu", RootPath: "/srv"},
		{Name: "Bad remote path", Icon: "folder", Connection: "host", RootPath: "relative/path"},
		{Name: "Oversize host", Icon: "folder", Connection: strings.Repeat("h", 1025), RootPath: "/srv"},
		{Name: "Oversize path", Icon: "folder", RootPath: "/" + strings.Repeat("p", 4096)},
		{Name: "Bad version", Icon: "folder", RootPath: root, ExpectedVersion: 1},
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range invalidProjects {
		if _, _, err := svc.SaveProject(ctx, input); err == nil {
			t.Fatalf("invalid project accepted: %+v", input)
		}
	}
	invalidProfiles := []ForceProfileInput{
		{Title: "", Icon: "robot", Adapter: "codex"},
		{Title: "Bad", Icon: "other", Adapter: "codex"},
		{Title: "Bad", Icon: "robot", Adapter: "unknown"},
		{Title: "Bad", Icon: "robot", Adapter: "codex", SystemPrompt: strings.Repeat("x", 32001)},
	}
	for _, input := range invalidProfiles {
		if _, _, err := svc.SaveProfile(ctx, input); err == nil {
			t.Fatalf("invalid profile accepted: %+v", input)
		}
	}
	catalog, err := svc.GetCatalog(ctx)
	if err != nil || len(catalog.Projects) != 1 || len(catalog.Profiles) != 1 || catalog.Projects[0].Name != "Updated" {
		t.Fatalf("invalid edits changed catalog: %+v err=%v", catalog, err)
	}
}

func TestProjectEventFollowsCommit(t *testing.T) {
	ctx := catalogStore(t)
	client := &catalogEventClient{ctx: ctx}
	previousClient := wps.Broker.GetClient()
	wps.Broker.SetClient(client)
	wps.Broker.Subscribe("force-catalog-test", wps.SubscriptionRequest{Event: wps.Event_WaveObjUpdate, AllScopes: true})
	t.Cleanup(func() {
		wps.Broker.UnsubscribeAll("force-catalog-test")
		wps.Broker.SetClient(previousClient)
	})
	project, _, err := (&ForceService{}).SaveProject(ctx, ForceProjectInput{
		Name: "Event", Icon: "folder", RootPath: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.err != nil || len(client.events) != 1 || !client.events[0].HasScope("forceproject:"+project.OID) {
		t.Fatalf("event did not follow commit: events=%+v err=%v", client.events, client.err)
	}
}
