GOLANGCI_LINT_VERSION ?= v1.64.8
GOLANGCI_LINT = go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: fmt fmt-check lint test vet check

fmt:
	gofmt -w .

fmt-check:
	@test -z "$(shell gofmt -l .)" || (echo "gofmt found unformatted files:" && gofmt -l . && exit 1)

lint:
	$(GOLANGCI_LINT) run ./...

test:
	go test ./...

vet:
	go vet ./...

check: fmt-check lint test vet
