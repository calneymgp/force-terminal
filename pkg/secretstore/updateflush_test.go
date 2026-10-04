package secretstore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// The fake captures the snapshot at the persistence boundary. No Electron,
// credential service, or real profile is involved.
func isolatedSecretStore(t *testing.T, persist func(context.Context, map[string]string) error) {
	t.Helper()
	lock.Lock()
	oldInitialized, oldSecrets, oldRequests := initialized, secrets, writeRequestChan
	oldPending, oldPersisted, oldPersist := pendingWriteGeneration, persistedWriteGeneration, persistSecretSnapshot
	initialized = true
	secrets = make(map[string]string)
	writeRequestChan = nil
	pendingWriteGeneration, persistedWriteGeneration = 0, 0
	persistSecretSnapshot = persist
	lock.Unlock()
	t.Cleanup(func() {
		lock.Lock()
		initialized, secrets, writeRequestChan = oldInitialized, oldSecrets, oldRequests
		pendingWriteGeneration, persistedWriteGeneration, persistSecretSnapshot = oldPending, oldPersisted, oldPersist
		lock.Unlock()
	})
}

func TestFlushForUpdateDoesNotInitializeUnusedStore(t *testing.T) {
	lock.Lock()
	wasInitialized := initialized
	initialized = false
	lock.Unlock()
	t.Cleanup(func() { lock.Lock(); initialized = wasInitialized; lock.Unlock() })
	if err := FlushForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	lock.Lock()
	defer lock.Unlock()
	if initialized {
		t.Fatal("flush initialized an unused secret store")
	}
}

func TestFlushForUpdatePersistsPendingSecretImmediately(t *testing.T) {
	var saved map[string]string
	isolatedSecretStore(t, func(_ context.Context, snapshot map[string]string) error { saved = snapshot; return nil })
	if err := SetSecret("TestKey", "test-value"); err != nil {
		t.Fatal(err)
	}
	if err := FlushForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if saved["TestKey"] != "test-value" {
		t.Fatal("flush returned before pending secret was persisted")
	}
}

func TestFlushForUpdateWaitsForInFlightWriteAndNewerMutation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var writesMu sync.Mutex
	var saved []string
	isolatedSecretStore(t, func(_ context.Context, snapshot map[string]string) error {
		if snapshot["TestKey"] == "first" {
			close(started)
			<-release
		}
		writesMu.Lock()
		saved = append(saved, snapshot["TestKey"])
		writesMu.Unlock()
		return nil
	})
	if err := SetSecret("TestKey", "first"); err != nil {
		t.Fatal(err)
	}
	backgroundDone := make(chan error, 1)
	go func() { backgroundDone <- writeSecretsToFile(context.Background()) }()
	<-started
	if err := SetSecret("TestKey", "second"); err != nil {
		t.Fatal(err)
	}
	flushDone := make(chan error, 1)
	go func() { flushDone <- FlushForUpdate(context.Background()) }()
	select {
	case <-flushDone:
		t.Fatal("flush returned while write was in flight")
	default:
	}
	close(release)
	if err := <-backgroundDone; err != nil {
		t.Fatal(err)
	}
	if err := <-flushDone; err != nil {
		t.Fatal(err)
	}
	writesMu.Lock()
	defer writesMu.Unlock()
	if len(saved) != 2 || saved[0] != "first" || saved[1] != "second" {
		t.Fatalf("flush missed newer mutation: %v", saved)
	}
}

func TestFlushForUpdateIncludesMutationDuringFlush(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var saved string
	isolatedSecretStore(t, func(_ context.Context, snapshot map[string]string) error {
		if snapshot["TestKey"] == "first" {
			close(started)
			<-release
		}
		saved = snapshot["TestKey"]
		return nil
	})
	if err := SetSecret("TestKey", "first"); err != nil {
		t.Fatal(err)
	}
	flushDone := make(chan error, 1)
	go func() { flushDone <- FlushForUpdate(context.Background()) }()
	<-started
	if err := SetSecret("TestKey", "second"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-flushDone; err != nil {
		t.Fatal(err)
	}
	if saved != "second" {
		t.Fatal("flush returned while a concurrent mutation remained pending")
	}
}

func TestFlushForUpdatePropagatesWriteErrorAndContextTimeout(t *testing.T) {
	want := errors.New("test storage failure")
	isolatedSecretStore(t, func(_ context.Context, _ map[string]string) error { return want })
	if err := SetSecret("TestKey", "value"); err != nil {
		t.Fatal(err)
	}
	if err := FlushForUpdate(context.Background()); !errors.Is(err, want) {
		t.Fatalf("got %v, want storage error", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	<-ctx.Done()
	if err := FlushForUpdate(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline", err)
	}
}

func TestFlushForUpdateTimesOutWaitingForConcurrentWrite(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	isolatedSecretStore(t, func(context.Context, map[string]string) error {
		close(started)
		<-release
		return nil
	})
	if err := SetSecret("TestKey", "test-value"); err != nil {
		t.Fatal(err)
	}
	writerDone := make(chan error, 1)
	go func() { writerDone <- writeSecretsToFile(context.Background()) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := FlushForUpdate(ctx)
	close(release)
	if writeErr := <-writerDone; writeErr != nil {
		t.Fatal(writeErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("concurrent write must not acknowledge timed out flush: %v", err)
	}
}

func TestFailedSecretWriteRemainsPendingForRetry(t *testing.T) {
	writes := 0
	isolatedSecretStore(t, func(context.Context, map[string]string) error {
		writes++
		if writes == 1 {
			return errors.New("test storage failure")
		}
		return nil
	})
	if err := SetSecret("TestKey", "test-value"); err != nil {
		t.Fatal(err)
	}
	if err := FlushForUpdate(context.Background()); err == nil {
		t.Fatal("failed write must not acknowledge persistence")
	}
	if err := FlushForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if writes != 2 {
		t.Fatal("retry skipped the pending secret write")
	}
}
