// SPDX-License-Identifier: Apache-2.0
package wshserver

import (
	"context"
	"errors"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
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
