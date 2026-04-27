PREFIX  ?= /usr/local
BINARY   = fenster
MODULE   = github.com/Arthur-Ficial/fenster
VERSION_FILE = .version
GOFLAGS  ?=
LDFLAGS  ?= -s -w
BUILD_DIR = .build/release
BIN_DIR   = bin
TYPE     ?= patch

.PHONY: check-toolchain build install uninstall clean test preflight \
        bump-patch bump-minor bump-major \
        generate-build-info generate-man-page man update-readme \
        version release release-patch release-minor release-major \
        package-release-asset print-release-asset print-release-sha256 \
        update-homebrew-formula vet lint vendor-tests

# --- Environment checks ---

check-toolchain:
	@if ! command -v go >/dev/null 2>&1; then \
		echo "error: 'go' not found in PATH. Install Go 1.22+: brew install go"; \
		exit 1; \
	fi; \
	gov=$$(go version | awk '{print $$3}' | sed 's/^go//'); \
	major=$$(echo $$gov | cut -d. -f1); \
	minor=$$(echo $$gov | cut -d. -f2); \
	if [ -z "$$major" ] || [ "$$major" -lt 1 ] || { [ "$$major" -eq 1 ] && [ "$$minor" -lt 22 ]; }; then \
		echo "error: fenster requires Go 1.22+, found $$gov"; \
		exit 1; \
	fi
	@if ! command -v python3 >/dev/null 2>&1; then \
		echo "warning: 'python3' not found. Tests/integration/ (apfel-compat suite) requires Python 3."; \
	fi

# --- Build ---

build: check-toolchain generate-build-info
	@mkdir -p $(BIN_DIR) $(BUILD_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/fenster
	cp $(BIN_DIR)/$(BINARY) $(BUILD_DIR)/$(BINARY)
	@$(MAKE) --no-print-directory generate-man-page

install: build
	@pkill -f "$(BINARY) --serve" 2>/dev/null || true
	@sleep 1
	@if command -v brew >/dev/null 2>&1 && brew list $(BINARY) >/dev/null 2>&1; then \
		brew_path=$$(brew --prefix)/bin/$(BINARY); \
		if [ -L "$$brew_path" ]; then \
			echo "unlinking Homebrew $(BINARY) (dev build takes priority)..."; \
			brew unlink $(BINARY) 2>/dev/null || true; \
		fi; \
	fi
	@if [ ! -d "$(PREFIX)/bin" ]; then \
		if [ -w "$(PREFIX)" ] 2>/dev/null || [ -w "$$(dirname $(PREFIX))" ]; then \
			mkdir -p "$(PREFIX)/bin"; \
		else \
			sudo mkdir -p "$(PREFIX)/bin"; \
		fi; \
	fi
	@if [ -w "$(PREFIX)/bin" ]; then \
		install $(BIN_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY); \
	else \
		sudo install $(BIN_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY); \
	fi
	@man_dir="$(PREFIX)/share/man/man1"; \
	if [ ! -d "$$man_dir" ]; then \
		if [ -w "$(PREFIX)/share" ] 2>/dev/null || [ -w "$(PREFIX)" ]; then \
			mkdir -p "$$man_dir"; \
		else \
			sudo mkdir -p "$$man_dir"; \
		fi; \
	fi; \
	if [ -f "$(BUILD_DIR)/$(BINARY).1" ]; then \
		if [ -w "$$man_dir" ]; then \
			install -m 0644 $(BUILD_DIR)/$(BINARY).1 "$$man_dir/$(BINARY).1"; \
		else \
			sudo install -m 0644 $(BUILD_DIR)/$(BINARY).1 "$$man_dir/$(BINARY).1"; \
		fi; \
	fi
	@echo "✓ installed: $$($(PREFIX)/bin/$(BINARY) --version 2>/dev/null || echo $(PREFIX)/bin/$(BINARY))"
	@resolved=$$(which $(BINARY) 2>/dev/null || echo "not in PATH"); \
	if [ "$$resolved" != "$(PREFIX)/bin/$(BINARY)" ]; then \
		echo "⚠ warning: 'which $(BINARY)' resolves to $$resolved, not $(PREFIX)/bin/$(BINARY)"; \
		echo "  Run: brew unlink $(BINARY)   (then make install again)"; \
	fi

# --- Version bumps ---

bump-patch:
	@v=$$(cat $(VERSION_FILE)); \
	major=$$(echo $$v | cut -d. -f1); \
	minor=$$(echo $$v | cut -d. -f2); \
	patch=$$(echo $$v | cut -d. -f3); \
	new="$$major.$$minor.$$((patch+1))"; \
	echo "$$new" > $(VERSION_FILE); \
	echo "$$v → $$new"

bump-minor:
	@v=$$(cat $(VERSION_FILE)); \
	major=$$(echo $$v | cut -d. -f1); \
	minor=$$(echo $$v | cut -d. -f2); \
	new="$$major.$$((minor+1)).0"; \
	echo "$$new" > $(VERSION_FILE); \
	echo "$$v → $$new"

bump-major:
	@v=$$(cat $(VERSION_FILE)); \
	major=$$(echo $$v | cut -d. -f1); \
	new="$$((major+1)).0.0"; \
	echo "$$new" > $(VERSION_FILE); \
	echo "$$v → $$new"

# --- Release-targeted builds ---

release-patch: check-toolchain bump-patch generate-build-info update-readme build
release-minor: check-toolchain bump-minor generate-build-info update-readme build
release-major: check-toolchain bump-major generate-build-info update-readme build

# --- Generated files ---

generate-build-info:
	@bash scripts/generate-build-info.sh

update-readme:
	@v=$$(cat $(VERSION_FILE)); \
	if [ -f README.md ]; then \
		sed -i.bak 's/Version [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*/Version '"$$v"'/' README.md && rm -f README.md.bak; \
		sed -i.bak 's/version-[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*-blue/version-'"$$v"'-blue/' README.md && rm -f README.md.bak; \
	fi

generate-man-page:
	@v=$$(cat $(VERSION_FILE)); \
	if [ ! -f man/$(BINARY).1.in ]; then \
		echo "warning: missing man/$(BINARY).1.in (created in FEN-017)"; \
		exit 0; \
	fi; \
	mkdir -p $(BUILD_DIR); \
	sed "s/@VERSION@/$$v/g" man/$(BINARY).1.in > $(BUILD_DIR)/$(BINARY).1; \
	if command -v mandoc >/dev/null 2>&1; then \
		if ! mandoc -Tlint -W warning $(BUILD_DIR)/$(BINARY).1 >/dev/null 2>&1; then \
			echo "error: mandoc -Tlint failed on $(BUILD_DIR)/$(BINARY).1"; \
			mandoc -Tlint -W warning $(BUILD_DIR)/$(BINARY).1; \
			exit 1; \
		fi; \
	fi

man: generate-man-page
	@man $(BUILD_DIR)/$(BINARY).1

# --- One-command release ---

release:
	@bash scripts/publish-release.sh $(TYPE)

# --- Test (build + all tests, single command) ---

test: build vet
	@echo ""
	@echo "=== Go unit tests ==="
	go test -race ./...
	@echo ""
	@echo "=== Integration tests (apfel-compat suite) ==="
	@if [ ! -f Tests/integration/conftest.py ]; then \
		echo "warning: Tests/integration/ not vendored yet — run scripts/port-apfel-tests.sh"; \
		exit 0; \
	fi
	@if ! command -v python3 >/dev/null 2>&1; then \
		echo "error: python3 required for Tests/integration/"; exit 1; \
	fi
	@pkill -f "$(BIN_DIR)/$(BINARY) --serve" 2>/dev/null || true
	@sleep 1
	@$(BIN_DIR)/$(BINARY) --serve --port 11434 2>/dev/null & echo $$! > /tmp/fenster-test-server.pid; \
	$(BIN_DIR)/$(BINARY) --serve --port 11435 --mcp mcp/calculator/server.py 2>/dev/null & echo $$! > /tmp/fenster-test-mcp.pid; \
	READY=0; for i in $$(seq 1 15); do \
		curl -sf http://localhost:11434/health >/dev/null 2>&1 && \
		curl -sf http://localhost:11435/health >/dev/null 2>&1 && \
		READY=1 && break; sleep 1; done; \
	if [ "$$READY" -ne 1 ]; then \
		echo "note: servers did not become healthy (expected during M0 RED state)"; \
	fi; \
	python3 -m pytest Tests/integration/ -v --tb=short; \
	STATUS=$$?; \
	kill $$(cat /tmp/fenster-test-server.pid) $$(cat /tmp/fenster-test-mcp.pid) 2>/dev/null || true; \
	rm -f /tmp/fenster-test-server.pid /tmp/fenster-test-mcp.pid; \
	exit $$STATUS

vet:
	go vet ./...

lint: vet
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed (optional); skipping"; \
	fi

vendor-tests:
	@echo "vendor-tests: archived. The apfel test suite was ported once and is now"
	@echo "fenster's own (Tests/integration/). See scripts/_port-apfel-tests.sh.archived."
	@exit 1

# --- Fast development loop ---
# `make test-fast` runs Go unit tests + model-free pytest only. Completes
# in <30s. Use this in your edit/build/test cycle.
#
# Model-hitting integration tests (~190) take ~5 min because each prompt
# is a real Gemini Nano inference on the M2 GPU. Run `make test` for that.
test-fast: build vet
	@echo ""
	@echo "=== Go unit tests (race) ==="
	go test -race ./...
	@echo ""
	@echo "=== Model-free integration tests ==="
	@if [ ! -f Tests/integration/conftest.py ]; then \
		echo "warning: Tests/integration/ not present"; \
		exit 0; \
	fi
	@if ! command -v python3 >/dev/null 2>&1; then \
		echo "error: python3 required for Tests/integration/"; exit 1; \
	fi
	@pkill -f "$(BIN_DIR)/$(BINARY) --serve" 2>/dev/null || true
	@sleep 1
	@FENSTER_BACKEND=echo $(BIN_DIR)/$(BINARY) --serve --port 11434 2>/dev/null & echo $$! > /tmp/fenster-fast-server.pid; \
	FENSTER_BACKEND=echo $(BIN_DIR)/$(BINARY) --serve --port 11435 --mcp mcp/calculator/server.py 2>/dev/null & echo $$! > /tmp/fenster-fast-mcp.pid; \
	for i in $$(seq 1 10); do curl -sf http://localhost:11434/health >/dev/null 2>&1 && break; sleep 1; done; \
	python3 -m pytest Tests/integration/ \
		--ignore=Tests/integration/test_chat.py \
		--ignore=Tests/integration/openai_client_test.py \
		--ignore=Tests/integration/mcp_server_test.py \
		--ignore=Tests/integration/mcp_remote_test.py \
		--ignore=Tests/integration/performance_test.py \
		-q --tb=no --no-header --timeout=15 -p no:cacheprovider; \
	STATUS=$$?; \
	kill $$(cat /tmp/fenster-fast-server.pid) $$(cat /tmp/fenster-fast-mcp.pid) 2>/dev/null || true; \
	rm -f /tmp/fenster-fast-server.pid /tmp/fenster-fast-mcp.pid; \
	exit $$STATUS

# --- Pre-release qualification ---

preflight:
	@bash scripts/release-preflight.sh

# --- Utilities ---

version:
	@cat $(VERSION_FILE)

uninstall:
	@if [ -w "$(PREFIX)/bin" ]; then \
		rm -f $(PREFIX)/bin/$(BINARY); \
	else \
		sudo rm -f $(PREFIX)/bin/$(BINARY); \
	fi
	@man_file="$(PREFIX)/share/man/man1/$(BINARY).1"; \
	if [ -e "$$man_file" ]; then \
		if [ -w "$(PREFIX)/share/man/man1" ]; then \
			rm -f "$$man_file"; \
		else \
			sudo rm -f "$$man_file"; \
		fi; \
	fi
	@if command -v brew >/dev/null 2>&1 && brew list $(BINARY) >/dev/null 2>&1; then \
		if ! [ -L "$$(brew --prefix)/bin/$(BINARY)" ]; then \
			echo "restoring Homebrew $(BINARY) link..."; \
			brew link $(BINARY) 2>/dev/null || true; \
		fi; \
	fi

clean:
	rm -rf $(BUILD_DIR) $(BIN_DIR) dist
	go clean -cache -testcache 2>/dev/null || true

package-release-asset:
	@v=$$(cat $(VERSION_FILE)); \
	os=$$(go env GOOS); arch=$$(go env GOARCH); \
	asset="$(BINARY)-$$v-$$arch-$$os.tar.gz"; \
	if [ ! -x "$(BUILD_DIR)/$(BINARY)" ]; then \
		echo "error: missing $(BUILD_DIR)/$(BINARY). Build a release binary first."; \
		exit 1; \
	fi; \
	tar -C $(BUILD_DIR) -czf "$$asset" $(BINARY) $$( [ -f "$(BUILD_DIR)/$(BINARY).1" ] && echo $(BINARY).1 ); \
	echo "$$asset"

print-release-asset:
	@v=$$(cat $(VERSION_FILE)); \
	os=$$(go env GOOS); arch=$$(go env GOARCH); \
	echo "$(BINARY)-$$v-$$arch-$$os.tar.gz"

print-release-sha256:
	@v=$$(cat $(VERSION_FILE)); \
	os=$$(go env GOOS); arch=$$(go env GOARCH); \
	asset="$(BINARY)-$$v-$$arch-$$os.tar.gz"; \
	if [ ! -f "$$asset" ]; then \
		echo "error: missing $$asset. Run make package-release-asset first."; \
		exit 1; \
	fi; \
	shasum -a 256 "$$asset" | awk '{print $$1}'

update-homebrew-formula:
	@if [ -z "$(HOMEBREW_FORMULA_OUTPUT)" ]; then \
		echo "error: set HOMEBREW_FORMULA_OUTPUT=/path/to/Formula/fenster.rb"; exit 1; \
	fi
	@if [ -z "$(HOMEBREW_FORMULA_SHA256)" ]; then \
		echo "error: set HOMEBREW_FORMULA_SHA256=<sha256>"; exit 1; \
	fi
	@./scripts/write-homebrew-formula.sh \
		--version "$$(cat $(VERSION_FILE))" \
		--sha256 "$(HOMEBREW_FORMULA_SHA256)" \
		--output "$(HOMEBREW_FORMULA_OUTPUT)"
