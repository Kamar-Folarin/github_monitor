package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseRepoURL parses a GitHub repository URL into owner and name components
func ParseRepoURL(repoURL string) (owner, name string, err error) {
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