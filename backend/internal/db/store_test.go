package db

import "testing"

func TestBuildOrTsQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "empty string returns empty",
			query: "",
			want:  "",
		},
		{
			name:  "whitespace only returns empty",
			query: "   ",
			want:  "",
		},
		{
			name:  "single word returns empty (no OR needed)",
			query: "kubernetes",
			want:  "",
		},
		{
			name:  "two words builds OR query",
			query: "prompt workflow",
			want:  "'prompt' | 'workflow'",
		},
		{
			name:  "multiple words builds OR chain",
			query: "self hosted prompt workflow library",
			want:  "'self' | 'hosted' | 'prompt' | 'workflow' | 'library'",
		},
		{
			name:  "dot-separated like Next.js splits into tokens",
			query: "Next.js",
			want:  "'next' | 'js'",
		},
		{
			name:  "hyphen-separated splits into tokens",
			query: "self-hosted",
			want:  "'self' | 'hosted'",
		},
		{
			name:  "extra whitespace is trimmed",
			query: "  go   cli   tool  ",
			want:  "'go' | 'cli' | 'tool'",
		},
		{
			name:  "single quote is escaped",
			query: "it's good",
			want:  "'it''s' | 'good'",
		},
		{
			name:  "single-char tokens filtered out",
			query: "a react b cli",
			want:  "'react' | 'cli'",
		},
		{
			name:  "owner/repo format splits on slash",
			query: "vercel/next.js",
			want:  "'vercel' | 'next' | 'js'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOrTsQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildOrTsQuery(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestPgQuoteLiteral(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it's", "'it''s'"},
		{"", "''"},
		{"a'b'c", "'a''b''c'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := pgQuoteLiteral(tt.input)
			if got != tt.want {
				t.Errorf("pgQuoteLiteral(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
