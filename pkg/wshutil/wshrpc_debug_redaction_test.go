package wshutil

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

type forceAgentDebugTestServer struct {
	seen chan wshrpc.CommandRemoteForceAgentPrepareData
}

func (*forceAgentDebugTestServer) WshServerImpl() {}

func (server *forceAgentDebugTestServer) RemoteForceAgentPrepareCommand(ctx context.Context, data wshrpc.CommandRemoteForceAgentPrepareData) (*wshrpc.CommandRemoteForceAgentPrepareRtnData, error) {
	server.seen <- data
	if data.Nonce == "error-nonce" {
		return nil, errors.New("ERROR_SENTINEL_debug_response")
	}
	return &wshrpc.CommandRemoteForceAgentPrepareRtnData{Protocol: data.Protocol, Nonce: data.Nonce, CLIPath: "RESPONSE_SENTINEL_debug_body"}, nil
}

func TestWshRpcDebugDoesNotLogRequestOrResponseBodies(t *testing.T) {
	var logs bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldOutput)

	server := &forceAgentDebugTestServer{seen: make(chan wshrpc.CommandRemoteForceAgentPrepareData, 2)}
	rpc := MakeWshRpc(wshrpc.RpcContext{}, server, "force-debug-test")
	rpc.Debug = true
	prompt := []byte("PROMPT_SENTINEL_debug_body")
	promptBase64 := base64.StdEncoding.EncodeToString(prompt)
	for _, tc := range []struct {
		id    string
		nonce string
	}{
		{id: "req-success", nonce: "success-nonce"},
		{id: "req-error", nonce: "error-nonce"},
	} {
		packet, err := json.Marshal(RpcMessage{Command: "remoteforceagentprepare", ReqId: tc.id, Data: map[string]any{
			"protocol": "force-agent-prepare-v1", "nonce": tc.nonce, "prompt": prompt,
			"env": "ENV_SENTINEL_debug_home", "auth": "AUTH_SENTINEL_debug_token",
		}})
		if err != nil || !rpc.SendRpcMessage(packet, 0, "") {
			t.Fatalf("request could not enter RPC: %v", err)
		}
		response, ok := rpc.RecvRpcMessage()
		if !ok {
			t.Fatal("RPC response channel closed")
		}
		var got RpcMessage
		if err := json.Unmarshal(response, &got); err != nil || got.ResId != tc.id {
			t.Fatalf("RPC dispatch/response failed: %#v, %v", got, err)
		}
		if tc.id == "req-success" && got.Data == nil || tc.id == "req-error" && got.Error != "ERROR_SENTINEL_debug_response" {
			t.Fatalf("RPC result changed: %#v", got)
		}
		if !rpc.SendRpcMessage(response, 0, "") {
			t.Fatal("response could not traverse debug receive path")
		}
	}
	close(rpc.InputCh)
	for range rpc.OutputCh {
	}
	for range 2 {
		if got := <-server.seen; string(got.Prompt) != string(prompt) {
			t.Fatal("RPC payload did not reach the handler")
		}
	}
	for _, secret := range []string{string(prompt), promptBase64, "ENV_SENTINEL_debug_home", "AUTH_SENTINEL_debug_token", "ERROR_SENTINEL_debug_response", "RESPONSE_SENTINEL_debug_body"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("RPC debug log exposed a body sentinel: %s", secret)
		}
	}
}
