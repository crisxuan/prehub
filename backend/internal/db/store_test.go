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
			want:  "to_tsquery('simple', 'prompt') | to_tsquery('simple', 'workflow')",
		},
		{
			name:  "multiple words builds OR chain",
			query: "self hosted prompt workflow library",
			want:  "to_tsquery('simple', 'self') | to_tsquery('simple', 'hosted') | to_tsquery('simple', 'prompt') | to_tsquery('simple', 'workflow') | to_tsquery('simple', 'library')",
		},
		{
			name:  "extra whitespace is trimmed",
			query: "  go   cli   tool  ",
			want:  "to_tsquery('simple', 'go') | to_tsquery('simple', 'cli') | to_tsquery('simple', 'tool')",
		},
		{
			name:  "single quote is escaped",
			query: "it's good",
			want:  "to_tsquery('simple', 'it''s') | to_tsquery('simple', 'good')",
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
