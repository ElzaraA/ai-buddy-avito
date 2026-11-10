.PHONY: run test cover html

run:
go run ./cmd/server

test:
go test ./...

cover:
go test ./... -coverprofile=coverage/coverage.out
go tool cover -func=coverage/coverage.out

html:
go tool cover -html=coverage/coverage.out -o coverage/index.html