// SPDX-License-Identifier: Apache-2.0
package wshremote

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

const forceAgentPrepareProtocol = "force-agent-prepare-v1"

var forceAgentCleanupLock sync.Mutex

type forceAgentPrepareReceipt struct {
	Protocol string `json:"protocol"`
	Nonce    string `json:"nonce"`
	Handle   string `json:"handle"`
	Hash     string `json:"hash"`
}

func (impl *ServerImpl) RemoteForceAgentContextCommand(ctx context.Context, data wshrpc.CommandRemoteForceAgentContextData) (*wshrpc.CommandRemoteForceAgentContextRtnData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if data.Protocol != forceAgentPrepareProtocol {
		return nil, errors.New("force_agent_protocol_unsupported")
	}
	if !validForceAgentNonce(data.Nonce) {
		return nil, errors.New("force_agent_nonce_invalid")
	}
	if impl.InitialEnv == nil {
		return nil, errors.New("force_agent_context_unavailable")
	}
	uid, fingerprint, err := forceAgentHostUserContextForEnv(impl.InitialEnv)
	if err != nil {
		return nil, errors.New("force_agent_context_unavailable")
	}
	return &wshrpc.CommandRemoteForceAgentContextRtnData{
		Protocol: data.Protocol, Nonce: data.Nonce, UID: uid, ContextFingerprint: fingerprint,
	}, nil
}

func (impl *ServerImpl) RemoteForceAgentPrepareCommand(ctx context.Context, data wshrpc.CommandRemoteForceAgentPrepareData) (*wshrpc.CommandRemoteForceAgentPrepareRtnData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if data.Protocol != forceAgentPrepareProtocol {
		return nil, errors.New("force_agent_protocol_unsupported")
	}
	if !validForceAgentNonce(data.Nonce) {
		return nil, errors.New("force_agent_nonce_invalid")
	}
	if data.ExpectedUID == "" || data.ExpectedContextFingerprint == "" {
		return nil, errors.New("force_agent_context_required")
	}
	base, err := impl.forceAgentPrivateStageBase()
	if err != nil {
		return nil, errors.New("force_agent_private_base_unavailable")
	}
	prepared, err := prepareForceAgentHostWithOps(ctx, forceAgentHostPrepareRequest{
		Root: data.Root, Cwd: data.Cwd, Prompt: data.Prompt, PromptHash: data.PromptHash,
		ExpectedUID: data.ExpectedUID, ExpectedContextFingerprint: data.ExpectedContextFingerprint,
		ContextEnv: impl.InitialEnv,
	}, forceAgentPrepareOps{baseDir: base})
	if err != nil {
		return nil, safeForceAgentRPCError(err, "force_agent_prepare_failed")
	}
	handle := filepath.Base(filepath.Dir(prepared.PromptPath))
	receipt := forceAgentPrepareReceipt{Protocol: data.Protocol, Nonce: data.Nonce, Handle: handle, Hash: prepared.PromptHash}
	if err := writeForceAgentReceipt(filepath.Dir(prepared.PromptPath), receipt); err != nil {
		_ = prepared.Cleanup()
		return nil, errors.New("force_agent_receipt_failed")
	}
	if err := ctx.Err(); err != nil {
		_ = prepared.Cleanup()
		return nil, err
	}
	return &wshrpc.CommandRemoteForceAgentPrepareRtnData{
		Protocol: data.Protocol, Nonce: data.Nonce,
		UID: prepared.UID, ContextFingerprint: prepared.ContextFingerprint,
		CanonicalRoot: prepared.CanonicalRoot, CanonicalCheckout: prepared.CanonicalCheckout,
		CanonicalCWD: prepared.CanonicalCWD, CLIPath: prepared.CLIPath,
		PromptPath: prepared.PromptPath, PromptHash: prepared.PromptHash,
		ParentMode: uint32(prepared.ParentMode), FileMode: uint32(prepared.FileMode),
		StagingMethod: prepared.StagingMethod, CleanupHandle: handle,
	}, nil
}

func (impl *ServerImpl) RemoteForceAgentCleanupCommand(ctx context.Context, data wshrpc.CommandRemoteForceAgentCleanupData) (*wshrpc.CommandRemoteForceAgentCleanupRtnData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if data.Protocol != forceAgentPrepareProtocol {
		return nil, errors.New("force_agent_protocol_unsupported")
	}
	if !validForceAgentNonce(data.Nonce) || !validForceAgentHandle(data.CleanupHandle) {
		return nil, errors.New("force_agent_cleanup_invalid")
	}
	forceAgentCleanupLock.Lock()
	defer forceAgentCleanupLock.Unlock()
	base, err := impl.forceAgentPrivateStageBase()
	if err != nil {
		return nil, errors.New("force_agent_private_base_unavailable")
	}
	dir := filepath.Join(base, data.CleanupHandle)
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return &wshrpc.CommandRemoteForceAgentCleanupRtnData{Protocol: data.Protocol, Nonce: data.Nonce}, nil
	}
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, errors.New("force_agent_cleanup_invalid")
	}
	receipt, err := readForceAgentReceipt(dir)
	if os.IsNotExist(err) {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || len(entries) != 0 {
			return nil, errors.New("force_agent_cleanup_invalid")
		}
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return nil, errors.New("force_agent_cleanup_failed")
		}
		return &wshrpc.CommandRemoteForceAgentCleanupRtnData{Protocol: data.Protocol, Nonce: data.Nonce}, nil
	}
	if err != nil || receipt.Protocol != data.Protocol || receipt.Nonce != data.Nonce || receipt.Handle != data.CleanupHandle {
		return nil, errors.New("force_agent_cleanup_invalid")
	}
	prompt := filepath.Join(dir, "prompt.txt")
	if _, err := os.Lstat(prompt); err == nil {
		if _, _, err := verifyForceAgentStaging(prompt, receipt.Hash); err != nil {
			return nil, errors.New("force_agent_cleanup_invalid")
		}
		if err := os.Remove(prompt); err != nil && !os.IsNotExist(err) {
			return nil, errors.New("force_agent_cleanup_failed")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("force_agent_cleanup_failed")
	}
	if err := os.Remove(filepath.Join(dir, "receipt.json")); err != nil && !os.IsNotExist(err) {
		return nil, errors.New("force_agent_cleanup_failed")
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return nil, errors.New("force_agent_cleanup_failed")
	}
	return &wshrpc.CommandRemoteForceAgentCleanupRtnData{Protocol: data.Protocol, Nonce: data.Nonce}, nil
}

func validForceAgentNonce(nonce string) bool {
	if len(nonce) < 16 || len(nonce) > 128 {
		return false
	}
	for _, c := range nonce {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func validForceAgentHandle(handle string) bool {
	if len(handle) != 32 {
		return false
	}
	for _, c := range handle {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func (impl *ServerImpl) forceAgentPrivateStageBase() (string, error) {
	if impl.InitialEnv == nil || !validForceAgentHostPath(impl.InitialEnv["HOME"]) {
		return "", errors.New("invalid agent home")
	}
	home, err := filepath.EvalSymlinks(impl.InitialEnv["HOME"])
	if err != nil {
		return "", err
	}
	info, err := os.Stat(home)
	if err != nil || !info.IsDir() {
		return "", errors.New("invalid agent home")
	}
	forceHome := filepath.Join(home, ".force-terminal")
	if err := ensureForceAgentPrivateDir(forceHome); err != nil {
		return "", err
	}
	base := filepath.Join(forceHome, "agent-prepares")
	if err := ensureForceAgentPrivateDir(base); err != nil {
		return "", err
	}
	return base, nil
}

func ensureForceAgentPrivateDir(path string) error {
	if err := os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("agent private directory unavailable")
	}
	return nil
}

func writeForceAgentReceipt(dir string, receipt forceAgentPrepareReceipt) error {
	tmp, err := os.CreateTemp(dir, ".receipt-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(receipt); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, "receipt.json")); err != nil {
		return err
	}
	if _, err := readForceAgentReceipt(dir); err != nil {
		return err
	}
	stage, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer stage.Close()
	return stage.Sync()
}

func readForceAgentReceipt(dir string) (forceAgentPrepareReceipt, error) {
	var receipt forceAgentPrepareReceipt
	path := filepath.Join(dir, "receipt.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return receipt, err
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 4096 {
		return receipt, errors.New("invalid agent receipt")
	}
	file, err := os.Open(path)
	if err != nil {
		return receipt, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(content) > 4096 {
		return receipt, errors.New("invalid agent receipt")
	}
	if err := json.Unmarshal(content, &receipt); err != nil {
		return receipt, err
	}
	if len(receipt.Hash) != 64 || strings.ContainsAny(receipt.Hash, "\x00\r\n") {
		return receipt, errors.New("invalid agent receipt")
	}
	return receipt, nil
}

func safeForceAgentRPCError(err error, code string) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errors.New(code)
}
