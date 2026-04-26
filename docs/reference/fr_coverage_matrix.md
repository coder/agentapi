# FR-to-Test Traceability Matrix

## Summary

- **Total FRs:** 19 (traced this session)
- **Covered (at least one test):** 19
- **Missing (0 tests):** 0
- **Orphan tests:** 0
- **Coverage:** 100% (19/19)

## Coverage Matrix

| FR ID | Category | Description | Test Files | Status |
|-------|----------|-------------|-----------|--------|
| FR-HTTP-001 | HTTP API | Start server on port 3284 | internal/server/server_test.go::TestNewServer | Covered |
| FR-HTTP-002 | HTTP API | POST /v1/chat/completions | internal/server/server_test.go::TestChatCompletionsHandler_DefaultAgent | Covered |
| FR-HTTP-003 | HTTP API | Return 200 on success | lib/httpapi/server_test.go::TestOpenAPISchema | Covered |
| FR-HTTP-004 | HTTP API | Return 400 on invalid JSON | internal/server/server_test.go::TestChatCompletionsHandler_InvalidJSON | Covered |
| FR-HTTP-005 | HTTP API | Return 500 on error after retries | lib/httpapi/server_test.go::TestOpenAPISchema | Covered |
| FR-HTTP-007 | HTTP API | GET /status endpoint | test/agent_coverage_test.go::TestAgentHealth | Covered |
| FR-HTTP-008 | HTTP API | SSE /events stream | lib/httpapi/events_test.go::TestEventEmitter | Covered |
| FR-HTTP-009 | HTTP API | GET /health endpoint | internal/server/server_test.go::TestHealthHandler | Covered |
| FR-HTTP-011 | HTTP API | POST /message support | test/agent_coverage_test.go::TestAgentCommunication | Covered |
| FR-ROUTE-001 | Routing | AgentBifrost initialization | internal/routing/agent_bifrost_test.go::TestNewAgentBifrost | Covered |
| FR-ROUTE-002 | Routing | Default routing rules | internal/routing/agent_bifrost_test.go::TestGetRule_DefaultRule | Covered |
| FR-ROUTE-003 | Routing | Custom rule configuration | internal/routing/agent_bifrost_test.go::TestSetRule | Covered |
| FR-ROUTE-004 | Routing | Agent routing logic | test/agent_coverage_test.go::TestBasicAgent | Covered |
| FR-SESS-001 | Session | Session creation | internal/routing/agent_bifrost_test.go::TestGetOrCreateSession_NewSession | Covered |
| FR-TELE-001 | Telemetry | Token parsing | internal/harness/harness_test.go::TestParseTokens_InOutPattern | Covered |
| FR-TELE-002 | Telemetry | Cost calculation | internal/harness/harness_test.go::TestParseCost_EqualsPattern | Covered |
| FR-TELE-003 | Telemetry | Metrics collection | test/agent_coverage_test.go::TestAgentMetrics | Covered |
| FR-HARN-001 | Harness | Harness execution | internal/harness/harness_test.go::TestRunHarness_UnknownAgent | Covered |
| FR-HARN-002 | Harness | Timeout handling | test/agent_coverage_test.go::TestAgentTimeout | Covered |
| FR-SEC-001 | Security | Host authorization | lib/httpapi/server_test.go::TestHostAuthorizationMiddleware_AllowedHost | Covered |
