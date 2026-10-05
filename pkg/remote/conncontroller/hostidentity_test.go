// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package conncontroller

import (
	"net"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGetHostIdentityFailsClosedWithoutVerifiedConnectedTransport(t *testing.T) {
	client := ssh.NewClient(&hostIdentityTestConn{sessionID: []byte("session")}, nil, nil)
	tests := []struct {
		name   string
		status string
		client *ssh.Client
	}{
		{name: "connected without captured identity", status: Status_Connected, client: client},
		{name: "connected without transport", status: Status_Connected},
		{name: "connecting", status: Status_Connecting, client: client},
		{name: "disconnected", status: Status_Disconnected, client: client},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := &SSHConn{
				lock:   &sync.Mutex{},
				Status: test.status,
				Client: test.client,
			}
			identity, ok := conn.GetHostIdentity()
			if ok || identity.KeyFingerprint() != "" {
				t.Fatalf("unverified connection returned identity %q", identity.KeyFingerprint())
			}
		})
	}
}

type hostIdentityTestConn struct {
	sessionID []byte
}

func (conn *hostIdentityTestConn) User() string          { return "test" }
func (conn *hostIdentityTestConn) SessionID() []byte     { return conn.sessionID }
func (conn *hostIdentityTestConn) ClientVersion() []byte { return []byte("SSH-2.0-test-client") }
func (conn *hostIdentityTestConn) ServerVersion() []byte { return []byte("SSH-2.0-test-server") }
func (conn *hostIdentityTestConn) RemoteAddr() net.Addr  { return &net.TCPAddr{} }
func (conn *hostIdentityTestConn) LocalAddr() net.Addr   { return &net.TCPAddr{} }
func (conn *hostIdentityTestConn) Close() error          { return nil }
func (conn *hostIdentityTestConn) Wait() error           { return nil }
func (conn *hostIdentityTestConn) SendRequest(string, bool, []byte) (bool, []byte, error) {
	return false, nil, nil
}
func (conn *hostIdentityTestConn) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, nil
}
