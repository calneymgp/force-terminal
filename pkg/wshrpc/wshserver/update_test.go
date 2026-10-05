// SPDX-License-Identifier: Apache-2.0
package wshserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	"testing"
)

func TestUpdateRPCRejectsUncoordinatedRequests(t *testing.T) {
	server := &WshServer{}
	if err := server.FlushForUpdateCommand(context.Background(), wshrpc.CommandFlushForUpdateData{}); err == nil {
		t.Fatal("flush acknowledgement must require Electron coordinator")
	}
	if _, err := server.GetUpdateBlockersCommand(context.Background(), wshrpc.CommandGetUpdateBlockersData{}); err == nil {
		t.Fatal("an uncoordinated caller cannot assert idle terminals")
	}
}

func TestUpdateIncludesPersistedAgentWithoutLoadedController(t *testing.T) {
	dir := t.TempDir()
	wavebase.DataHome_VarCache = dir
	if err := os.MkdirAll(filepath.Join(dir, wavebase.WaveDBDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := wstore.InitWStore(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := &waveobj.ForceAgentInstance{OID: uuid.NewString(), BlockID: uuid.NewString(),
		TitleSnapshot: "DevOps", Status: "uncertain", WriterLeaseKey: "test-owned-lease"}
	if err := wstore.DBInsert(ctx, agent); err != nil {
		t.Fatal(err)
	}
	check := func(want bool) {
		t.Helper()
		reasons, err := collectUpdateBlockers(ctx, wshrpc.CommandGetUpdateBlockersData{VerifiedIdleBlocks: []string{agent.BlockID}})
		if err != nil {
			t.Fatal(err)
		}
		blocked := false
		for _, reason := range reasons {
			if strings.Contains(reason, "Agente DevOps") {
				blocked = true
			}
		}
		if blocked != want {
			t.Fatalf("persisted agent blocked=%v; reasons=%v", blocked, reasons)
		}
	}
	check(true)
	agent.Status, agent.WriterLeaseKey = "exited", ""
	if err := wstore.DBUpdate(ctx, agent); err != nil {
		t.Fatal(err)
	}
	check(false)
}

func TestUpdateFlushStopsBeforeSecretsWhenFileFlushFails(t *testing.T) {
	want := errors.New("test file flush failure")
	secretCalled := false
	err := flushUpdateStores(context.Background(), func(context.Context) error { return want }, func(context.Context) error {
		secretCalled = true
		return nil
	})
	if !errors.Is(err, want) || secretCalled {
		t.Fatalf("file failure must stop update flush: err=%v, secretCalled=%v", err, secretCalled)
	}
}

func TestUpdateFlushPropagatesSecretFailure(t *testing.T) {
	want := errors.New("test secret flush failure")
	fileCalled := false
	err := flushUpdateStores(context.Background(), func(context.Context) error { fileCalled = true; return nil }, func(context.Context) error { return want })
	if !fileCalled || !errors.Is(err, want) {
		t.Fatalf("secret failure must reject update: err=%v, fileCalled=%v", err, fileCalled)
	}
}
