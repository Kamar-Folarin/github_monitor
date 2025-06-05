package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"		

	"golang.org/x/oauth2"
)

type GitHubClient struct {
	client *http.Client
	token  string
}

func NewGitHubClient(token string) *GitHubClient {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	// Set a custom HTTP client with a longer timeout
	httpClient := oauth2.NewClient(context.Background(), ts)
	httpClient.Timeout = 120 * time.Second

	return &GitHubClient{
		client: httpClient,
		token:  token,
	}
}

func (c *GitHubClient) GetRepository(ctx context.Context, owner, name string) (*Repository, error) {
	var repo Repository
	err := doWithRetry(3, 5*time.Second, func() error {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
		}
		return json.NewDecoder(resp.Body).Decode(&repo)
	})
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func (c *GitHubClient) GetCommits(ctx context.Context, owner, name string, since *time.Time) ([]*Commit, error) {
	if owner == "" || name == "" {
		return nil, fmt.Errorf("owner and name cannot be empty")
	}

	baseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits", owner, name)
	query := ""
	if since != nil {
		query = "since=" + since.Format(time.RFC3339)
	}

	var allCommits []*Commit
	page := 1

	for {
		var resp *http.Response
		var err error
		pagedURL := baseURL
		if query != "" {
			pagedURL += "?" + query + "&page=" + strconv.Itoa(page)
		} else {
			pagedURL += "?page=" + strconv.Itoa(page)
		}
		pagedURL += "&per_page=100"

		log.Printf("Requesting page %d for %s/%s: %s", page, owner, name, pagedURL)
		err = doWithRetry(3, 5*time.Second, func() error {
			req, reqErr := http.NewRequestWithContext(ctx, "GET", pagedURL, nil)
			if reqErr != nil {
				return reqErr
			}
			req.Header.Add("Accept", "application/vnd.github.v3+json")
			req.Header.Add("Authorization", "token "+c.getToken())
			resp, err = c.client.Do(req)
			if err != nil {
				log.Printf("Error requesting page %d: %v", page, err)
				return err
			}
			log.Printf("Received response for page %d (status: %d)", page, resp.StatusCode)
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		var commits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
			HTMLURL string `json:"html_url"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
			resp.Body.Close()
			log.Printf("Failed to decode response for page %d: %v", page, err)
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		log.Printf("Decoded %d commits for page %d", len(commits), page)

		if len(commits) == 0 {
			break
		}

		for _, c := range commits {
			allCommits = append(allCommits, &Commit{
				SHA:         c.SHA,
				Message:     c.Commit.Message,
				AuthorName:  c.Commit.Author.Name,
				AuthorEmail: c.Commit.Author.Email,
				AuthorDate:  c.Commit.Author.Date,
				CommitURL:   c.HTMLURL,
			})
		}

		page++
	}

	return allCommits, nil
}

// GetCommitsPage fetches a single page of commits for a repo, for use in per-page DB saving
func (c *GitHubClient) GetCommitsPage(ctx context.Context, owner, name string, since *time.Time, page int) ([]*Commit, error) {
	if owner == "" || name == "" {
		return nil, fmt.Errorf("owner and name cannot be empty")
	}

	baseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits", owner, name)
	query := ""
	if since != nil {
		query = "since=" + since.Format(time.RFC3339)
	}

	pagedURL := baseURL
	if query != "" {
		pagedURL += "?" + query + "&page=" + strconv.Itoa(page)
	} else {
		pagedURL += "?page=" + strconv.Itoa(page)
	}
	pagedURL += "&per_page=100"

	log.Printf("Requesting page %d for %s/%s: %s", page, owner, name, pagedURL)
	var resp *http.Response
	var err error
	err = doWithRetry(3, 5*time.Second, func() error {
		req, reqErr := http.NewRequestWithContext(ctx, "GET", pagedURL, nil)
		if reqErr != nil {
			return reqErr
		}
		req.Header.Add("Accept", "application/vnd.github.v3+json")
		req.Header.Add("Authorization", "token "+c.getToken())
		resp, err = c.client.Do(req)
		if err != nil {
			log.Printf("Error requesting page %d: %v", page, err)
			return err
		}
		log.Printf("Received response for page %d (status: %d)", page, resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		resp.Body.Close()
		log.Printf("Failed to decode response for page %d: %v", page, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	resp.Body.Close()

	log.Printf("Decoded %d commits for page %d", len(commits), page)

	var result []*Commit
	for _, c := range commits {
		result = append(result, &Commit{
			SHA:         c.SHA,
			Message:     c.Commit.Message,
			AuthorName:  c.Commit.Author.Name,
			AuthorEmail: c.Commit.Author.Email,
			AuthorDate:  c.Commit.Author.Date,
			CommitURL:   c.HTMLURL,
		})
	}

	return result, nil
}

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

func (c *GitHubClient) getToken() string {
	return c.token
}

// Add a helper for retrying HTTP requests
func doWithRetry(attempts int, sleep time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(sleep)
	}
	return err
}