SHELL := /bin/bash
NODE_SHELL ?= /bin/zsh -lc

GO ?= go
GOFMT ?= gofmt
BIN_NAME ?= cricinfo
CMD_PATH ?= ./cmd/$(BIN_NAME)
DIST_DIR ?= dist
BIN_PATH ?= $(DIST_DIR)/$(BIN_NAME)
VERSION ?= $(shell node -p "require('./npm/package.json').version" 2>/dev/null)
LDFLAGS ?= -s -w -X github.com/amxv/cricinfo-cli/internal/buildinfo.Version=$(if $(VERSION),$(VERSION),dev)
ACCEPTANCE_LEAGUE ?= 11132
ACCEPTANCE_MATCH ?= 1527689
ACCEPTANCE_PLAYER ?= Virat Kohli
ACCEPTANCE_TEAM ?= rr

.PHONY: docs-check docs-build help fmt test test-live test-live-smoke fixtures-refresh vet lint check build build-all npm-smoke acceptance install-local clean release-tag

help:
	@echo "cricinfo command runner"
	@echo ""
	@echo "Targets:"
	@echo "  make fmt          - format Go files"
	@echo "  make test         - run go test ./..."
	@echo "  make test-live    - run opt-in live Cricinfo matrix tests"
	@echo "  make test-live-smoke - run opt-in live smoke coverage tests"
	@echo "  make fixtures-refresh - refresh curated live fixtures into testdata"
	@echo "  make vet          - run go vet ./..."
	@echo "  make lint         - run Node script checks"
	@echo "  make check        - fmt + test + vet + lint"
	@echo "  make build        - build local binary to dist/cricinfo"
	@echo "  make build-all    - build release binaries for 5 target platforms"
	@echo "  make npm-smoke    - pack + install npm tarball in a temp prefix and run cricinfo --help"
	@echo "  make acceptance   - run live end-to-end CLI acceptance checks over key command families"
	@echo "  make install-local- install CLI to ~/.local/bin/cricinfo"
	@echo "  make clean        - remove dist artifacts"
	@echo "  make release-tag  - create and push git tag (requires VERSION=x.y.z)"

fmt:
	@$(GOFMT) -w $$(find cmd internal -type f -name '*.go')

test:
	@$(GO) test ./...

test-live:
	@CRICINFO_LIVE_MATRIX=1 $(GO) test ./internal/cricinfo -run 'TestLive' -count=1

test-live-smoke:
	@CRICINFO_LIVE_SMOKE=1 $(GO) test ./internal/cricinfo -run 'TestLiveSmoke' -count=1

fixtures-refresh:
	@$(GO) run ./internal/cricinfo/cmd/fixture-refresh --write

vet:
	@$(GO) vet ./...

lint:
	@npm --prefix npm run lint

docs-check:
	@cd docs && bun run check

docs-build:
	@cd docs && bun run build

check: fmt test vet lint

build:
	@mkdir -p $(DIST_DIR)
	@$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_PATH) $(CMD_PATH)

build-all:
	@mkdir -p $(DIST_DIR)
	@for target in "darwin amd64" "darwin arm64" "linux amd64" "linux arm64" "windows amd64"; do \
		set -- $$target; \
		GOOS=$$1; GOARCH=$$2; \
		EXT=""; \
		if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
		echo "Building $(BIN_NAME) for $$GOOS/$$GOARCH"; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o "$(DIST_DIR)/$(BIN_NAME)_$$GOOS_$$GOARCH$$EXT" $(CMD_PATH); \
	done

npm-smoke:
	@set -euo pipefail; \
	node npm/scripts/prepare-package.mjs; \
	PACK_NAME="$$(cd npm/.publish && npm pack --silent --ignore-scripts)"; \
TMP_DIR="$$(mktemp -d)"; \
	/bin/rm -rf "$$TMP_DIR"; \
	mkdir -p "$$TMP_DIR"; \
	trap '/bin/rm -rf "$$TMP_DIR" "npm/.publish/$$PACK_NAME"' EXIT; \
	$(NODE_SHELL) "npm install --prefix '$$TMP_DIR/prefix' './npm/.publish/$$PACK_NAME' >/dev/null"; \
	$(NODE_SHELL) "'$$TMP_DIR/prefix/node_modules/.bin/$(BIN_NAME)' --help >/dev/null"; \
	echo "npm smoke ok: $$PACK_NAME"

acceptance: build
	@set -euo pipefail; \
	$(MAKE) npm-smoke; \
	BIN="$(BIN_PATH)"; \
	"$$BIN" --help >/dev/null; \
	"$$BIN" matches --help >/dev/null; \
	"$$BIN" players --help >/dev/null; \
	"$$BIN" teams --help >/dev/null; \
	"$$BIN" leagues --help >/dev/null; \
	"$$BIN" analysis --help >/dev/null; \
	"$$BIN" matches list --limit 5 --format json >/dev/null; \
	"$$BIN" matches live --limit 5 --format json >/dev/null; \
	"$$BIN" matches show "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" matches scorecard "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" matches innings "$(ACCEPTANCE_MATCH)" --team "$(ACCEPTANCE_TEAM)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" players search "$(ACCEPTANCE_PLAYER)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" players profile "$(ACCEPTANCE_PLAYER)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" players match-stats "$(ACCEPTANCE_PLAYER)" --match "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" players deliveries "$(ACCEPTANCE_PLAYER)" --match "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" teams show "$(ACCEPTANCE_TEAM)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" teams roster "$(ACCEPTANCE_TEAM)" --match "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" teams leaders "$(ACCEPTANCE_TEAM)" --match "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" leagues show "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" leagues seasons "$(ACCEPTANCE_LEAGUE)" --format json >/dev/null; \
	"$$BIN" seasons show "$(ACCEPTANCE_LEAGUE)" --season 2025 --format json >/dev/null; \
	"$$BIN" analysis bowling --metric economy --scope "match:$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --top 10 --format json >/dev/null; \
	"$$BIN" analysis dismissals --league "$(ACCEPTANCE_LEAGUE)" --seasons 2025 --top 5 --format json >/dev/null; \
	"$$BIN" analysis partnerships --scope "match:$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --top 5 --format json >/dev/null; \
	! "$$BIN" analysis bowling --metric economy --scope "match:$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --top 10 --format text | grep -E '(https?://|/v2/sports/cricket)'; \
	! "$$BIN" teams leaders "$(ACCEPTANCE_TEAM)" --match "$(ACCEPTANCE_MATCH)" --league "$(ACCEPTANCE_LEAGUE)" --format text | grep -E '(https?://|/v2/sports/cricket)'; \
	echo "acceptance ok: live command traversal and json rendering"

install-local: build
	@mkdir -p $$HOME/.local/bin
	@install -m 755 $(BIN_PATH) $$HOME/.local/bin/$(BIN_NAME)
	@echo "Installed $(BIN_NAME) to $$HOME/.local/bin/$(BIN_NAME)"

clean:
	@rm -rf $(DIST_DIR) npm/.publish docs/dist docs/.astro .vercel/output

release-tag:
	@test -n "$(VERSION)" || (echo "Usage: make release-tag VERSION=x.y.z" && exit 1)
	@git tag "v$(VERSION)"
	@git push origin "v$(VERSION)"
