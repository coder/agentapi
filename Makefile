CHAT_SOURCES_STAMP = chat/.sources.stamp
CHAT_SOURCES = $(shell find chat \( -path chat/node_modules -o -path chat/out -o -path chat/.next \) -prune -o -not -path chat/.sources.stamp -type f -print)
BINPATH ?= out/agentapi
# This must be kept in sync with the magicBasePath in lib/httpapi/embed.go.
BASE_PATH ?= /magic-base-path-placeholder

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
