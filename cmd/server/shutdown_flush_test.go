package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestShutdownFlushPersistsBothStores(t *testing.T) {
	var files, secrets atomic.Bool
	err := flushShutdownStores(context.Background(), func(context.Context) error {
		files.Store(true)
		return nil
	}, func(context.Context) error {
		secrets.Store(true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !files.Load() || !secrets.Load() {
		t.Fatal("shutdown did not persist both stores")
	}
}

func TestShutdownFlushDoesNotLetSlowFileFlushStarveSecrets(t *testing.T) {
	secretRan := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := flushShutdownStores(ctx, func(ctx context.Context) error {
		select {
		case <-secretRan:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}, func(context.Context) error { close(secretRan); return nil })
	if err != nil {
		t.Fatalf("secret flush could not run while file flush was waiting: %v", err)
	}
}

func TestShutdownFlushReportsSecretFailure(t *testing.T) {
	want := errors.New("test persistence failure")
	err := flushShutdownStores(context.Background(), func(context.Context) error { return nil }, func(context.Context) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want secret failure", err)
	}
}

func TestShutdownFlushAttemptsSecretsAfterFileFailure(t *testing.T) {
	want := errors.New("test file failure")
	secretCalled := false
	err := flushShutdownStores(context.Background(), func(context.Context) error { return want }, func(context.Context) error {
		secretCalled = true
		return nil
	})
	if !secretCalled || !errors.Is(err, want) {
		t.Fatalf("shutdown skipped secrets after file failure: err=%v, secretCalled=%v", err, secretCalled)
	}
}
