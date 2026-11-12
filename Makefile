.PHONY: run test race cover html lint check docker

COVER_MIN ?= 80

run:
	go run ./cmd/server

test:
	go test ./...

# -race требует cgo (Linux/macOS или gcc на Windows); в CI запускается всегда
race:
	go test -race ./...

cover:
	go test ./... -coverprofile=coverage/coverage.out
	go tool cover -func=coverage/coverage.out | tail -1
	@go tool cover -func=coverage/coverage.out | awk '/^total:/ {sub("%","",$$3); if ($$3+0 < $(COVER_MIN)) {print "coverage below $(COVER_MIN)%"; exit 1}}'

html: cover
	go tool cover -html=coverage/coverage.out -o coverage/index.html

lint:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	go vet ./...

check: lint test cover

docker:
	docker compose up --build
