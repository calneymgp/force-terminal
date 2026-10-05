// SPDX-License-Identifier: Apache-2.0
package forceservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

const ClaudeAdapterVersion = "claude-code-cli-v1"

type ClaudeLaunchMode string

const (
	ClaudeLaunchStart  ClaudeLaunchMode = "start"
	ClaudeLaunchResume ClaudeLaunchMode = "resume"
)

// ClaudeLaunchSpec carries a persisted snapshot, not a live profile reference.
// CLIPath is an internal binary override for controlled tests; its basename must
// remain claude. No environment or history override is accepted here.
type ClaudeLaunchSpec struct {
	SessionID      string
	PromptSnapshot string
	PromptHash     string
	Connection     string
	UserContext    string
	Cwd            string
	Mode           ClaudeLaunchMode
	CLIPath        string
	RemoteStager   ClaudeRemoteStager
}

type ClaudeRemoteStageRequest struct {
	Connection  string
	UserContext string
	Cwd         string
	Prompt      []byte
	PromptHash  string
}

// ClaudeRemoteStager is a deliberately narrow boundary. Its implementation must
// stage on the execution host and user with a private 0700 directory, a 0600
// file written through a same-directory temporary file and atomic rename, then
// read back and hash the final file. Existing remote file RPCs do not establish
// these guarantees. The stager must independently check that claude exists on
// that host. No production SSH implementation is assumed by this interface.
type ClaudeRemoteStager interface {
	StageClaudePrompt(context.Context, ClaudeRemoteStageRequest) (ClaudeRemoteStageResult, error)
}

type ClaudeRemoteStageResult struct {
	Connection    string
	UserContext   string
	PromptPath    string
	PromptHash    string
	CLIPath       string
	PrivateParent bool
	PrivateFile   bool
	AtomicRename  bool
	CLIAvailable  bool
	Cleanup       func() error
}

type ClaudePreparedLaunch struct {
	Argv             []string
	PromptPath       string
	PromptHash       string
	AdapterVersion   string
	IdentityEvidence string // requested; argv alone cannot validate provider identity
	Cleanup          func() error
}

func prepareClaudeLaunch(ctx context.Context, spec ClaudeLaunchSpec) (*ClaudePreparedLaunch, error) {
	if err := validateClaudeLaunchSpec(spec); err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(spec.PromptSnapshot))
	hashHex := hex.EncodeToString(hash[:])
	if hashHex != spec.PromptHash {
		return nil, errors.New("claude prompt snapshot hash mismatch")
	}

	var promptPath, cliPath string
	var cleanup func() error
	if spec.Connection == "" {
		current, err := user.Current()
		if err != nil || (spec.UserContext != current.Username && spec.UserContext != current.Uid) {
			return nil, errors.New("claude local user context mismatch")
		}
		cwdInfo, err := os.Stat(spec.Cwd)
		if err != nil || !cwdInfo.IsDir() {
			return nil, errors.New("claude local destination unavailable")
		}
		cliPath, err = resolveLocalClaude(spec.CLIPath)
		if err != nil {
			return nil, err
		}
		promptPath, cleanup, err = stageLocalClaudePrompt([]byte(spec.PromptSnapshot), hashHex)
		if err != nil {
			return nil, err
		}
	} else {
		if spec.RemoteStager == nil {
			return nil, errors.New("claude remote private staging unavailable")
		}
		stage, err := spec.RemoteStager.StageClaudePrompt(ctx, ClaudeRemoteStageRequest{
			Connection: spec.Connection, UserContext: spec.UserContext, Cwd: spec.Cwd,
			Prompt: []byte(spec.PromptSnapshot), PromptHash: hashHex,
		})
		if err != nil {
			return nil, errors.New("claude remote private staging failed")
		}
		if stage.Connection != spec.Connection || stage.UserContext != spec.UserContext || stage.PromptHash != hashHex ||
			!stage.PrivateParent || !stage.PrivateFile || !stage.AtomicRename || !stage.CLIAvailable ||
			!validClaudePath(stage.PromptPath) || !validClaudeBinary(stage.CLIPath) || stage.Cleanup == nil {
			if stage.Cleanup != nil {
				_ = stage.Cleanup()
			}
			return nil, errors.New("claude remote staging evidence incomplete")
		}
		promptPath, cliPath, cleanup = stage.PromptPath, stage.CLIPath, stage.Cleanup
	}

	argv := []string{cliPath}
	if spec.Mode == ClaudeLaunchStart {
		argv = append(argv, "--session-id", spec.SessionID)
	} else {
		argv = append(argv, "--resume", spec.SessionID)
	}
	argv = append(argv, "--append-system-prompt-file", promptPath)
	if spec.Mode == ClaudeLaunchStart {
		argv = append(argv, "--system-prompt-snapshot", "on")
	}
	return &ClaudePreparedLaunch{
		Argv: argv, PromptPath: promptPath, PromptHash: hashHex,
		AdapterVersion: ClaudeAdapterVersion, IdentityEvidence: "requested", Cleanup: cleanup,
	}, nil
}

func validateClaudeLaunchSpec(spec ClaudeLaunchSpec) error {
	parsed, err := uuid.Parse(spec.SessionID)
	if err != nil || parsed.String() != spec.SessionID {
		return errors.New("invalid claude session ID")
	}
	if spec.Mode != ClaudeLaunchStart && spec.Mode != ClaudeLaunchResume {
		return errors.New("invalid claude launch mode")
	}
	if len(spec.PromptSnapshot) > 1024*1024 || len(spec.PromptHash) != 64 {
		return errors.New("invalid claude prompt snapshot")
	}
	if spec.UserContext == "" || strings.ContainsAny(spec.UserContext, "\x00\r\n") || !validClaudePath(spec.Cwd) {
		return errors.New("invalid claude execution destination")
	}
	if strings.ContainsAny(spec.Connection, "\x00\r\n") || (spec.Connection != "" && spec.CLIPath != "") {
		return errors.New("invalid claude execution destination")
	}
	return nil
}

func validClaudePath(path string) bool {
	return filepath.IsAbs(path) && !strings.ContainsAny(path, "\x00\r\n")
}

func validClaudeBinary(path string) bool {
	return validClaudePath(path) && filepath.Base(path) == "claude"
}

func resolveLocalClaude(override string) (string, error) {
	path := override
	if path == "" {
		path, _ = exec.LookPath("claude")
		if path == "" {
			// Finder-launched apps can have a restricted PATH. Probe documented
			// installation locations without running a login shell or changing HOME.
			var candidates []string
			if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
				candidates = append(candidates, filepath.Join(home, ".local", "bin", "claude"))
			}
			if runtime.GOOS == "darwin" {
				candidates = append(candidates, "/opt/homebrew/bin/claude", "/usr/local/bin/claude")
			}
			for _, candidate := range candidates {
				if resolved, err := resolveLocalClaude(candidate); err == nil {
					return resolved, nil
				}
			}
			return "", errors.New("claude CLI unavailable")
		}
	}
	path, err := filepath.Abs(path)
	if err != nil || !validClaudeBinary(path) {
		return "", errors.New("claude CLI unavailable")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", errors.New("claude CLI unavailable")
	}
	return path, nil
}

func stageLocalClaudePrompt(prompt []byte, wantHash string) (string, func() error, error) {
	dir, err := os.MkdirTemp("", "force-claude-")
	if err != nil {
		return "", nil, errors.New("claude private staging failed")
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	fail := func() (string, func() error, error) {
		_ = cleanup()
		return "", nil, errors.New("claude private staging failed")
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		return fail()
	}
	tmp, err := os.CreateTemp(dir, ".prompt-")
	if err != nil {
		return fail()
	}
	tmpPath := tmp.Name()
	if _, err = tmp.Write(prompt); err != nil {
		_ = tmp.Close()
		return fail()
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fail()
	}
	if err = tmp.Close(); err != nil {
		return fail()
	}
	path := filepath.Join(dir, "prompt.txt")
	if err = os.Rename(tmpPath, path); err != nil {
		return fail()
	}
	file, err := os.Open(path)
	if err != nil {
		return fail()
	}
	fileInfo, statErr := file.Stat()
	if statErr != nil || fileInfo.Mode().Perm() != 0600 {
		_ = file.Close()
		return fail()
	}
	actualHash := sha256.New()
	_, err = io.Copy(actualHash, file)
	closeErr := file.Close()
	if err != nil || closeErr != nil || fmt.Sprintf("%x", actualHash.Sum(nil)) != wantHash {
		return fail()
	}
	return path, cleanup, nil
}
