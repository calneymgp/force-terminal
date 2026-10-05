package forceservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type claudeFakeTrace struct {
	Args       []string `json:"args"`
	Cwd        string   `json:"cwd"`
	Marker     string   `json:"marker"`
	Prompt     string   `json:"prompt"`
	PromptHash string   `json:"promptHash"`
}

func TestResolveLocalClaudeNativeInstallationOutsidePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	bin := filepath.Join(home, ".local", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(bin), 0700); err != nil {
		t.Fatal(err)
	}
	// Finding an installation must not execute it or read its user configuration.
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	got, err := resolveLocalClaude("")
	if err != nil || got != bin {
		t.Fatalf("native CLI not found outside GUI PATH: got=%q err=%v", got, err)
	}
	if err := os.Chmod(bin, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLocalClaude(bin); err == nil {
		t.Fatal("explicit non-executable CLI was accepted")
	}
	if _, err := resolveLocalClaude(filepath.Join(home, "missing", "claude")); err == nil {
		t.Fatal("invalid explicit CLI silently fell back to another installation")
	}
}

func TestResolveLocalClaudePrefersExplicitPathInstallation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	pathDir := filepath.Join(root, "path")
	t.Setenv("PATH", pathDir)
	for _, dir := range []string{pathDir, filepath.Join(root, ".local", "bin")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nexit 99\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(pathDir, "claude")
	got, err := resolveLocalClaude("")
	if err != nil || got != want {
		t.Fatalf("PATH installation preference changed: got=%q err=%v", got, err)
	}
}

type fakeRemoteClaudeStager struct {
	request ClaudeRemoteStageRequest
	result  ClaudeRemoteStageResult
	err     error
}

func (f *fakeRemoteClaudeStager) StageClaudePrompt(_ context.Context, request ClaudeRemoteStageRequest) (ClaudeRemoteStageResult, error) {
	f.request = request
	return f.result, f.err
}

func TestClaudeFakeCLI(t *testing.T) {
	if os.Getenv("FORCE_CLAUDE_FAKE") != "1" {
		return
	}
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	var promptPath string
	for i := range args {
		if args[i] == "--append-system-prompt-file" && i+1 < len(args) {
			promptPath = args[i+1]
		}
	}
	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		os.Exit(20)
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(21)
	}
	hash := sha256.Sum256(prompt)
	trace := claudeFakeTrace{Args: args, Cwd: cwd, Marker: os.Getenv("FORCE_CLAUDE_MARKER"), Prompt: string(prompt), PromptHash: hex.EncodeToString(hash[:])}
	data, err := json.Marshal(trace)
	if err != nil {
		os.Exit(22)
	}
	if err := os.WriteFile(os.Getenv("FORCE_CLAUDE_TRACE"), data, 0600); err != nil {
		os.Exit(23)
	}
	os.Exit(0)
}

func fakeClaudePath(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "claude")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestClaudeFakeCLI$ -- \"$@\"\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestClaudeLaunchLocalStartAndResume(t *testing.T) {
	bin := fakeClaudePath(t)
	cwd := filepath.Join(t.TempDir(), "projeto ç com espaços")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	id := "e6b8e037-3ed5-4fa0-9e84-e9fab6a47f8f"
	prompt := "Orientação privada: café ☕\n"
	hash := sha256.Sum256([]byte(prompt))
	hashHex := hex.EncodeToString(hash[:])
	for _, tc := range []struct {
		mode ClaudeLaunchMode
		want []string
	}{
		{ClaudeLaunchStart, []string{"--session-id", id, "--append-system-prompt-file", "PROMPT", "--system-prompt-snapshot", "on"}},
		{ClaudeLaunchResume, []string{"--resume", id, "--append-system-prompt-file", "PROMPT"}},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			prepared, err := prepareClaudeLaunch(context.Background(), ClaudeLaunchSpec{SessionID: id, PromptSnapshot: prompt, PromptHash: hashHex, Cwd: cwd, UserContext: current.Username, Mode: tc.mode, CLIPath: bin})
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Cleanup()
			if prepared.IdentityEvidence != "requested" || prepared.PromptHash != hashHex {
				t.Fatalf("identity/hash evidence: %q %q", prepared.IdentityEvidence, prepared.PromptHash)
			}
			if len(prepared.Argv) < 2 || prepared.Argv[0] != bin {
				t.Fatalf("argv executable: %q", prepared.Argv)
			}
			want := append([]string(nil), tc.want...)
			for i := range want {
				if want[i] == "PROMPT" {
					want[i] = prepared.PromptPath
				}
			}
			if !reflect.DeepEqual(prepared.Argv[1:], want) {
				t.Fatalf("argv = %q, want %q", prepared.Argv[1:], want)
			}
			if strings.Contains(strings.Join(prepared.Argv, " "), "--last") {
				t.Fatal("implicit last session")
			}
			parent, err := os.Stat(filepath.Dir(prepared.PromptPath))
			if err != nil || parent.Mode().Perm() != 0700 {
				t.Fatalf("private parent: %v %v", parent, err)
			}
			file, err := os.Stat(prepared.PromptPath)
			if err != nil || file.Mode().Perm() != 0600 {
				t.Fatalf("private file: %v %v", file, err)
			}
			tracePath := filepath.Join(t.TempDir(), "trace.json")
			cmd := exec.Command(prepared.Argv[0], prepared.Argv[1:]...)
			cmd.Dir = cwd
			cmd.Env = append(os.Environ(), "FORCE_CLAUDE_FAKE=1", "FORCE_CLAUDE_MARKER=local-user", "FORCE_CLAUDE_TRACE="+tracePath)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("fake CLI: %v %s", err, output)
			}
			data, err := os.ReadFile(tracePath)
			if err != nil {
				t.Fatal(err)
			}
			var trace claudeFakeTrace
			if err := json.Unmarshal(data, &trace); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(trace.Args, want) || trace.Cwd != cwd || trace.Marker != "local-user" || trace.Prompt != prompt || trace.PromptHash != hashHex {
				t.Fatalf("fake trace mismatch: %+v", trace)
			}
		})
	}
}

func TestClaudeLaunchRemoteRequiresPrivateSameHostEvidence(t *testing.T) {
	bin := fakeClaudePath(t)
	remoteRoot := filepath.Join(t.TempDir(), "host simulado 🛰")
	if err := os.Mkdir(remoteRoot, 0700); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(remoteRoot, "prompt privado.txt")
	prompt := "snapshot antes da edição"
	stageFile, err := os.CreateTemp(remoteRoot, ".prompt-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stageFile.WriteString(prompt); err != nil {
		t.Fatal(err)
	}
	if err := stageFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stageFile.Name(), promptPath); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(prompt))
	hashHex := hex.EncodeToString(hash[:])
	stage := &fakeRemoteClaudeStager{result: ClaudeRemoteStageResult{
		Connection: "ssh://host", UserContext: "remoto", PromptPath: promptPath,
		PromptHash: hashHex, CLIPath: bin, PrivateParent: true, PrivateFile: true,
		AtomicRename: true, CLIAvailable: true, Cleanup: func() error { return os.Remove(promptPath) },
	}}
	spec := ClaudeLaunchSpec{
		SessionID: "e6b8e037-3ed5-4fa0-9e84-e9fab6a47f8f", PromptSnapshot: prompt,
		PromptHash: hashHex, Connection: "ssh://host", UserContext: "remoto",
		Cwd: "/workspace/projeto ç com espaços", Mode: ClaudeLaunchResume, RemoteStager: stage,
	}
	prepared, err := prepareClaudeLaunch(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if stage.request.Connection != spec.Connection || stage.request.UserContext != spec.UserContext || stage.request.Cwd != spec.Cwd || string(stage.request.Prompt) != prompt || stage.request.PromptHash != hashHex {
		t.Fatalf("remote stage request differs from saved context: %+v", stage.request)
	}
	want := []string{bin, "--resume", spec.SessionID, "--append-system-prompt-file", promptPath}
	if !reflect.DeepEqual(prepared.Argv, want) {
		t.Fatalf("remote argv = %q, want %q", prepared.Argv, want)
	}
	spec.PromptSnapshot = "edited live profile"
	tracePath := filepath.Join(t.TempDir(), "remote trace.json")
	cmd := exec.Command(prepared.Argv[0], prepared.Argv[1:]...)
	cmd.Dir = remoteRoot
	cmd.Env = append(os.Environ(), "FORCE_CLAUDE_FAKE=1", "FORCE_CLAUDE_MARKER=remote-user", "FORCE_CLAUDE_TRACE="+tracePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fake remote CLI: %v %s", err, output)
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	var trace claudeFakeTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	if trace.Prompt != prompt || trace.PromptHash != hashHex || trace.Marker != "remote-user" || !reflect.DeepEqual(trace.Args, want[1:]) {
		t.Fatalf("fake remote trace mismatch: %+v", trace)
	}
	if err := prepared.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(promptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remote cleanup left prompt file: %v", err)
	}
}

func TestClaudeLaunchRejectsUnsafePreparation(t *testing.T) {
	bin := fakeClaudePath(t)
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	prompt := "SECRET_PROMPT_CANARY"
	hash := sha256.Sum256([]byte(prompt))
	base := ClaudeLaunchSpec{SessionID: "e6b8e037-3ed5-4fa0-9e84-e9fab6a47f8f", PromptSnapshot: prompt, PromptHash: hex.EncodeToString(hash[:]), Cwd: t.TempDir(), UserContext: current.Username, Mode: ClaudeLaunchStart, CLIPath: bin}
	cases := []struct {
		name string
		edit func(*ClaudeLaunchSpec)
	}{
		{"missing CLI", func(s *ClaudeLaunchSpec) { s.CLIPath = filepath.Join(t.TempDir(), "claude") }},
		{"bad destination", func(s *ClaudeLaunchSpec) { s.Cwd = filepath.Join(t.TempDir(), "missing") }},
		{"wrong user", func(s *ClaudeLaunchSpec) { s.UserContext = "someone-else" }},
		{"changed snapshot", func(s *ClaudeLaunchSpec) { s.PromptSnapshot = "edited live profile" }},
		{"invalid UUID", func(s *ClaudeLaunchSpec) { s.SessionID = "--last" }},
		{"remote unavailable", func(s *ClaudeLaunchSpec) { s.Connection = "ssh://host"; s.Cwd = "/workspace"; s.CLIPath = "" }},
		{"remote staging failed", func(s *ClaudeLaunchSpec) {
			s.Connection = "ssh://host"
			s.Cwd = "/workspace"
			s.CLIPath = ""
			s.RemoteStager = &fakeRemoteClaudeStager{err: errors.New(prompt)}
		}},
		{"remote mode unverified", func(s *ClaudeLaunchSpec) {
			s.Connection = "ssh://host"
			s.Cwd = "/workspace"
			s.CLIPath = ""
			s.RemoteStager = &fakeRemoteClaudeStager{result: ClaudeRemoteStageResult{Connection: "ssh://host", UserContext: s.UserContext, PromptHash: s.PromptHash, PromptPath: "/tmp/prompt", CLIPath: "/usr/bin/claude", CLIAvailable: true, Cleanup: func() error { return nil }}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := base
			tc.edit(&spec)
			prepared, err := prepareClaudeLaunch(context.Background(), spec)
			if err == nil {
				if prepared != nil && prepared.Cleanup != nil {
					_ = prepared.Cleanup()
				}
				t.Fatal("unsafe launch prepared")
			}
			if strings.Contains(err.Error(), prompt) {
				t.Fatalf("error leaked prompt: %v", err)
			}
		})
	}
}

func TestClaudeLaunchAcceptsEmptySavedProfilePrompt(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareClaudeLaunch(context.Background(), ClaudeLaunchSpec{
		SessionID:      "e6b8e037-3ed5-4fa0-9e84-e9fab6a47f8f",
		PromptSnapshot: "", PromptHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Cwd: t.TempDir(), UserContext: current.Username, Mode: ClaudeLaunchStart, CLIPath: fakeClaudePath(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Cleanup()
	data, err := os.ReadFile(prepared.PromptPath)
	if err != nil || len(data) != 0 {
		t.Fatalf("empty saved snapshot staged incorrectly: %q, %v", data, err)
	}
}
