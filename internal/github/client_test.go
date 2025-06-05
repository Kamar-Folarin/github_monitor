package github

import (
	"testing"
)

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		url      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"https://github.com/golang/go", "golang", "go", false},
		{"https://github.com/torvalds/linux", "torvalds", "linux", false},
		{"https://github.com/foo", "", "", true}, // invalid
		{"not-a-url", "", "", true}, // invalid
	}

	for _, tt := range tests {
		owner, repo, err := ParseRepoURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRepoURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseRepoURL(%q) = (%q, %q), want (%q, %q)", tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestNewGitHubClient(t *testing.T) {
	token := "dummy-token"
	client := NewGitHubClient(token)
	if client == nil {
		t.Fatal("NewGitHubClient returned nil")
	}
	if client.token != token {
		t.Errorf("expected token %q, got %q", token, client.token)
	}
	if client.client == nil {
		t.Error("expected non-nil http.Client")
	}
}

func TestGetCommits_InvalidInput(t *testing.T) {
	client := NewGitHubClient("dummy-token")
	tests := []struct {
		owner string
		repo  string
		wantErr bool
	}{
		{"", "repo", true},
		{"owner", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		_, err := client.GetCommits(nil, tt.owner, tt.repo, nil)
		if (err != nil) != tt.wantErr {
			t.Errorf("GetCommits(%q, %q) error = %v, wantErr %v", tt.owner, tt.repo, err, tt.wantErr)
		}
	}
} 