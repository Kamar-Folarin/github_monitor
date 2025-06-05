.PHONY: run test migrate swagger

run: swagger  # Depends on swagger docs being generated first
    @echo "Starting server..."
    go run cmd/server/main.go

test:
    @echo "Running tests..."
    go test -v ./...

migrate:
    @echo "Running migrations..."
    goose -dir internal/db/migrations postgres "$$(grep DB_CONNECTION_STRING .env | cut -d '=' -f2)" up

swagger:
    @echo "Generating Swagger docs..."
    swag init -g cmd/server/main.go