# Minimum total statement coverage, in percent.
COVERAGE_MIN := 80

# Renovate updates this, once a release is at least 3 days old.
GOVULNCHECK_VERSION := v1.9.0

.PHONY: check fmt lint test cover vulncheck

check: lint cover

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

test:
	go test -count=1 -race ./...

cover:
	go test -count=1 -race -coverprofile=cover.out ./...
	@go tool cover -func=cover.out | awk -v min=$(COVERAGE_MIN) \
		'/^total:/ { sub("%", "", $$3); printf "coverage: %s%% (min %s%%)\n", $$3, min; exit ($$3 + 0 < min) }'

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
