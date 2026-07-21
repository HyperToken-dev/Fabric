.PHONY: generate build run migrate-up migrate-down lint test

generate:
	sqlc generate
	buf --timeout 5m generate

build: generate
	go build ./...

run:
	go run ./cmd/gateway

migrate-up:
	migrate -path db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DATABASE_URL)" down

lint:
	go vet ./...

test:
	go test ./...

sqlc-check:
	sqlc compile
