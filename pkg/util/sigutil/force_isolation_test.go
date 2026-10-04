//go:build !windows

package sigutil

import "testing"

func TestSignalDumpDoesNotUseWaveLog(t *testing.T) {
	if DumpFilePath != "/tmp/force-terminal-usr1-dump.log" {
		t.Fatalf("signal dump: %s", DumpFilePath)
	}
}
