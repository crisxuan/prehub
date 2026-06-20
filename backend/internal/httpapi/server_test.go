package httpapi

import "testing"

func TestParseSearchRepositoryRef(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{
			name:      "full GitHub URL",
			query:     "https://github.com/vercel/next.js",
			wantOwner: "vercel",
			wantRepo:  "next.js",
			wantOK:    true,
		},
		{
			name:      "owner/repo format",
			query:     "vercel/next.js",
			wantOwner: "vercel",
			wantRepo:  "next.js",
			wantOK:    true,
		},
		{
			name:   "plain keyword",
			query:  "Next.js",
			wantOK: false,
		},
		{
			name:   "single word keyword",
			query:  "react",
			wantOK: false,
		},
		{
			name:   "multi-word keyword",
			query:  "AI agent monitoring",
			wantOK: false,
		},
		{
			name:   "empty string",
			query:  "",
			wantOK: false,
		},
		{
			name:      "GitHub URL with trailing content",
			query:     "check out https://github.com/facebook/react for UI",
			wantOwner: "facebook",
			wantRepo:  "react",
			wantOK:    true,
		},
		{
			name:   "reserved GitHub path",
			query:  "explore/topics",
			wantOK: false,
		},
		{
			name:      "owner/repo with hyphens",
			query:     "multica-ai/multica",
			wantOwner: "multica-ai",
			wantRepo:  "multica",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := parseSearchRepositoryRef(tt.query)
			if ok != tt.wantOK {
				t.Fatalf("parseSearchRepositoryRef(%q) ok = %v, want %v", tt.query, ok, tt.wantOK)
			}
			if ok {
				if owner != tt.wantOwner {
					t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
				}
			}
		})
	}
}
