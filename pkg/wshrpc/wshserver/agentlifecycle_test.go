package wshserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func TestGenericBlockRPCRejectsAgentReplacementAndDestinationChange(t *testing.T) {
	dataDir := t.TempDir()
	wavebase.DataHome_VarCache = dataDir
	if err := os.MkdirAll(filepath.Join(dataDir, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tab := &waveobj.Tab{OID: uuid.NewString()}
	block := &waveobj.Block{OID: uuid.NewString(), ParentORef: waveobj.MakeORef(waveobj.OType_Tab, tab.OID).String(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term", waveobj.MetaKey_CmdCwd: "/checkout"}}
	agent := &waveobj.ForceAgentInstance{OID: uuid.NewString(), TabID: tab.OID, BlockID: block.OID}
	block.Meta["force:agentinstanceid"] = agent.OID
	for _, obj := range []waveobj.WaveObj{tab, block, agent} {
		if err := wstore.DBInsert(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	countBefore, err := wstore.DBGetCount[*waveobj.Block](ctx)
	if err != nil {
		t.Fatal(err)
	}
	server := &WshServer{}
	if _, err := server.CreateBlockCommand(ctx, wshrpc.CommandCreateBlockData{TabId: tab.OID, BlockDef: &waveobj.BlockDef{Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "launcher"}}, TargetBlockId: block.OID, TargetAction: "replace"}); err == nil {
		t.Fatal("generic replace accepted a Force terminal")
	}
	countAfter, err := wstore.DBGetCount[*waveobj.Block](ctx)
	if err != nil || countAfter != countBefore {
		t.Fatalf("replacement left an extra block: before=%d after=%d err=%v", countBefore, countAfter, err)
	}
	if err := server.SetMetaCommand(ctx, wshrpc.CommandSetMetaData{ORef: waveobj.MakeORef(waveobj.OType_Block, block.OID), Meta: waveobj.MetaMapType{waveobj.MetaKey_CmdCwd: "/elsewhere"}}); err == nil {
		t.Fatal("RPC changed Force cwd")
	}
	if err := server.SetMetaCommand(ctx, wshrpc.CommandSetMetaData{ORef: waveobj.MakeORef(waveobj.OType_Block, block.OID), Meta: waveobj.MetaMapType{"term:theme": "dark"}}); err != nil {
		t.Fatalf("cosmetic RPC rejected: %v", err)
	}
}
