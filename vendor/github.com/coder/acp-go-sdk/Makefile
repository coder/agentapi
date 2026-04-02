ARCH := $(shell uname -m)
GO_FILES := $(shell find . -name '*.go' -type f)
ACP_VERSION := $(file < ./schema/version)
GOCACHE ?= $(CURDIR)/.gocache
MDSH ?= mdsh

version: README.md schema/meta.json schema/schema.json
	cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .
	env -u GOPATH -u GOMODCACHE go run mvdan.cc/gofumpt@latest -w .
	touch $@
	echo $(ACP_VERSION) > $@

schema/meta.json: schema/version
	curl -o $@ --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/v$(ACP_VERSION)/meta.json

schema/schema.json: schema/version
	curl -o $@ --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/v$(ACP_VERSION)/schema.json

README.md: schema/version
	@command -v $(MDSH) >/dev/null || { echo "mdsh not found; run 'nix develop' or install it." 1>&2; exit 1; }
	$(MDSH) --input README.md

.PHONY: guard-readme
guard-readme:
	@command -v $(MDSH) >/dev/null || { echo "mdsh not found; run 'nix develop' or install it." 1>&2; exit 1; }
	$(MDSH) --frozen --input README.md

.PHONY: fmt
fmt:
	nix fmt

.PHONY: check
check: guard-readme
	nix flake check

.PHONY: test
test: $(GO_FILES)
	GOFLAGS=$(GOFLAGS) GOCACHE=$(GOCACHE) go test ./...
	GOFLAGS=$(GOFLAGS) GOCACHE=$(GOCACHE) go build ./example/...

.PHONY: clean
clean:
	rm schema/meta.json schema/schema.json version
	mdsh --clean --input README.md
	touch schema/version # Touching the schema version file ensures that the README.md is regenerated on next make.

.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (e.g. 'make release VERSION=0.4.4')" 1>&2; \
		exit 1; \
	fi
	@printf '%s\n' "$(VERSION)" > schema/version
	$(MAKE) version
	$(MAKE) fmt
	GOCACHE=$(GOCACHE) $(MAKE) test
	$(MAKE) check
	@cmp -s schema/version version || (echo "schema/version and version differ; rerun 'make version'" 1>&2; exit 1)
	@echo
	@echo "Release candidate for $(VERSION) is ready. Review changes, commit, then tag v$(VERSION)."
