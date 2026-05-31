.PHONY: fmt test vet

fmt:
	gofmt -w .

test:
	go test ./...

vet:
	go vet ./...
