package utils

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseGitHubURL(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository URL")
	}

	return parts[0], parts[1], nil
}

func IsValidGitHubURL(repoURL string) bool {
	_, _, err := ParseGitHubURL(repoURL)
	return err == nil
}