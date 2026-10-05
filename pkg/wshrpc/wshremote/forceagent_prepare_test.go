package wshremote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func forcePrepareFixture(t *testing.T) (forceAgentHostPrepareRequest, string) {
	t.Helper()
	base := t.TempDir()
	base, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(base, "fake bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(bin, "claude")
	if err := os.WriteFile(cli, []byte("fake executable; never run"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("TMPDIR", filepath.Join(base, "temporary"))
	if err := os.Mkdir(os.Getenv("TMPDIR"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home one"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "history one"))
	root := filepath.Join(base, "projeto ü com espaço")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(root, "src com espaço")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	prompt := []byte("snapshot confidencial ü")
	hash := sha256.Sum256(prompt)
	return forceAgentHostPrepareRequest{Root: root, Cwd: cwd, Prompt: prompt, PromptHash: hex.EncodeToString(hash[:])}, cli
}

func TestForceAgentHostPrepareStagesPrivateSnapshotWithoutSpawn(t *testing.T) {
	req, cli := forcePrepareFixture(t)
	alias := filepath.Join(filepath.Dir(req.Root), "alias")
	if err := os.Symlink(req.Root, alias); err != nil {
		t.Fatal(err)
	}
	req.Root = alias
	prepared, err := prepareForceAgentHost(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prepared.Cleanup(); err != nil {
			t.Fatal(err)
		}
	})
	if prepared.CanonicalCheckout != filepath.Dir(req.Cwd) || prepared.CanonicalCWD != req.Cwd || prepared.CLIPath != cli {
		t.Fatalf("wrong canonical destination or CLI: %#v", prepared)
	}
	if prepared.StagingMethod != "same-directory-rename" || prepared.ParentMode != 0700 || prepared.FileMode != 0600 || prepared.PromptHash != req.PromptHash {
		t.Fatalf("private staging evidence missing: %#v", prepared)
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if prepared.UID != current.Uid || prepared.ContextFingerprint == "" {
		t.Fatal("host user/context evidence missing")
	}
	req.Prompt[0] = 'X'
	got, err := os.ReadFile(prepared.PromptPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "snapshot confidencial ü" {
		t.Fatal("staged prompt changed with caller's slice")
	}
}

func TestForceAgentHostPrepareRejectsChangedContextAndUnsafeDestination(t *testing.T) {
	req, _ := forcePrepareFixture(t)
	prepared, err := prepareForceAgentHost(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Cleanup(); err != nil {
		t.Fatal(err)
	}
	req.ExpectedContextFingerprint = prepared.ContextFingerprint
	changedHistory := filepath.Join(filepath.Dir(req.Root), "history two")
	t.Setenv("CLAUDE_CONFIG_DIR", changedHistory)
	if _, err := prepareForceAgentHost(context.Background(), req); err == nil || strings.Contains(err.Error(), changedHistory) {
		t.Fatal("history context mismatch not safely rejected")
	}
	req.ExpectedContextFingerprint = ""
	req.ExpectedUID = "wrong-uid"
	if _, err := prepareForceAgentHost(context.Background(), req); err == nil {
		t.Fatal("UID mismatch accepted")
	}
	req.ExpectedUID = ""
	marker := filepath.Join(req.Root, ".git")
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(req.Root, "src com espaço"), marker); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareForceAgentHost(context.Background(), req); err == nil {
		t.Fatal("symlink git marker accepted")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(marker, 0700); err != nil {
		t.Fatal(err)
	}
	req.Cwd = filepath.Dir(req.Root)
	if _, err := prepareForceAgentHost(context.Background(), req); err == nil {
		t.Fatal("cwd outside checkout accepted")
	}
}

func TestForceAgentHostPrepareRejectsBadHashMissingCLIAndCancellation(t *testing.T) {
	req, cli := forcePrepareFixture(t)
	req.PromptHash = strings.Repeat("0", 64)
	if _, err := prepareForceAgentHost(context.Background(), req); err == nil {
		t.Fatal("bad snapshot hash accepted")
	}
	hash := sha256.Sum256(req.Prompt)
	req.PromptHash = hex.EncodeToString(hash[:])
	if err := os.Chmod(cli, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareForceAgentHost(context.Background(), req); err == nil {
		t.Fatal("non-executable CLI accepted")
	}
	if err := os.Chmod(cli, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareForceAgentHost(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestForceAgentHostPrepareChecksModesReadbackAndCleanupErrors(t *testing.T) {
	req, _ := forcePrepareFixture(t)
	prepared, err := prepareForceAgentHost(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(prepared.PromptPath, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyForceAgentStaging(prepared.PromptPath, req.PromptHash); err == nil {
		t.Fatal("unsafe file mode accepted")
	}
	if err := os.Chmod(prepared.PromptPath, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prepared.PromptPath, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyForceAgentStaging(prepared.PromptPath, req.PromptHash); err == nil {
		t.Fatal("readback hash mismatch accepted")
	}
	if err := prepared.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifyForceAgentStaging(prepared.PromptPath, req.PromptHash); err == nil {
		t.Fatal("removed staged file unexpectedly verified")
	}
	injected := errors.New("injected cleanup failure")
	prepared, err = prepareForceAgentHostWithOps(context.Background(), req, forceAgentPrepareOps{removeAll: func(string) error { return injected }})
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Cleanup(); !errors.Is(err, injected) {
		t.Fatalf("cleanup failure lost: %v", err)
	}
	if err := os.RemoveAll(filepath.Dir(prepared.PromptPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareForceAgentHostWithOps(context.Background(), req, forceAgentPrepareOps{rename: func(string, string) error { return errors.New("injected rename failure") }}); err == nil {
		t.Fatal("failed atomic rename accepted")
	}
	for _, tc := range []struct {
		name   string
		mutate func(string) error
	}{
		{"unsafe mode", func(path string) error { return os.Chmod(path, 0644) }},
		{"tampered bytes", func(path string) error { return os.WriteFile(path, []byte("tampered"), 0600) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stagedPath string
			_, err := prepareForceAgentHostWithOps(context.Background(), req, forceAgentPrepareOps{rename: func(src, dst string) error {
				if err := os.Rename(src, dst); err != nil {
					return err
				}
				stagedPath = dst
				return tc.mutate(dst)
			}})
			if err == nil {
				t.Fatal("unverified staging accepted")
			}
			if stagedPath == "" {
				t.Fatal("test did not reach staging")
			}
			if _, err := os.Stat(filepath.Dir(stagedPath)); !os.IsNotExist(err) {
				t.Fatalf("failed staging directory remains: %v", err)
			}
		})
	}
}

type deadlineAfterRenameContext struct {
	context.Context
	expired *bool
}

func (c deadlineAfterRenameContext) Err() error {
	if *c.expired {
		return context.DeadlineExceeded
	}
	return nil
}

func TestForceAgentHostPreparePreservesDeadlineAndCleansStage(t *testing.T) {
	req, _ := forcePrepareFixture(t)
	expired := false
	ctx := deadlineAfterRenameContext{Context: context.Background(), expired: &expired}
	var stagedPath string
	_, err := prepareForceAgentHostWithOps(ctx, req, forceAgentPrepareOps{rename: func(src, dst string) error {
		if err := os.Rename(src, dst); err != nil {
			return err
		}
		stagedPath = dst
		expired = true
		return nil
	}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline lost after staging: %v", err)
	}
	if stagedPath == "" {
		t.Fatal("test did not reach atomic rename")
	}
	if _, err := os.Stat(filepath.Dir(stagedPath)); !os.IsNotExist(err) {
		t.Fatalf("canceled staging directory remains: %v", err)
	}
}
