// SPDX-License-Identifier: Apache-2.0
package filestore

import (
	"bytes"
	"context"
	"errors"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"testing"
	"time"
)

func TestFlushForUpdatePersistsPendingWrites(t *testing.T) {
	initDb(t)
	defer cleanupDb(t)
	ctx := context.Background()
	if err := WFS.MakeFile(ctx, "update", "note", nil, wshrpc.FileOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := WFS.WriteFile(ctx, "update", "note", []byte("saved before restart")); err != nil {
		t.Fatal(err)
	}
	if err := WFS.FlushForUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	WFS.clearCache()
	_, data, err := WFS.ReadFile(ctx, "update", "note")
	if err != nil || !bytes.Equal(data, []byte("saved before restart")) {
		t.Fatalf("disk contents=%q err=%v", data, err)
	}
}

func TestFlushForUpdateCannotSucceedWhileAnotherFlushIsStuck(t *testing.T) {
	initDb(t)
	defer cleanupDb(t)
	WFS.setIsFlushing(true)
	defer WFS.setIsFlushing(false)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := WFS.FlushForUpdate(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got %v", err)
	}
}
