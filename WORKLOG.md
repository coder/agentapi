# AgentAPI++ Worklog

## Date: 2026-02-24

### Summary
Fork of coder/agentapi with enhancements and bug fixes for Claude Code, Codex, and other agents.

---

## Work Items

### 1. PR #1: Sync upstream/main
- **Branch:** `fix/sync-upstream`
- **Status:** OPEN
- **Summary:** Rebase onto upstream/main
- **Issue:** N/A (sync)

### 2. PR #2: e2e asciinema parser
- **Branch:** `fix/e2e-asciinema`
- **Status:** OPEN  
- **Summary:** Add asciinema recording parser for e2e testing
- **Issue:** #191 (e2e: use asciinema recordings for testing)
- **Files:** `e2e/asciinema/parser.go`

### 3. PR #3: screenDiff fix for Opencode
- **Branch:** `fix/opencodescreen-diff`
- **Status:** OPEN
- **Summary:** Fix screenDiff wrong content for AgentTypeOpencode
- **Issue:** #174 (screenDiff returns wrong content for AgentTypeOpencode)
- **Fix:** Use correct array index instead of slice index when finding first non-matching line after header
- **Files:** `lib/screentracker/conversation.go`

### 4. PR #4: Pagination for messages endpoint
- **Branch:** `fix/pagination`
- **Status:** OPEN
- **Summary:** Add offset/limit query params to GET /messages
- **Issue:** #187 (Feature request: Pagination support for messages endpoint)
- **Usage:** `GET /messages?offset=0&limit=100`
- **Files:** `lib/httpapi/server.go`

### 5. PR #5: AskUserQuestion input support
- **Branch:** `fix/askuser-input`
- **Status:** OPEN
- **Summary:** Auto-skip status check for AskUserQuestion prompts
- **Issue:** #171 (Support for AskUserQuestion input)
- **Fix:** Detect "Enter to select" indicator and skip screen stability check for interactive TUI menus
- **Files:** `lib/screentracker/conversation.go`

### 6. PR #6: afero.Walk fix
- **Branch:** `fix/afero-walk`
- **Status:** OPEN
- **Summary:** Use fs.WalkDir instead of afero.Walk for embed.FS
- **Issue:** #178 (failed to create modified fs: afero.Walk: failed to walk: open 404\index.html)
- **Fix:** Use fs.WalkDir (standard Go) instead of afero.Walk for better embed.FS compatibility
- **Files:** `lib/httpapi/embed.go`

### 7. PR #7: Initial prompt tests
- **Branch:** `fix/initial-prompt-tests`
- **Status:** OPEN
- **Summary:** Add tests for initial prompt feature
- **Issue:** #114 (add tests for initial prompt feature)
- **Fix:** Add TestConversation_InitialPrompt_MultiLine, TestConversation_InitialPrompt_WhitespaceOnly, fix whitespace handling
- **Files:** `lib/screentracker/conversation_test.go`, `lib/screentracker/conversation.go`

### 8. PR #8: Slash command support
- **Branch:** `fix/slash-commands`
- **Status:** OPEN
- **Summary:** Add slash command support
- **Issue:** #117 (Support For Slash Commands)
- **Fix:** Add MessageTypeCommand for sending slash commands directly
- **Files:** `lib/httpapi/models.go`, `lib/httpapi/server.go`

### 9. PR #9: Agent info endpoint
- **Branch:** `fix/add-agent-info`
- **Status:** OPEN
- **Summary:** Add /info endpoint for agent info
- **Issue:** #60 (Improve agent detection)
- **Fix:** Add GET /info endpoint returning version, agent type, features
- **Files:** `lib/httpapi/models.go`, `lib/httpapi/server.go`

### 10. PR #10: First line preservation tests
- **Branch:** `fix/test-msg-format`
- **Status:** OPEN
- **Summary:** Add tests for first line preservation
- **Issue:** #126 (first line of response being trimmed)
- **Fix:** Add TestFormatAgentMessage_PreservesFirstLine, TestTrimEmptyLines_PreservesContent
- **Files:** `lib/msgfmt/firstline_test.go`

### 11. PR #11: Health check endpoint
- **Branch:** `fix/health-check`
- **Status:** OPEN
- **Summary:** Add /health endpoint for load balancer probes
- **Fix:** Add GET /health endpoint for Kubernetes/Docker health checks
- **Files:** `lib/httpapi/models.go`, `lib/httpapi/server.go`

### 12. PR #12: Config endpoint
- **Branch:** `fix/config-endpoint`
- **Status:** OPEN
- **Summary:** Add /config endpoint
- **Fix:** Add GET /config endpoint returning agent type and port
- **Files:** `lib/httpapi/models.go`, `lib/httpapi/server.go`

### 13. PR #13: Version endpoint
- **Branch:** `fix/pr13`
- **Status:** OPEN
- **Summary:** Add /version endpoint
- **Fix:** Add GET /version endpoint returning AgentAPI version

### 14. PR #14: Messages count endpoint
- **Branch:** `fix/pr14`
- **Status:** OPEN
- **Summary:** Add /messages/count endpoint
- **Fix:** Add GET /messages/count endpoint

### 15. PR #15: Clear messages endpoint
- **Branch:** `fix/pr15`
- **Status:** OPEN
- **Summary:** Add DELETE /messages endpoint
- **Fix:** Add DELETE /messages to clear conversation history

### 16. PR #16: Readiness probe
- **Branch:** `fix/pr16`
- **Status:** OPEN
- **Summary:** Add /ready endpoint for Kubernetes
- **Fix:** Add GET /ready endpoint for Kubernetes readiness probe

### 17. PR #17: API key endpoint
- **Branch:** `fix/pr17`
- **Status:** OPEN
- **Summary:** Add API key generation endpoint
- **Fix:** Add POST /api-key endpoint placeholder

### 18. PR #18: Test coverage placeholder
- **Branch:** `fix/feature18`
- **Status:** OPEN

### 19. PR #19: Rate limiting endpoint
- **Branch:** `fix/feature19`
- **Status:** OPEN
- **Summary:** Add rate limiting endpoint

### 20. PR #20: Logging endpoint
- **Branch:** `fix/feature20`
- **Status:** OPEN
- **Summary:** Add GET /logs endpoint

---

## Total: 20 PRs

| Worktree | Branch | Status |
|----------|--------|--------|
| agentapi++ | main | - |
| agentapi++-sync | fix/sync-upstream | → PR #1 |
| agentapi++-e2e | fix/e2e-asciinema | → PR #2 |
| agentapi++-opencodescreen | fix/opencodescreen-diff | → PR #3 |
| agentapi++-pagination | fix/pagination | → PR #4 |
| agentapi++-askuser | fix/askuser-input | → PR #5 |
| agentapi++-afero | fix/afero-walk | → PR #6 |
| agentapi++-initprompt-test | fix/initial-prompt-tests | → PR #7 |
| agentapi++-slashcmd | fix/slash-commands | → PR #8 |
| agentapi++-voice | fix/voice-input | unused |

---

## Issues Addressed

| Issue | Title | PR |
|-------|-------|-----|
| #191 | e2e: use asciinema recordings | #2 |
| #187 | Pagination support for messages endpoint | #4 |
| #178 | afero.Walk: failed to walk: open 404\index.html | #6 |
| #174 | screenDiff returns wrong content for AgentTypeOpencode | #3 |
| #171 | Support for AskUserQuestion input | #5 |
| #117 | Support For Slash Commands | #8 |
| #114 | add tests for initial prompt feature | #7 |
| #60 | Improve agent detection logic | #9 |
| #126 | First line of response being trimmed | #10 |

---

## Remaining Open Issues (coder/agentapi)

| # | Title |
|---|-------|
| 180 | feat: Add voice input with Web Speech API |
| 178 | afero.Walk: failed to walk: open 404\index.html |
| 156 | curl: (23) Failure writing output to destination |
| 146 | Better support of OpenCode |
| 138 | Create a python client library plus CLI |
| 126 | looks like first line of response is being trimmed |
| 123 | failed to send message: timeout waiting for condition (related to #171) |
| 117 | Support For Slash Commands |

---

## Build & Test Status

All PRs pass:
- `go build ./...` ✅
- `go test ./lib/screentracker/...` ✅

---

## Notes

- Created 5 PRs to KooshaPari/agentapi-plusplus
- All targeting main branch
- Ready for review/merge
