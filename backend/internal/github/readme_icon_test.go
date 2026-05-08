package github

import "testing"

func TestResolveReadmeIconURLPrefersLogoOverBadges(t *testing.T) {
	readme := `
# Project

![Build](https://img.shields.io/github/actions/workflow/status/acme/tool/ci.yml)
<img src="./docs/logo.svg" alt="Tool logo">
![Screenshot](./docs/screenshot.png)
`

	got := ResolveReadmeIconURL(readme, "acme", "tool", "trunk")
	want := "https://raw.githubusercontent.com/acme/tool/trunk/docs/logo.svg"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveReadmeIconURLNormalizesGitHubBlobURL(t *testing.T) {
	readme := `![Logo](https://github.com/acme/tool/blob/main/assets/icon.png?raw=true)`

	got := ResolveReadmeIconURL(readme, "acme", "tool", "main")
	want := "https://raw.githubusercontent.com/acme/tool/main/assets/icon.png"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveReadmeIconURLSkipsBannerMedia(t *testing.T) {
	readme := `![Project banner](./docs/images/banner.png)`

	got := ResolveReadmeIconURL(readme, "acme", "tool", "main")
	if got != "" {
		t.Fatalf("expected no icon candidate, got %q", got)
	}
}
