// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHHostIdentityCaptureRequiresAcceptedHostKeyCallback(t *testing.T) {
	key := testHostKey(t, 1)
	wantFingerprint := fingerprintFromKeyBytes(key.Marshal())
	rejected := errors.New("rejected")

	tests := []struct {
		name     string
		callback ssh.HostKeyCallback
		key      ssh.PublicKey
		wantErr  bool
		wantKey  bool
	}{
		{
			name: "accepted",
			callback: func(string, net.Addr, ssh.PublicKey) error {
				return nil
			},
			key:     key,
			wantKey: true,
		},
		{
			name: "rejected",
			callback: func(string, net.Addr, ssh.PublicKey) error {
				return rejected
			},
			key:     key,
			wantErr: true,
		},
		{
			name: "panic",
			callback: func(string, net.Addr, ssh.PublicKey) error {
				panic("callback panic")
			},
			key:     key,
			wantErr: true,
		},
		{
			name:    "nil callback",
			wantErr: true,
		},
		{
			name: "nil key",
			callback: func(string, net.Addr, ssh.PublicKey) error {
				return nil
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collector := &sshHostIdentityCollector{}
			wrapped := collector.wrap(test.callback)
			callbackErr := wrapped("host", nil, test.key)
			if (callbackErr != nil) != test.wantErr {
				t.Fatalf("callback error = %v, wantErr %v", callbackErr, test.wantErr)
			}
			identity, ok := collector.identityForSession([]byte("session-one"))
			if ok != test.wantKey {
				t.Fatalf("identity present = %v, want %v", ok, test.wantKey)
			}
			if test.wantKey && identity.KeyFingerprint() != wantFingerprint {
				t.Fatalf("fingerprint = %q, want %q", identity.KeyFingerprint(), wantFingerprint)
			}
		})
	}
}

func TestSSHHostIdentityCallbackPanicDoesNotExposeRecoveredValue(t *testing.T) {
	const sentinel = "callback-panic-secret-sentinel"
	collector := &sshHostIdentityCollector{}
	callback := collector.wrap(func(string, net.Addr, ssh.PublicKey) error {
		panic(sentinel)
	})
	err := callback("host", nil, testHostKey(t, 6))
	if err == nil {
		t.Fatal("panicking host-key callback returned no error")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("callback panic value leaked into error: %q", err)
	}
	if _, ok := collector.identityForSession([]byte("failed-session")); ok {
		t.Fatal("panicking callback retained a host identity")
	}
}

func TestSSHHostIdentityBindsAcceptedKeyToOneSession(t *testing.T) {
	keyA := testHostKey(t, 2)
	keyB := testHostKey(t, 3)
	identityA := captureTestIdentity(t, keyA, []byte("session-a"))
	identityAAgain := captureTestIdentity(t, keyA, []byte("session-a"))
	identitySessionB := captureTestIdentity(t, keyA, []byte("session-b"))
	identityKeyB := captureTestIdentity(t, keyB, []byte("session-a"))

	if identityA.KeyFingerprint() == identityKeyB.KeyFingerprint() {
		t.Fatal("distinct accepted keys produced the same fingerprint")
	}
	if identityA.KeyFingerprint() != identitySessionB.KeyFingerprint() {
		t.Fatal("the same accepted key changed fingerprint across sessions")
	}
	if !identityA.MatchesClient(testSSHClient([]byte("session-a"))) {
		t.Fatal("identity did not match its SSH session")
	}
	if identityA.MatchesClient(testSSHClient([]byte("session-b"))) {
		t.Fatal("identity matched a different SSH session")
	}
	if identityA.MatchesClient(&ssh.Client{}) {
		t.Fatal("identity matched a client without a transport")
	}
	if identityA.KeyFingerprint() != identityAAgain.KeyFingerprint() {
		t.Fatal("same accepted key produced inconsistent fingerprints")
	}
}

func TestSSHHostIdentityRejectsMissingSessionID(t *testing.T) {
	collector := &sshHostIdentityCollector{}
	if err := collector.wrap(func(string, net.Addr, ssh.PublicKey) error { return nil })("host", nil, testHostKey(t, 4)); err != nil {
		t.Fatal(err)
	}
	if identity, ok := collector.identityForSession(nil); ok || identity.KeyFingerprint() != "" {
		t.Fatalf("missing session ID yielded identity: %#v", identity)
	}
}

func captureTestIdentity(t *testing.T, key ssh.PublicKey, sessionID []byte) SSHHostIdentity {
	t.Helper()
	collector := &sshHostIdentityCollector{}
	callback := collector.wrap(func(string, net.Addr, ssh.PublicKey) error { return nil })
	if err := callback("host", nil, key); err != nil {
		t.Fatal(err)
	}
	identity, ok := collector.identityForSession(sessionID)
	if !ok {
		t.Fatal("accepted key and session ID did not yield identity")
	}
	return identity
}

func testHostKey(t *testing.T, seed byte) ssh.PublicKey {
	t.Helper()
	privateKey := ed25519.NewKeyFromSeed(bytesOf(seed, ed25519.SeedSize))
	key, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func testSSHClient(sessionID []byte) *ssh.Client {
	return ssh.NewClient(&testSSHConn{sessionID: sessionID}, nil, nil)
}

func fingerprintFromKeyBytes(keyBytes []byte) string {
	digest := sha256.Sum256(keyBytes)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(digest[:])
}

func bytesOf(value byte, length int) []byte {
	return []byte(strings.Repeat(string([]byte{value}), length))
}

type testSSHConn struct {
	sessionID []byte
}

func (conn *testSSHConn) User() string          { return "test" }
func (conn *testSSHConn) SessionID() []byte     { return conn.sessionID }
func (conn *testSSHConn) ClientVersion() []byte { return []byte("SSH-2.0-test-client") }
func (conn *testSSHConn) ServerVersion() []byte { return []byte("SSH-2.0-test-server") }
func (conn *testSSHConn) RemoteAddr() net.Addr  { return &net.TCPAddr{} }
func (conn *testSSHConn) LocalAddr() net.Addr   { return &net.TCPAddr{} }
func (conn *testSSHConn) Close() error          { return nil }
func (conn *testSSHConn) Wait() error           { return nil }
func (conn *testSSHConn) SendRequest(string, bool, []byte) (bool, []byte, error) {
	return false, nil, nil
}
func (conn *testSSHConn) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, errors.New("unexpected channel open")
}

func TestConnectInternalWithHostIdentityOnlyReturnsAfterHandshake(t *testing.T) {
	serverSigner, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(bytesOf(5, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		auth        []ssh.AuthMethod
		wantSuccess bool
	}{
		{name: "successful handshake", wantSuccess: true},
		{name: "authentication failure", auth: []ssh.AuthMethod{ssh.Password("invalid")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverConfig := &ssh.ServerConfig{
				NoClientAuth: test.wantSuccess,
				PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
					return nil, errors.New("authentication rejected")
				},
			}
			serverConfig.AddHostKey(serverSigner)
			address, serverDone := startTestSSHServer(t, serverConfig)
			collector := &sshHostIdentityCollector{}
			clientConfig := &ssh.ClientConfig{
				User:    "test",
				Auth:    test.auth,
				Timeout: 3 * time.Second,
				HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
					return nil
				},
			}
			client, identity, connectErr := connectInternalWithHostIdentity(context.Background(), address, clientConfig, nil, collector)
			if test.wantSuccess {
				if connectErr != nil {
					t.Fatalf("connect failed: %v", connectErr)
				}
				if identity.KeyFingerprint() != fingerprintFromKeyBytes(serverSigner.PublicKey().Marshal()) {
					t.Fatalf("fingerprint = %q", identity.KeyFingerprint())
				}
				if !identity.MatchesClient(client) {
					t.Fatal("identity is not bound to the returned SSH client")
				}
				if err := client.Close(); err != nil {
					t.Logf("client close: %v", err)
				}
			} else {
				if connectErr == nil {
					t.Fatal("authentication failure returned no error")
				}
				if client != nil || identity.KeyFingerprint() != "" {
					t.Fatalf("failed handshake exposed client or identity: client=%v identity=%q", client != nil, identity.KeyFingerprint())
				}
				if _, ok := collector.identityForSession([]byte("failed-session")); ok {
					t.Fatal("failed handshake retained a captured host identity")
				}
			}
			select {
			case <-serverDone:
			case <-time.After(3 * time.Second):
				t.Fatal("fake SSH server did not finish")
			}
		})
	}
}

func startTestSSHServer(t *testing.T, config *ssh.ServerConfig) (string, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		serverConn, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			conn.Close()
			return
		}
		go ssh.DiscardRequests(requests)
		go func() {
			for channel := range channels {
				channel.Reject(ssh.Prohibited, "test server does not open channels")
			}
		}()
		_ = serverConn.Wait()
	}()
	t.Cleanup(func() { listener.Close() })
	return listener.Addr().String(), done
}
