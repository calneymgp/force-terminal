// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package secretstore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
)

const (
	SecretsFileName   = "secrets.enc"
	WriteDebounceMs   = 1000
	EncryptionTimeout = 5000
	InitRetryMs       = 1000
	SecretNamePattern = `^[A-Za-z][A-Za-z0-9_]*$`
	WriteTsKey        = "wave:writets"
)

var lock sync.Mutex
var secrets = make(map[string]string)
var writeRequestChan chan struct{}
var initialized bool
var lastInitTryTime time.Time
var lastInitErr error
var secretNameRegexp = regexp.MustCompile(SecretNamePattern)
var linuxStorageBackend string
var pendingWriteGeneration uint64
var persistedWriteGeneration uint64
var writeGate = make(chan struct{}, 1)
var persistSecretSnapshot = persistSnapshot

// must hold lock
func getLinuxStorageBackend() error {
	if runtime.GOOS != "linux" {
		return nil
	}

	rpcClient := wshclient.GetBareRpcClient()
	ctx, cancel := context.WithTimeout(context.Background(), EncryptionTimeout*time.Millisecond)
	defer cancel()

	encryptData := wshrpc.CommandElectronEncryptData{
		PlainText: "hello",
	}
	rpcOpts := &wshrpc.RpcOpts{
		Route:   wshutil.ElectronRoute,
		Timeout: EncryptionTimeout,
	}

	result, err := wshclient.ElectronEncryptCommand(rpcClient, encryptData, rpcOpts)
	if err != nil {
		return fmt.Errorf("failed to get storage backend: %w", err)
	}

	if ctx.Err() != nil {
		return fmt.Errorf("encryption timeout: %w", ctx.Err())
	}

	if result.StorageBackend != "" {
		linuxStorageBackend = result.StorageBackend
	}

	return nil
}

// must hold lock
func readSecretsFromFile() (map[string]string, error) {
	configDir := wavebase.GetWaveConfigDir()
	secretsPath := filepath.Join(configDir, SecretsFileName)

	encryptedData, err := os.ReadFile(secretsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("secretstore: could not read secrets file: %v\n", err)
		}
		if err := getLinuxStorageBackend(); err != nil {
			log.Printf("secretstore: could not get linux storage backend: %v\n", err)
		}
		return make(map[string]string), nil
	}

	rpcClient := wshclient.GetBareRpcClient()
	ctx, cancel := context.WithTimeout(context.Background(), EncryptionTimeout*time.Millisecond)
	defer cancel()

	decryptData := wshrpc.CommandElectronDecryptData{
		CipherText: string(encryptedData),
	}
	rpcOpts := &wshrpc.RpcOpts{
		Route:   wshutil.ElectronRoute,
		Timeout: EncryptionTimeout,
	}

	result, err := wshclient.ElectronDecryptCommand(rpcClient, decryptData, rpcOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets: %w", err)
	}

	if ctx.Err() != nil {
		return nil, fmt.Errorf("decryption timeout: %w", ctx.Err())
	}

	if result.StorageBackend != "" {
		linuxStorageBackend = result.StorageBackend
	}

	var decryptedSecrets map[string]string
	if err := json.Unmarshal([]byte(result.PlainText), &decryptedSecrets); err != nil {
		return nil, fmt.Errorf("failed to parse secrets: %w", err)
	}

	return decryptedSecrets, nil
}

func initSecretStore() error {
	lock.Lock()
	defer lock.Unlock()
	if initialized {
		return nil
	}

	now := time.Now()
	if !lastInitTryTime.IsZero() && now.Sub(lastInitTryTime) < InitRetryMs*time.Millisecond {
		return lastInitErr
	}

	lastInitTryTime = now
	loadedSecrets, err := readSecretsFromFile()
	if err != nil {
		lastInitErr = err
		return err
	}
	secrets = loadedSecrets

	writeRequestChan = make(chan struct{}, 1)
	initialized = true
	lastInitErr = nil
	go writerLoop()
	return nil
}

func writerLoop() {
	var timer *time.Timer
	for range writeRequestChan {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(WriteDebounceMs*time.Millisecond, func() {
			if err := writeSecretsToFile(context.Background()); err != nil {
				log.Printf("secretstore: error writing secrets: %v\n", err)
			}
		})
	}
}

func writeSecretsToFile(ctx context.Context) error {
	select {
	case writeGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-writeGate }()
	if err := ctx.Err(); err != nil {
		return err
	}

	lock.Lock()
	if pendingWriteGeneration == persistedWriteGeneration {
		lock.Unlock()
		return nil
	}
	generation := pendingWriteGeneration
	secretsCopy := make(map[string]string, len(secrets)+1)
	for k, v := range secrets {
		secretsCopy[k] = v
	}
	lock.Unlock()
	if err := persistSecretSnapshot(ctx, secretsCopy); err != nil {
		return err
	}
	lock.Lock()
	persistedWriteGeneration = generation
	lock.Unlock()
	return nil
}

// FlushForUpdate confirms pending mutations, including ones made during a
// concurrent write. An unused store never triggers a credential read.
func FlushForUpdate(ctx context.Context) error {
	lock.Lock()
	if !initialized {
		lock.Unlock()
		return ctx.Err()
	}
	lock.Unlock()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		lock.Lock()
		done := persistedWriteGeneration == pendingWriteGeneration
		lock.Unlock()
		if done {
			return nil
		}
		if err := writeSecretsToFile(ctx); err != nil {
			return err
		}
	}
}

func persistSnapshot(ctx context.Context, secretsCopy map[string]string) error {
	secretsCopy[WriteTsKey] = time.Now().UTC().Format(time.RFC3339)

	jsonData, err := json.Marshal(secretsCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	rpcClient := wshclient.GetBareRpcClient()
	if err := ctx.Err(); err != nil {
		return err
	}
	timeout := int64(EncryptionTimeout)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline).Milliseconds()
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		if remaining < timeout {
			timeout = remaining
		}
	}

	encryptData := wshrpc.CommandElectronEncryptData{
		PlainText: string(jsonData),
	}
	rpcOpts := &wshrpc.RpcOpts{
		Route:   wshutil.ElectronRoute,
		Timeout: timeout,
	}

	result, err := wshclient.ElectronEncryptCommand(rpcClient, encryptData, rpcOpts)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}

	if ctx.Err() != nil {
		return fmt.Errorf("encryption timeout: %w", ctx.Err())
	}

	configDir := wavebase.GetWaveConfigDir()
	secretsPath := filepath.Join(configDir, SecretsFileName)

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.WriteFile(secretsPath, []byte(result.CipherText), 0600); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	return nil
}

func requestWrite() {
	select {
	case writeRequestChan <- struct{}{}:
	default:
	}
}

func SetSecret(name string, value string) error {
	if name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}
	if !secretNameRegexp.MatchString(name) {
		return fmt.Errorf("secret name must start with a letter and contain only letters, numbers, and underscores")
	}
	if err := initSecretStore(); err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()

	secrets[name] = strings.TrimRight(value, "\r\n")
	pendingWriteGeneration++
	requestWrite()
	return nil
}

func DeleteSecret(name string) error {
	if name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}
	if err := initSecretStore(); err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()

	delete(secrets, name)
	pendingWriteGeneration++
	requestWrite()
	return nil
}

func GetSecret(name string) (string, bool, error) {
	if name == WriteTsKey {
		return "", false, nil
	}
	if err := initSecretStore(); err != nil {
		return "", false, err
	}
	lock.Lock()
	defer lock.Unlock()

	value, exists := secrets[name]
	return value, exists, nil
}

func GetSecretNames() ([]string, error) {
	if err := initSecretStore(); err != nil {
		return nil, err
	}
	lock.Lock()
	defer lock.Unlock()

	names := make([]string, 0, len(secrets))
	for name := range secrets {
		if name == WriteTsKey {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func CountSecrets() (int, error) {
	lock.Lock()
	defer lock.Unlock()

	if !initialized {
		return 0, fmt.Errorf("secret store not initialized")
	}

	count := 0
	for name := range secrets {
		if name == WriteTsKey {
			continue
		}
		count++
	}
	return count, nil
}

func GetLinuxStorageBackend() (string, error) {
	if runtime.GOOS != "linux" {
		return "", nil
	}

	lock.Lock()
	defer lock.Unlock()

	if linuxStorageBackend != "" {
		return linuxStorageBackend, nil
	}

	if err := getLinuxStorageBackend(); err != nil {
		return "", err
	}

	if linuxStorageBackend == "" {
		return "", fmt.Errorf("failed to determine linux storage backend")
	}

	return linuxStorageBackend, nil
}
