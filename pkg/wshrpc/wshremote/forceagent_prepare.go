// SPDX-License-Identifier: Apache-2.0
package wshremote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// This is a host-local preparation boundary, not an RPC method. The caller
// cannot attest SSH host-key identity or connection binding through it.
type forceAgentHostPrepareRequest struct {
	Root                       string
	Cwd                        string
	Prompt                     []byte
	PromptHash                 string
	ExpectedUID                string
	ExpectedContextFingerprint string
}

type forceAgentHostPrepared struct {
	UID                string
	ContextFingerprint string
	CanonicalRoot      string
	CanonicalCheckout  string
	CanonicalCWD       string
	CLIPath            string
	PromptPath         string
	PromptHash         string
	ParentMode         os.FileMode
	FileMode           os.FileMode
	StagingMethod      string
	Cleanup            func() error
}

// Only system-call failure hooks needed for deterministic local tests. There is
// no caller-supplied permission or atomicity flag.
type forceAgentPrepareOps struct {
	rename    func(string, string) error
	removeAll func(string) error
}

func prepareForceAgentHost(ctx context.Context, req forceAgentHostPrepareRequest) (*forceAgentHostPrepared, error) {
	return prepareForceAgentHostWithOps(ctx, req, forceAgentPrepareOps{})
}

func prepareForceAgentHostWithOps(ctx context.Context, req forceAgentHostPrepareRequest, ops forceAgentPrepareOps) (*forceAgentHostPrepared, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ops.rename == nil {
		ops.rename = os.Rename
	}
	if ops.removeAll == nil {
		ops.removeAll = os.RemoveAll
	}
	if len(req.Prompt) > 1024*1024 || len(req.PromptHash) != 64 {
		return nil, errors.New("invalid agent prompt snapshot")
	}
	wantHash := sha256.Sum256(req.Prompt)
	if hex.EncodeToString(wantHash[:]) != req.PromptHash {
		return nil, errors.New("agent prompt snapshot hash mismatch")
	}
	root, checkout, cwd, err := resolveForceAgentHostDestination(req.Root, req.Cwd)
	if err != nil {
		return nil, err
	}
	uid, contextFingerprint, err := forceAgentHostUserContext()
	if err != nil {
		return nil, err
	}
	if (req.ExpectedUID != "" && req.ExpectedUID != uid) ||
		(req.ExpectedContextFingerprint != "" && req.ExpectedContextFingerprint != contextFingerprint) {
		return nil, errors.New("agent execution context changed")
	}
	cliPath, err := findForceAgentClaude()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	promptPath, parentMode, fileMode, cleanup, err := stageForceAgentPrompt(ctx, req.Prompt, req.PromptHash, ops)
	if err != nil {
		return nil, err
	}
	return &forceAgentHostPrepared{
		UID: uid, ContextFingerprint: contextFingerprint,
		CanonicalRoot: root, CanonicalCheckout: checkout, CanonicalCWD: cwd,
		CLIPath: cliPath, PromptPath: promptPath, PromptHash: req.PromptHash,
		ParentMode: parentMode, FileMode: fileMode, StagingMethod: "same-directory-rename",
		Cleanup: cleanup,
	}, nil
}

func validForceAgentHostPath(path string) bool {
	return filepath.IsAbs(path) && !strings.ContainsAny(path, "\x00\r\n")
}

func resolveForceAgentHostDestination(rootPath, cwdPath string) (string, string, string, error) {
	if !validForceAgentHostPath(rootPath) || !validForceAgentHostPath(cwdPath) {
		return "", "", "", errors.New("agent destination unavailable")
	}
	root, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", "", "", errors.New("agent destination unavailable")
	}
	cwd, err := filepath.EvalSymlinks(cwdPath)
	if err != nil {
		return "", "", "", errors.New("agent destination unavailable")
	}
	for _, path := range []string{root, cwd} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return "", "", "", errors.New("agent destination unavailable")
		}
	}
	checkout := root
	for dir := root; ; dir = filepath.Dir(dir) {
		marker, err := os.Lstat(filepath.Join(dir, ".git"))
		if err == nil {
			if !marker.IsDir() && !marker.Mode().IsRegular() {
				return "", "", "", errors.New("agent checkout marker unavailable")
			}
			checkout = dir
			break
		}
		if !os.IsNotExist(err) {
			return "", "", "", errors.New("agent checkout marker unavailable")
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	rel, err := filepath.Rel(checkout, cwd)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", "", errors.New("agent cwd outside checkout")
	}
	return filepath.Clean(root), filepath.Clean(checkout), filepath.Clean(cwd), nil
}

func forceAgentHostUserContext() (string, string, error) {
	current, err := user.Current()
	if err != nil || current.Uid == "" || current.HomeDir == "" {
		return "", "", errors.New("agent user context unavailable")
	}
	home := os.Getenv("HOME")
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if !validForceAgentHostPath(home) || configDir != "" && !validForceAgentHostPath(configDir) {
		return "", "", errors.New("agent CLI history context unavailable")
	}
	sum := sha256.Sum256([]byte(current.Uid + "\x00" + current.HomeDir + "\x00" + home + "\x00" + configDir))
	return current.Uid, hex.EncodeToString(sum[:]), nil
}

func findForceAgentClaude() (string, error) {
	var candidates []string
	if path, err := exec.LookPath("claude"); err == nil {
		candidates = append(candidates, path)
	}
	if home := os.Getenv("HOME"); validForceAgentHostPath(home) {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "claude"))
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/opt/homebrew/bin/claude", "/usr/local/bin/claude")
	}
	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate)
		if err != nil || !validForceAgentHostPath(path) || filepath.Base(path) != "claude" {
			continue
		}
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", errors.New("claude CLI unavailable on execution host")
}

func stageForceAgentPrompt(ctx context.Context, prompt []byte, wantHash string, ops forceAgentPrepareOps) (string, os.FileMode, os.FileMode, func() error, error) {
	dir, err := os.MkdirTemp("", "force-agent-prompt-")
	if err != nil {
		return "", 0, 0, nil, errors.New("agent private staging failed")
	}
	cleanup := func() error { return ops.removeAll(dir) }
	fail := func(cause error) (string, os.FileMode, os.FileMode, func() error, error) {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return "", 0, 0, nil, errors.New("agent private staging cleanup failed")
		}
		if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
			return "", 0, 0, nil, cause
		}
		return "", 0, 0, nil, errors.New("agent private staging failed")
	}
	parent, err := os.Lstat(dir)
	if err != nil || !parent.IsDir() || parent.Mode().Perm() != 0700 {
		return fail(err)
	}
	tmp, err := os.CreateTemp(dir, ".prompt-")
	if err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(prompt); err != nil {
		_ = tmp.Close()
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	path := filepath.Join(dir, "prompt.txt")
	if err := ops.rename(tmp.Name(), path); err != nil {
		return fail(err)
	}
	parentMode, fileMode, err := verifyForceAgentStaging(path, wantHash)
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return path, parentMode, fileMode, cleanup, nil
}

func verifyForceAgentStaging(path, wantHash string) (os.FileMode, os.FileMode, error) {
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode().Perm() != 0700 {
		return 0, 0, errors.New("agent private staging verification failed")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return 0, 0, errors.New("agent private staging verification failed")
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, errors.New("agent private staging verification failed")
	}
	sum := sha256.New()
	_, copyErr := io.Copy(sum, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || hex.EncodeToString(sum.Sum(nil)) != wantHash {
		return 0, 0, errors.New("agent private staging verification failed")
	}
	return parent.Mode().Perm(), info.Mode().Perm(), nil
}
