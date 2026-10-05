package jobcontroller

import (
	"context"
	"strings"
	"testing"
)

func TestStartJobRejectsInvalidCwdBeforeConnectionOrPersistence(t *testing.T) {
	for _, cwd := range []string{"relative", "bad\x00path", "bad\rpath", "bad\npath"} {
		_, err := StartJob(context.Background(), StartJobParams{ConnName: "unconnected-fake", JobKind: JobKind_Shell, Cmd: "/fake/cli", Cwd: cwd})
		if err == nil || !strings.Contains(err.Error(), "working directory") {
			t.Fatalf("invalid cwd reached connection lookup: %q, %v", cwd, err)
		}
	}
}
