// SPDX-License-Identifier: Apache-2.0
package wshserver

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

func TestGenericDestroyRPCRejectsDurableAgentBinding(t *testing.T) {
	dataDir := t.TempDir()
	wavebase.DataHome_VarCache = dataDir
	if err := os.MkdirAll(filepath.Join(dataDir, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	instance := &waveobj.ForceAgentInstance{OID: uuid.NewString(), TabID: uuid.NewString(), BlockID: uuid.NewString(), Status: "prepared"}
	if err := wstore.DBInsert(ctx, instance); err != nil {
		t.Fatal(err)
	}
	block := &waveobj.Block{OID: instance.BlockID, ParentORef: waveobj.MakeORef(waveobj.OType_Tab, instance.TabID).String(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term", "force:agentinstanceid": instance.OID}}
	if err := wstore.DBInsert(ctx, block); err != nil {
		t.Fatal(err)
	}
	if err := (&WshServer{}).ControllerDestroyCommand(ctx, block.OID); err == nil {
		t.Fatal("RPC accepted generic destruction of a Force agent")
	}
	if _, err := wstore.DBMustGet[*waveobj.Block](ctx, block.OID); err != nil {
		t.Fatalf("agent block was removed: %v", err)
	}
	plain := &waveobj.Block{OID: uuid.NewString(), Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"}}
	if err := wstore.DBInsert(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if err := (&WshServer{}).ControllerDestroyCommand(ctx, plain.OID); err != nil {
		t.Fatalf("normal terminal destroy was rejected: %v", err)
	}
}
