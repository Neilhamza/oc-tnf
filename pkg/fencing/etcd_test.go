package fencing

import (
	"testing"
)

func TestParseEtcdHealth(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "both healthy",
			input:   `[{"health":true,"took":"10ms"},{"health":true,"took":"12ms"}]`,
			wantErr: false,
		},
		{
			name:    "one unhealthy",
			input:   `[{"health":true},{"health":false,"error":"context deadline exceeded"}]`,
			wantErr: true,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "single endpoint",
			input:   `[{"health":true}]`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := parseEtcdHealth(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseEtcdHealth() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestParseEtcdMembers(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		ipA     string
		ipB     string
		wantErr bool
	}{
		{
			name: "two voters",
			input: `{"members":[
				{"isLearner":false,"clientURLs":["https://10.0.0.1:2379"]},
				{"isLearner":false,"clientURLs":["https://10.0.0.2:2379"]}
			]}`,
			ipA:     "10.0.0.1",
			ipB:     "10.0.0.2",
			wantErr: false,
		},
		{
			name: "one learner",
			input: `{"members":[
				{"isLearner":false,"clientURLs":["https://10.0.0.1:2379"]},
				{"isLearner":true,"clientURLs":["https://10.0.0.2:2379"]}
			]}`,
			ipA:     "10.0.0.1",
			ipB:     "10.0.0.2",
			wantErr: true,
		},
		{
			name: "missing node B",
			input: `{"members":[
				{"isLearner":false,"clientURLs":["https://10.0.0.1:2379"]}
			]}`,
			ipA:     "10.0.0.1",
			ipB:     "10.0.0.2",
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `bad`,
			ipA:     "10.0.0.1",
			ipB:     "10.0.0.2",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := parseEtcdMembers(tc.input, tc.ipA, tc.ipB)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseEtcdMembers() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestFormatEtcdURL(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		expect string
	}{
		{
			name:   "IPv4 address",
			ip:     "10.0.0.1",
			expect: "https://10.0.0.1:2379",
		},
		{
			name:   "IPv6 address",
			ip:     "fd00::1",
			expect: "https://[fd00::1]:2379",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatEtcdURL(tc.ip)
			if got != tc.expect {
				t.Errorf("formatEtcdURL(%q) = %q, want %q", tc.ip, got, tc.expect)
			}
		})
	}
}
