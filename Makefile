GO ?= go
VERSION ?= dev
LDFLAGS = -X github.com/SisyphusSQ/mybenchx/internal/cli.Version=$(VERSION)

.PHONY: build test race vet integration cross clean
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/mybenchx ./cmd/mybenchx

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

# Requires explicit authorization for the target in this ignored config file.
# No local database is started and no private profile is selected automatically.
integration:
	@test -n "$(MYBENCHX_INTEGRATION_CONFIG)" || (echo "Set MYBENCHX_INTEGRATION_CONFIG to an authorized target" >&2; exit 1)
	$(GO) test -race -tags=integration -count=1 -v ./internal/bench

cross:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/mybenchx-linux-amd64 ./cmd/mybenchx
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/mybenchx-linux-arm64 ./cmd/mybenchx
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/mybenchx-darwin-amd64 ./cmd/mybenchx
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/mybenchx-darwin-arm64 ./cmd/mybenchx

clean:
	rm -f bin/mybenchx dist/mybenchx-linux-amd64 dist/mybenchx-linux-arm64 dist/mybenchx-darwin-amd64 dist/mybenchx-darwin-arm64
