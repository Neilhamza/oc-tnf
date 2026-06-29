package fencing

import (
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple string",
			input:  "pcs status nodes",
			expect: "'pcs status nodes'",
		},
		{
			name:   "string with single quotes",
			input:  "echo 'hello'",
			expect: "'echo '\"'\"'hello'\"'\"''",
		},
		{
			name:   "empty string",
			input:  "",
			expect: "''",
		},
		{
			name:   "subshell syntax",
			input:  "$(whoami)",
			expect: "'$(whoami)'",
		},
		{
			name:   "semicolon and pipe",
			input:  "cmd1; cmd2 | cmd3",
			expect: "'cmd1; cmd2 | cmd3'",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shellQuote(tc.input)
			if got != tc.expect {
				t.Errorf("shellQuote(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}
