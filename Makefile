CHAT_SOURCES_STAMP = chat/.sources.stamp
CHAT_SOURCES = $(shell find chat \( -path chat/node_modules -o -path chat/out -o -path chat/.next \) -prune -o -not -path chat/.sources.stamp -type f -print)
BINPATH ?= out/agentapi
# This must be kept in sync with the magicBasePath in lib/httpapi/embed.go.
BASE_PATH ?= /magic-base-path-placeholder
FIND_EXCLUSIONS= \
	-not \( \( -path '*/.git/*' -o -path './out/*' -o -path '*/node_modules/*' -o -path '*/.terraform/*' \) -prune \)
SHELL_SRC_FILES := $(shell find . $(FIND_EXCLUSIONS) -type f -name '*.sh')

$(CHAT_SOURCES_STAMP): $(CHAT_SOURCES)
	@echo "Chat sources changed. Running build steps..."
	cd chat && NEXT_PUBLIC_BASE_PATH="${BASE_PATH}" bun run build
	rm -rf lib/httpapi/chat && mkdir -p lib/httpapi/chat && touch lib/httpapi/chat/marker
	cp -r chat/out/. lib/httpapi/chat/
	touch $@

.PHONY: embed
embed: $(CHAT_SOURCES_STAMP)
	@echo "Chat build is up to date."

.PHONY: build
build: embed
	CGO_ENABLED=0 go build -o ${BINPATH} main.go

.PHONY: gen
gen:
	go generate ./...

lint: lint/shellcheck lint/go lint/ts lint/actions/actionlint
.PHONY: lint

lint/go:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0 run
	go run github.com/coder/paralleltestctx/cmd/paralleltestctx@v0.0.1 ./...
.PHONY: lint/go

lint/shellcheck: $(SHELL_SRC_FILES)
	echo "--- shellcheck"
	shellcheck --external-sources $(SHELL_SRC_FILES)
.PHONY: lint/shellcheck

lint/ts:
	cd ./chat && bun lint
.PHONY: lint/ts

lint/actions/actionlint:
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 --config-file actionlint.yaml
.PHONY: lint/actions/actionlint
