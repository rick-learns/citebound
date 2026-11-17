package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no args", args: nil, wantCode: 2, wantStderr: "Usage:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "dev"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Commands:"},
		{name: "unknown", args: []string{"nope"}, wantCode: 2, wantStderr: `unknown command "nope"`},
		{name: "corpus verify", args: []string{"corpus", "verify", "-dir", "../../corpus"}, wantCode: 0, wantStdout: "21 RFCs verified"},
		{name: "corpus verify missing dir", args: []string{"corpus", "verify", "-dir", "does-not-exist"}, wantCode: 1, wantStderr: "read manifest"},
		{name: "corpus stats", args: []string{"corpus", "stats", "-dir", "../../corpus"}, wantCode: 0, wantStdout: "Total"},
		{name: "search", args: []string{"search", "-dir", "../../corpus", "-k", "1", "refresh", "token", "rotation"}, wantCode: 0, wantStdout: "1. RFC"},
		{name: "search without query", args: []string{"search"}, wantCode: 2, wantStderr: "Usage: citebound search"},
		{name: "search with only stopwords", args: []string{"search", "-dir", "../../corpus", "what is the"}, wantCode: 1, wantStderr: "no searchable words"},
		{name: "corpus without subcommand", args: []string{"corpus"}, wantCode: 2, wantStderr: "corpus verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
