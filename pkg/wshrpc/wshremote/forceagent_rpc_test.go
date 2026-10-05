package wshremote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

func forceAgentRPCFixture(t *testing.T) (*ServerImpl, wshrpc.CommandRemoteForceAgentPrepareData) {
	t.Helper()
	local, _ := forcePrepareFixture(t)
	home := os.Getenv("HOME")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	uid, fingerprint, err := forceAgentHostUserContext()
	if err != nil {
		t.Fatal(err)
	}
	return &ServerImpl{InitialEnv: map[string]string{"HOME": home, "CLAUDE_CONFIG_DIR": os.Getenv("CLAUDE_CONFIG_DIR"), "PATH": os.Getenv("PATH")}}, wshrpc.CommandRemoteForceAgentPrepareData{
		Protocol: forceAgentPrepareProtocol, Nonce: "nonce-1234567890abcdef",
		Root: local.Root, Cwd: local.Cwd, Prompt: local.Prompt, PromptHash: local.PromptHash,
		ExpectedUID: uid, ExpectedContextFingerprint: fingerprint,
	}
}

func TestRemoteForceAgentContextIsVersionedAndReadOnly(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home never created")
	impl := &ServerImpl{InitialEnv: map[string]string{"HOME": home, "CLAUDE_CONFIG_DIR": filepath.Join(base, "history never read")}}
	data := wshrpc.CommandRemoteForceAgentContextData{Protocol: forceAgentPrepareProtocol, Nonce: "nonce-1234567890abcdef"}
	response, err := impl.RemoteForceAgentContextCommand(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	wantUID, wantFingerprint, err := forceAgentHostUserContextForEnv(impl.InitialEnv)
	if err != nil {
		t.Fatal(err)
	}
	if response.Protocol != data.Protocol || response.Nonce != data.Nonce || response.UID != wantUID || response.ContextFingerprint != wantFingerprint {
		t.Fatal("host-derived context or challenge changed")
	}
	if _, err := os.Lstat(home); !os.IsNotExist(err) {
		t.Fatal("context inspection created a home or stage")
	}
	bad := data
	bad.Protocol = "legacy"
	if _, err := impl.RemoteForceAgentContextCommand(context.Background(), bad); err == nil {
		t.Fatal("old context protocol accepted")
	}
	bad = data
	bad.Nonce = "../bad"
	if _, err := impl.RemoteForceAgentContextCommand(context.Background(), bad); err == nil {
		t.Fatal("unsafe context nonce accepted")
	}
	changed := &ServerImpl{InitialEnv: map[string]string{"HOME": home, "CLAUDE_CONFIG_DIR": filepath.Join(base, "changed")}}
	other, err := changed.RemoteForceAgentContextCommand(context.Background(), data)
	if err != nil || other.ContextFingerprint == response.ContextFingerprint {
		t.Fatal("changed effective job context was not observed")
	}
	if _, err := (&ServerImpl{}).RemoteForceAgentContextCommand(context.Background(), data); err == nil {
		t.Fatal("missing captured job environment accepted")
	}
}

func TestRemoteForceAgentPrepareReturnsHostEvidenceWithoutSpawn(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	cli := filepath.Join(filepath.Dir(req.Root), "fake bin", "claude")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n: > \"$0.spawned\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	rsp, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if rsp.Protocol != req.Protocol || rsp.Nonce != req.Nonce || rsp.UID != req.ExpectedUID || rsp.ContextFingerprint != req.ExpectedContextFingerprint {
		t.Fatal("version, nonce, or host context evidence changed")
	}
	if rsp.CanonicalRoot != req.Root || rsp.CanonicalCheckout != req.Root || rsp.CanonicalCWD != req.Cwd || !filepath.IsAbs(rsp.CLIPath) {
		t.Fatal("host-derived destination or CLI missing")
	}
	if rsp.PromptHash != req.PromptHash || rsp.ParentMode != 0700 || rsp.FileMode != 0600 || rsp.StagingMethod != "same-directory-rename" || rsp.CleanupHandle == "" {
		t.Fatal("private staging evidence missing")
	}
	if _, _, err := verifyForceAgentStaging(rsp.PromptPath, req.PromptHash); err != nil {
		t.Fatal(err)
	}
	receiptInfo, err := os.Lstat(filepath.Join(filepath.Dir(rsp.PromptPath), "receipt.json"))
	if err != nil || !receiptInfo.Mode().IsRegular() || receiptInfo.Mode().Perm() != 0600 {
		t.Fatal("private durable receipt missing")
	}
	if impl.JobManagerMap != nil {
		t.Fatal("preparation unexpectedly initialized job state")
	}
	if _, err := os.Lstat(cli + ".spawned"); !os.IsNotExist(err) {
		t.Fatal("fake CLI was executed")
	}
	clean := wshrpc.CommandRemoteForceAgentCleanupData{Protocol: req.Protocol, Nonce: req.Nonce, CleanupHandle: rsp.CleanupHandle}
	if _, err := impl.RemoteForceAgentCleanupCommand(context.Background(), clean); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(rsp.PromptPath)); !os.IsNotExist(err) {
		t.Fatal("cleanup left staged directory")
	}
}

func TestRemoteForceAgentPrepareRejectsVersionNonceAndChangedContext(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	for _, change := range []func(*wshrpc.CommandRemoteForceAgentPrepareData){
		func(r *wshrpc.CommandRemoteForceAgentPrepareData) { r.Protocol = "legacy" },
		func(r *wshrpc.CommandRemoteForceAgentPrepareData) { r.Nonce = "../bad" },
		func(r *wshrpc.CommandRemoteForceAgentPrepareData) { r.ExpectedUID = "changed" },
		func(r *wshrpc.CommandRemoteForceAgentPrepareData) { r.ExpectedContextFingerprint = "changed" },
		func(r *wshrpc.CommandRemoteForceAgentPrepareData) { r.PromptHash = strings.Repeat("0", 64) },
	} {
		bad := req
		change(&bad)
		if _, err := impl.RemoteForceAgentPrepareCommand(context.Background(), bad); err == nil {
			t.Fatal("invalid RPC preparation request accepted")
		}
	}
	impl.InitialEnv["CLAUDE_CONFIG_DIR"] = filepath.Join(t.TempDir(), "changed history")
	if _, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req); err == nil {
		t.Fatal("changed effective job context accepted")
	}
}

func TestRemoteForceAgentCleanupSurvivesReopenAndRejectsTampering(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	rsp, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	clean := wshrpc.CommandRemoteForceAgentCleanupData{Protocol: req.Protocol, Nonce: req.Nonce, CleanupHandle: rsp.CleanupHandle}
	reopened := &ServerImpl{InitialEnv: impl.InitialEnv}
	for _, handle := range []string{"../escape", filepath.Join(t.TempDir(), "outside"), strings.Repeat("a", 32)} {
		bad := clean
		bad.CleanupHandle = handle
		if _, err := reopened.RemoteForceAgentCleanupCommand(context.Background(), bad); err == nil && handle != strings.Repeat("a", 32) {
			t.Fatal("unsafe cleanup handle accepted")
		}
	}
	bad := clean
	bad.Nonce = "nonce-ffffffffffffffff"
	if _, err := reopened.RemoteForceAgentCleanupCommand(context.Background(), bad); err == nil {
		t.Fatal("wrong nonce cleaned receipt")
	}
	if _, err := reopened.RemoteForceAgentCleanupCommand(context.Background(), clean); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.RemoteForceAgentCleanupCommand(context.Background(), clean); err != nil {
		t.Fatal("duplicate cleanup should be idempotent")
	}
}

func TestRemoteForceAgentCleanupConcurrentAndPartialRetries(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	rsp, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	clean := wshrpc.CommandRemoteForceAgentCleanupData{Protocol: req.Protocol, Nonce: req.Nonce, CleanupHandle: rsp.CleanupHandle}
	var wg sync.WaitGroup
	errorsSeen := make(chan error, 24)
	for range cap(errorsSeen) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := (&ServerImpl{InitialEnv: impl.InitialEnv}).RemoteForceAgentCleanupCommand(context.Background(), clean)
			errorsSeen <- err
		}()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("concurrent cleanup failed: %v", err)
		}
	}
	if _, err := os.Lstat(filepath.Dir(rsp.PromptPath)); !os.IsNotExist(err) {
		t.Fatal("concurrent cleanup left stage behind")
	}

	rsp, err = impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	clean.CleanupHandle = rsp.CleanupHandle
	dir := filepath.Dir(rsp.PromptPath)
	if err := os.Remove(rsp.PromptPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "receipt.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := (&ServerImpl{InitialEnv: impl.InitialEnv}).RemoteForceAgentCleanupCommand(context.Background(), clean); err != nil {
		t.Fatalf("empty generated stage could not be recovered: %v", err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("empty stage still exists")
	}
}

func TestRemoteForceAgentCleanupMissingReceiptDoesNotDeleteUnexpectedFiles(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	rsp, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(rsp.PromptPath)
	if err := os.Remove(filepath.Join(dir, "receipt.json")); err != nil {
		t.Fatal(err)
	}
	unexpected := filepath.Join(dir, "unrelated.txt")
	if err := os.WriteFile(unexpected, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	clean := wshrpc.CommandRemoteForceAgentCleanupData{Protocol: req.Protocol, Nonce: req.Nonce, CleanupHandle: rsp.CleanupHandle}
	if _, err := impl.RemoteForceAgentCleanupCommand(context.Background(), clean); err == nil {
		t.Fatal("cleanup accepted missing receipt with files present")
	}
	if _, err := os.Lstat(unexpected); err != nil {
		t.Fatal("cleanup removed an unrelated file")
	}
}

func TestRemoteForceAgentCleanupRejectsSymlinkEscapeAndPromptTamper(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	rsp, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	clean := wshrpc.CommandRemoteForceAgentCleanupData{Protocol: req.Protocol, Nonce: req.Nonce, CleanupHandle: rsp.CleanupHandle}
	outside := t.TempDir()
	if err := os.Remove(rsp.PromptPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, rsp.PromptPath); err != nil {
		t.Fatal(err)
	}
	if _, err := impl.RemoteForceAgentCleanupCommand(context.Background(), clean); err == nil {
		t.Fatal("cleanup accepted symlinked prompt")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("cleanup escaped private stage")
	}
	if err := os.Remove(rsp.PromptPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rsp.PromptPath, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := impl.RemoteForceAgentCleanupCommand(context.Background(), clean); err == nil {
		t.Fatal("cleanup accepted tampered prompt")
	}
}

func TestRemoteForceAgentCleanupRejectsSymlinkedStageAndReceipt(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	rsp, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	clean := wshrpc.CommandRemoteForceAgentCleanupData{Protocol: req.Protocol, Nonce: req.Nonce, CleanupHandle: rsp.CleanupHandle}
	dir := filepath.Dir(rsp.PromptPath)
	parked := dir + "-parked"
	if err := os.Rename(dir, parked); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := impl.RemoteForceAgentCleanupCommand(context.Background(), clean); err == nil {
		t.Fatal("symlinked stage accepted")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("cleanup escaped private stage")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parked, dir); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"protocol":"tampered"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := impl.RemoteForceAgentCleanupCommand(context.Background(), clean); err == nil {
		t.Fatal("tampered receipt accepted")
	}
}

func TestRemoteForceAgentPreparePreservesCancellationAndSafeErrors(t *testing.T) {
	impl, req := forceAgentRPCFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := impl.RemoteForceAgentPrepareCommand(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	secret := []byte("SECRET_SENTINEL")
	req.Prompt = secret
	hash := sha256.Sum256(secret)
	req.PromptHash = hex.EncodeToString(hash[:])
	req.Root = filepath.Join(t.TempDir(), "missing SECRET_SENTINEL")
	if _, err := impl.RemoteForceAgentPrepareCommand(context.Background(), req); err == nil || strings.Contains(err.Error(), string(secret)) {
		t.Fatal("raw prompt or path leaked in error")
	}
}
