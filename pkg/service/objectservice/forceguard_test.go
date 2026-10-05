package objectservice

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func TestGenericObjectServiceCannotChangeForceBinding(t *testing.T) {
	dataDir := t.TempDir()
	wavebase.DataHome_VarCache = dataDir
	if err := os.MkdirAll(filepath.Join(dataDir, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tab := &waveobj.Tab{OID: uuid.NewString(), LayoutState: uuid.NewString()}
	block := &waveobj.Block{OID: uuid.NewString(), ParentORef: waveobj.MakeORef(waveobj.OType_Tab, tab.OID).String(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term", waveobj.MetaKey_CmdCwd: "/checkout"}}
	agent := &waveobj.ForceAgentInstance{OID: uuid.NewString(), TabID: tab.OID, BlockID: block.OID}
	block.Meta["force:agentinstanceid"] = agent.OID
	for _, obj := range []waveobj.WaveObj{tab, block, agent} {
		if err := wstore.DBInsert(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	svc := &ObjectService{}
	block.Meta[waveobj.MetaKey_CmdCwd] = "/elsewhere"
	if _, err := svc.UpdateObject(waveobj.UIContext{}, block, true); err == nil {
		t.Fatal("full-object update changed Force destination")
	}
	if _, err := svc.UpdateObjectMeta(waveobj.UIContext{}, waveobj.MakeORef(waveobj.OType_Block, block.OID).String(), waveobj.MetaMapType{waveobj.MetaKey_Connection: "remote"}); err == nil {
		t.Fatal("metadata update changed Force connection")
	}
	delete(block.Meta, waveobj.MetaKey_CmdCwd)
	block.Meta[waveobj.MetaKey_CmdCwd] = "/checkout"
	block.Meta["frame:title"] = "Visible title"
	if _, err := svc.UpdateObject(waveobj.UIContext{}, block, true); err != nil {
		t.Fatalf("cosmetic update rejected: %v", err)
	}
	tab.LayoutState = uuid.NewString()
	if _, err := svc.UpdateObject(waveobj.UIContext{}, tab, true); err == nil {
		t.Fatal("changed Force tab layout identity")
	}
	if _, err := svc.UpdateObject(waveobj.UIContext{}, agent, true); err == nil {
		t.Fatal("updated Force instance through generic object service")
	}
}
