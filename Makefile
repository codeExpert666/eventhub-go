.PHONY: fmt sqlc test vet

fmt:
	gofmt -w .

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate

test:
	go test ./...

vet:
	go vet ./...
