package models

import "time"

type Commit struct {
    ID          int       `json:"-"`
    RepositoryID int       `json:"-"`
    SHA         string    `json:"sha"`
    Message     string    `json:"message"`
    AuthorName  string    `json:"author_name"`
    AuthorEmail string    `json:"author_email"`
    AuthorDate  time.Time `json:"author_date"`
    CommitURL   string    `json:"html_url"`
}

type AuthorStats struct {
    Name        string `json:"name"`
    Email       string `json:"email"`
    CommitCount int    `json:"commit_count"`
}