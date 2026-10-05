// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"crypto/sha256"
	"errors"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

type SSHHostIdentity struct {
	keyFingerprint string
	sessionBinding [sha256.Size]byte
}

func (identity SSHHostIdentity) KeyFingerprint() string {
	return identity.keyFingerprint
}

func (identity SSHHostIdentity) MatchesClient(client *ssh.Client) bool {
	if client == nil || client.Conn == nil || identity.keyFingerprint == "" {
		return false
	}
	sessionID := client.SessionID()
	if len(sessionID) == 0 {
		return false
	}
	return identity.sessionBinding == sshSessionBinding(sessionID)
}

type sshHostIdentityCollector struct {
	lock           sync.Mutex
	keyFingerprint string
}

func (collector *sshHostIdentityCollector) wrap(callback ssh.HostKeyCallback) ssh.HostKeyCallback {
	callback = protectHostKeyCallback(callback)
	return func(hostname string, remote net.Addr, key ssh.PublicKey) (callbackErr error) {
		defer func() {
			if recover() != nil {
				collector.clear()
				callbackErr = errors.New("SSH host identity capture failed")
			}
			if callbackErr != nil {
				collector.clear()
			}
		}()

		collector.clear()
		if key == nil {
			return errors.New("SSH host key is unavailable")
		}
		if err := callback(hostname, remote, key); err != nil {
			return err
		}
		fingerprint := ssh.FingerprintSHA256(key)
		if fingerprint == "" {
			return errors.New("SSH host key fingerprint is unavailable")
		}
		collector.record(fingerprint)
		return nil
	}
}

// Panic payloads can contain host or configuration data, so callers receive only a generic failure.
func protectHostKeyCallback(callback ssh.HostKeyCallback) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) (callbackErr error) {
		defer func() {
			if recover() != nil {
				callbackErr = errors.New("SSH host key callback panicked")
			}
		}()
		if callback == nil {
			return errors.New("SSH host key callback is unavailable")
		}
		return callback(hostname, remote, key)
	}
}

func (collector *sshHostIdentityCollector) identityForSession(sessionID []byte) (SSHHostIdentity, bool) {
	if collector == nil || len(sessionID) == 0 {
		return SSHHostIdentity{}, false
	}
	collector.lock.Lock()
	defer collector.lock.Unlock()
	if collector.keyFingerprint == "" {
		return SSHHostIdentity{}, false
	}
	return SSHHostIdentity{
		keyFingerprint: collector.keyFingerprint,
		sessionBinding: sshSessionBinding(sessionID),
	}, true
}

func (collector *sshHostIdentityCollector) record(fingerprint string) {
	collector.lock.Lock()
	defer collector.lock.Unlock()
	collector.keyFingerprint = fingerprint
}

func (collector *sshHostIdentityCollector) clear() {
	if collector == nil {
		return
	}
	collector.lock.Lock()
	defer collector.lock.Unlock()
	collector.keyFingerprint = ""
}

func sshSessionBinding(sessionID []byte) [sha256.Size]byte {
	return sha256.Sum256(append([]byte("waveterm-ssh-session-binding-v1\x00"), sessionID...))
}
