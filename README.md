# GitHub Monitor

A Go service that fetches data from GitHub's public APIs to retrieve repository information and commits, saves the data in a PostgreSQL database, and continuously monitors the repository for changes. Exposes a REST API for querying commit and repository data.

## Features

- Fetches commit data (message, author, date, URL) and repository metadata from GitHub
- Stores data in PostgreSQL
- Periodically syncs new commits (default: every hour)
- Avoids duplicate commits; DB mirrors GitHub
- Configurable sync interval and reset point
- REST API for querying commits and top authors
- Swagger/OpenAPI documentation
- Unit tests for core logic

## Requirements

- Go 1.20+
- PostgreSQL
- GitHub Personal Access Token (for higher rate limits)

## Setup

1. **Clone the repository:**

   ```sh
   git clone <your-repo-url>
   cd github_monitor
   ```

2. **Configure environment variables:**
   Copy `.env.example` to `.env` and fill in your values:

   ```sh
   cp .env.example .env
   ```

   - `DB_CONNECTION_STRING`: PostgreSQL connection string
   - `GITHUB_TOKEN`: GitHub personal access token
   - `DEFAULT_SYNC_REPO`: (optional) GitHub repo to monitor, e.g. https://github.com/chromium/chromium (default: https://github.com/golang/go)
   - `PORT`: (optional) Port to run the server (default: 8080)
   - `SYNC_INTERVAL_MINUTES`: (optional) Sync interval in minutes (default: 60)

3. **Run database migrations:**

   ```sh
   go run cmd/server/main.go # Migrations run automatically on startup
   ```

4. **Start the server:**
   ```sh
   go run cmd/server/main.go
   ```

## Database Schema

- **repositories**: Stores repository metadata
- **commits**: Stores commit data

See `internal/db/migrations/0001_init.up.sql` for full schema.

## API Usage

- **Swagger UI:** [http://localhost:8080/swagger/](http://localhost:8080/swagger/)

### Get Commits

```
GET /api/v1/repos/{owner}/{repo}/commits?limit=50&offset=0
```

Returns paginated commits for a repository.

### Get Top Commit Authors

```
GET /api/v1/repos/{owner}/{repo}/top-authors?limit=10
```

Returns the top N commit authors by commit count.

### Reset Repository Sync Point

```
POST /api/v1/repos/{owner}/{repo}/reset?since=2024-01-01T00:00:00Z
```

Resets the sync point to fetch commits from a specific date.

## Testing

Run unit tests:

```sh
go test ./internal/github/...
```

## Example Unit Test

A unit test for the `ParseRepoURL` function is included in `internal/github/client_test.go`.

## Contribution

- Fork the repo and create a PR
- Open issues for bugs or feature requests

## License

MIT
