package wcore

import (
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func TestGenericDeletionPreservesForceAgentBinding(t *testing.T) {
	ctx := initTestWStore(t)
	ws := &waveobj.Workspace{OID: uuid.NewString(), TabIds: []string{}}
	tab := &waveobj.Tab{OID: uuid.NewString(), LayoutState: uuid.NewString()}
	block := &waveobj.Block{OID: uuid.NewString(), ParentORef: waveobj.MakeORef(waveobj.OType_Tab, tab.OID).String(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}
	agent := &waveobj.ForceAgentInstance{OID: uuid.NewString(), TabID: tab.OID, BlockID: block.OID, Status: "prepared"}
	block.Meta["force:agentinstanceid"] = agent.OID
	tab.BlockIds = []string{block.OID}
	ws.TabIds = []string{tab.OID}
	for _, obj := range []waveobj.WaveObj{ws, tab, block, agent} {
		if err := wstore.DBInsert(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteBlock(ctx, block.OID, true); err == nil {
		t.Fatal("deleted Force block")
	}
	if _, err := DeleteTab(ctx, ws.OID, tab.OID, false); err == nil {
		t.Fatal("deleted Force tab")
	}
	if _, _, err := DeleteWorkspace(ctx, ws.OID, true); err == nil {
		t.Fatal("deleted Force workspace")
	}
	if err := UpdateWorkspaceTabIds(ctx, ws.OID, nil); err == nil {
		t.Fatal("removed Force tab from workspace without deleting it")
	}
	if _, err := wstore.DBMustGet[*waveobj.Block](ctx, block.OID); err != nil {
		t.Fatalf("Force block missing: %v", err)
	}
	if _, err := wstore.DBMustGet[*waveobj.Tab](ctx, tab.OID); err != nil {
		t.Fatalf("Force tab missing: %v", err)
	}
	if _, err := wstore.DBMustGet[*waveobj.Workspace](ctx, ws.OID); err != nil {
		t.Fatalf("Force workspace missing: %v", err)
	}
}

func TestWindowClosePreservesUnsavedForceWorkspace(t *testing.T) {
	ctx := initTestWStore(t)
	ws := &waveobj.Workspace{OID: uuid.NewString()}
	tab := &waveobj.Tab{OID: uuid.NewString()}
	block := &waveobj.Block{OID: uuid.NewString(), ParentORef: waveobj.MakeORef(waveobj.OType_Tab, tab.OID).String(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}
	agent := &waveobj.ForceAgentInstance{OID: uuid.NewString(), TabID: tab.OID, BlockID: block.OID}
	block.Meta["force:agentinstanceid"] = agent.OID
	tab.BlockIds, ws.TabIds = []string{block.OID}, []string{tab.OID}
	window := &waveobj.Window{OID: uuid.NewString(), WorkspaceId: ws.OID}
	client := &waveobj.Client{OID: uuid.NewString(), WindowIds: []string{window.OID}}
	for _, obj := range []waveobj.WaveObj{ws, tab, block, agent, window, client} {
		if err := wstore.DBInsert(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	if err := CloseWindow(ctx, window.OID, true); err != nil {
		t.Fatal(err)
	}
	if got, err := wstore.DBGet[*waveobj.Window](ctx, window.OID); err != nil || got != nil {
		t.Fatalf("window still present: %+v %v", got, err)
	}
	for _, obj := range []struct{ typ, id string }{{waveobj.OType_Workspace, ws.OID}, {waveobj.OType_Tab, tab.OID}, {waveobj.OType_Block, block.OID}} {
		got, err := wstore.DBGetORef(ctx, waveobj.MakeORef(obj.typ, obj.id))
		if err != nil || got == nil {
			t.Fatalf("agent context %s missing: %v", obj.typ, err)
		}
	}
}

func TestGenericBlockCreateAndPlainDelete(t *testing.T) {
	ctx := initTestWStore(t)
	tab := &waveobj.Tab{OID: uuid.NewString()}
	if err := wstore.DBInsert(ctx, tab); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateBlock(ctx, tab.OID, &waveobj.BlockDef{Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term", "force:agentinstanceid": uuid.NewString()}}, nil); err == nil {
		t.Fatal("generic create accepted copied Force marker")
	}
	plain, err := CreateBlock(ctx, tab.OID, &waveobj.BlockDef{Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteBlock(ctx, plain.OID, false); err != nil {
		t.Fatalf("plain terminal deletion rejected: %v", err)
	}
	if got, err := wstore.DBGet[*waveobj.Block](ctx, plain.OID); err != nil || got != nil {
		t.Fatalf("plain terminal still present: %+v %v", got, err)
	}
}

func TestForceBlockStructuralMetadataGuardAllowsCosmetics(t *testing.T) {
	ctx := initTestWStore(t)
	block := &waveobj.Block{OID: uuid.NewString(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term", waveobj.MetaKey_CmdCwd: "/checkout", waveobj.MetaKey_Connection: ""}}
	agent := &waveobj.ForceAgentInstance{OID: uuid.NewString(), BlockID: block.OID}
	block.Meta["force:agentinstanceid"] = agent.OID
	for _, obj := range []waveobj.WaveObj{block, agent} {
		if err := wstore.DBInsert(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CreateSubBlock(ctx, block.OID, &waveobj.BlockDef{Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}); err == nil {
		t.Fatal("generic subblock attached to Force terminal")
	}
	ref := waveobj.MakeORef(waveobj.OType_Block, block.OID)
	for _, meta := range []waveobj.MetaMapType{{waveobj.MetaKey_CmdCwd: "/other"}, {waveobj.MetaKey_Connection: "remote"}, {waveobj.MetaKey_View: "web"}, {"force:agentinstanceid": nil}} {
		if err := wstore.UpdateObjectMeta(ctx, ref, meta, false); err == nil {
			t.Fatalf("accepted structural metadata: %+v", meta)
		}
	}
	if err := wstore.UpdateObjectMeta(ctx, ref, waveobj.MetaMapType{"term:theme": "dark"}, false); err != nil {
		t.Fatalf("cosmetic metadata rejected: %v", err)
	}
	current, err := wstore.DBMustGet[*waveobj.Block](ctx, block.OID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Meta.GetString("term:theme", "") != "dark" || current.Meta.GetString(waveobj.MetaKey_CmdCwd, "") != "/checkout" {
		t.Fatalf("metadata changed unexpectedly: %+v", current.Meta)
	}
	current.JobId = uuid.NewString()
	if err := wstore.ValidateGenericObjectUpdate(ctx, current); err == nil {
		t.Fatal("accepted generic Force job binding change")
	}
	if err := wstore.ValidateGenericObjectUpdate(ctx, agent); err == nil {
		t.Fatal("accepted generic Force instance update")
	}
	plain := &waveobj.Block{OID: uuid.NewString(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}
	if err := wstore.DBInsert(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if err := wstore.UpdateObjectMeta(ctx, waveobj.MakeORef(waveobj.OType_Block, plain.OID), waveobj.MetaMapType{waveobj.MetaKey_CmdCwd: "/ordinary"}, false); err != nil {
		t.Fatalf("ordinary terminal metadata rejected: %v", err)
	}
}
