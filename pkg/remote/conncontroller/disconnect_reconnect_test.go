// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package conncontroller

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/remote"
	"github.com/wavetermdev/waveterm/pkg/wconfig"
	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"
)

func TestDelayedOldDisconnectDoesNotCloseReconnectedClient(t *testing.T) {
	newClient, newIdentity := makeConnectedTestClient(t)
	oldTransport := &delayedWaitTestSSHConn{
		waitStarted: make(chan struct{}),
		releaseWait: make(chan struct{}),
	}
	var releaseOnce sync.Once
	releaseOldWait := func() { releaseOnce.Do(func() { close(oldTransport.releaseWait) }) }
	t.Cleanup(releaseOldWait)
	oldClient := ssh.NewClient(oldTransport, nil, nil)
	conn := &SSHConn{
		lock:          &sync.Mutex{},
		lifecycleLock: &sync.Mutex{},
		Status:        Status_Connected,
		WshEnabled:    &atomic.Bool{},
		Opts:          &remote.SSHOpts{SSHHost: "test"},
		Client:        oldClient,
	}
	waiterDone := make(chan struct{})
	go func() {
		conn.waitForDisconnect(oldClient)
		close(waiterDone)
	}()
	select {
	case <-oldTransport.waitStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("old client waiter did not start")
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	conn.lifecycleLock.Lock()
	conn.WithLock(func() {
		conn.Client = newClient
		conn.hostIdentity = newIdentity
		conn.Status = Status_Connected
	})
	conn.lifecycleLock.Unlock()
	releaseOldWait()

	select {
	case <-waiterDone:
	case <-time.After(3 * time.Second):
		t.Fatal("old client waiter did not finish")
	}

	var status string
	conn.WithLock(func() { status = conn.Status })
	if status != Status_Connected {
		t.Fatalf("status after delayed old waiter = %q, want %q", status, Status_Connected)
	}
	if got := conn.GetClient(); got != newClient {
		t.Fatal("delayed old waiter replaced or closed the new client")
	}
	identity, ok := conn.GetHostIdentity()
	if !ok || identity.KeyFingerprint() != newIdentity.KeyFingerprint() {
		t.Fatalf("new identity after delayed old waiter = (%q, %v), want (%q, true)", identity.KeyFingerprint(), ok, newIdentity.KeyFingerprint())
	}
}

func makeConnectedTestClient(t *testing.T) (*ssh.Client, remote.SSHHostIdentity) {
	t.Helper()
	serverKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, ed25519.SeedSize))
	signer, err := ssh.NewSignerFromKey(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		serverConn, channels, requests, err := ssh.NewServerConn(conn, serverConfig)
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

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	knownHostEntry := xknownhosts.Line([]string{xknownhosts.Normalize(listener.Addr().String())}, signer.PublicKey()) + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(knownHostEntry), 0600); err != nil {
		t.Fatal(err)
	}
	identityAgent := ""
	passwordSecret := ""
	user := "test"
	flags := &wconfig.ConnKeywords{
		SshUser:                         &user,
		SshHostName:                     &host,
		SshPort:                         &port,
		SshPasswordSecretName:           &passwordSecret,
		SshBatchMode:                    boolPointer(true),
		SshPubkeyAuthentication:         boolPointer(false),
		SshPasswordAuthentication:       boolPointer(false),
		SshKbdInteractiveAuthentication: boolPointer(false),
		SshPreferredAuthentications:     []string{},
		SshIdentityAgent:                &identityAgent,
		SshIdentitiesOnly:               boolPointer(true),
		SshProxyJump:                    []string{},
		SshUserKnownHostsFile:           []string{knownHostsPath},
		SshGlobalKnownHostsFile:         []string{knownHostsPath},
	}
	client, identity, _, err := remote.ConnectToClientWithHostIdentity(context.Background(), &remote.SSHOpts{
		SSHHost: host,
		SSHUser: user,
		SSHPort: port,
	}, nil, 0, flags)
	if err != nil {
		t.Fatalf("connect to local fake SSH server: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		select {
		case <-serverDone:
		case <-time.After(3 * time.Second):
			t.Error("fake SSH server did not stop after client close")
		}
	})
	select {
	case <-serverDone:
		t.Fatal("fake SSH server disconnected before test completed")
	default:
	}
	return client, identity
}

func boolPointer(value bool) *bool {
	return &value
}

type delayedWaitTestSSHConn struct {
	waitStarted chan struct{}
	releaseWait chan struct{}
	waitOnce    sync.Once
}

func (conn *delayedWaitTestSSHConn) User() string          { return "test" }
func (conn *delayedWaitTestSSHConn) SessionID() []byte     { return []byte("old-session") }
func (conn *delayedWaitTestSSHConn) ClientVersion() []byte { return []byte("SSH-2.0-test-client") }
func (conn *delayedWaitTestSSHConn) ServerVersion() []byte { return []byte("SSH-2.0-test-server") }
func (conn *delayedWaitTestSSHConn) RemoteAddr() net.Addr  { return &net.TCPAddr{} }
func (conn *delayedWaitTestSSHConn) LocalAddr() net.Addr   { return &net.TCPAddr{} }
func (conn *delayedWaitTestSSHConn) Close() error          { return nil }
func (conn *delayedWaitTestSSHConn) Wait() error {
	conn.waitOnce.Do(func() {
		close(conn.waitStarted)
		<-conn.releaseWait
	})
	return errors.New("old test client disconnected")
}
func (conn *delayedWaitTestSSHConn) SendRequest(string, bool, []byte) (bool, []byte, error) {
	return false, nil, nil
}
func (conn *delayedWaitTestSSHConn) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, errors.New("unexpected channel open")
}
