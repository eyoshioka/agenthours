# Minimum total statement coverage, in percent.
COVERAGE_MIN := 80

.PHONY: check fmt lint test cover

check: lint cover

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

test:
	go test -count=1 -race ./...

cover:
	go test -count=1 -coverprofile=cover.out ./...
	@go tool cover -func=cover.out | awk -v min=$(COVERAGE_MIN) \
		'/^total:/ { sub("%", "", $$3); printf "coverage: %s%% (min %s%%)\n", $$3, min; exit ($$3 + 0 < min) }'
