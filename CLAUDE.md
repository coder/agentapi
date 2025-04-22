# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build/Test Commands
```bash
# Build the Go project
go build ./...

# Run Go tests
go test ./...
# Run a specific Go test
go test github.com/coder/agentapi/lib/msgfmt -run TestFormatAgentMessage

# For chat interface (Next.js)
cd chat && npm install  # Install dependencies
cd chat && npm run dev  # Start development server
cd chat && npm run build  # Build for production
```

## Code Style Guidelines
- **Go imports**: Group in order: standard library, third-party, local imports
- **Error handling**: Use xerrors with context: `xerrors.Errorf("failed to...: %w", err)`
- **Naming**: 
  - Go: CamelCase for types/funcs (ServerConfig), camelCase for vars (agentType)
  - TypeScript: PascalCase for components (ChatInterface), camelCase for vars/funcs
- **Documentation**: Add comments for exported functions and complex logic
- **Testing**: Test files should be in same package with `_test.go` suffix
- **TypeScript**: Use TypeScript types for all components and functions

Maintain existing patterns when modifying code. Run tests before committing changes.