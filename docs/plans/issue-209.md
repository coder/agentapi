# Plan: Fix Send() blocking indefinitely when ReadyForInitialPrompt returns false (#209)

Research: [issue-209-research.md](./issue-209-research.md)

Stacked on: [PR #208](https://github.com/coder/agentapi/pull/208) (`fix/write-stabilize-non-fatal-phase1`)

## Problem

`Send()` hangs indefinitely when `ReadyForInitialPrompt` never returns `true`, even though `Status()` reports `"stable"`. This is because `statusLocked()` doesn't check `initialPromptReady`, allowing `Send()` to enqueue messages, but the `stableSignal` (which the send loop waits on) requires `initialPromptReady == true`. If readiness never arrives, the signal never fires and `Send()` blocks forever. (See research: Problem Context)

The real-world trigger is Claude Code v2.1.87's onboarding screen using `╌` (U+254C) instead of `─` (U+2500) in its box-drawing characters, causing message box detection to fail. (See research: Code Analysis — findGreaterThanMessageBox)

## Decisions

1. **Guard `statusLocked()` with `initialPromptReady` check AND fix detection.**
   - Options: (A) guard statusLocked only, (B) decouple stableSignal, (C) fix detection only, (D) both A+C.
   - Chosen: D — both guard and detection.
   - Classification: Agent-recommended (issue author also recommends option D).
   - Reasoning: The `stableSignal` gates ALL outbound messages on `initialPromptReady`. `statusLocked()` must reflect this — otherwise status says "stable" but the system cannot process messages. The detection fix handles the immediate trigger; the guard prevents the class of bugs.

2. **Return `ConversationStatusChanging` (not `Initializing`) when `initialPromptReady` is false.**
   - Options: `changing` vs `initializing`.
   - Chosen: `changing`.
   - Classification: Agent-recommended.
   - Reasoning: The snapshot buffer IS full (past the `initializing` phase). The error `ErrMessageValidationChanging` says "message can only be sent when the agent is waiting for user input" — which is semantically correct when readiness hasn't been detected.

3. **Apply the guard unconditionally (not only when `InitialPrompt` is configured).**
   - Options: Conditional (only when InitialPrompt set) vs unconditional.
   - Chosen: Unconditional.
   - Classification: Agent-recommended.
   - Reasoning: The `stableSignal` gates on `initialPromptReady` for ALL queued messages, not just the initial prompt. If `initialPromptReady` is false and a user calls `Send()`, the message hangs regardless of whether `InitialPrompt` is configured. Status must reflect actual send capability. When `ReadyForInitialPrompt` is nil (the default), it auto-returns `true` and `initialPromptReady` becomes `true` on the first snapshot tick — before status could ever transition to `stable`. So the unconditional guard causes no regressions for the default case. (See research: Approach A — Nuance)

4. **Add `╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌` (U+254C repeated) as an alternative pattern in message box detection.**
   - Options: just U+254C, also U+254D, broad set of horizontal box-drawing characters.
   - Chosen: U+254C only.
   - Classification: Agent-recommended.
   - Reasoning: This is the specific character seen in the wild. The `statusLocked()` guard provides the safety net for future unknown characters. Adding a broad set risks false positives.

5. **Update existing tests that assert `stable` when `initialPromptReady` is false.**
   - The tests `"agent not ready - status is stable until agent becomes ready"` and `"no initial prompt - normal status logic applies"` currently assert `stable` when readiness is false. The research notes the second test's assertion was correct from a pure screen-stability perspective, but is inconsistent with `Send()` behavior: `stableSignal` gates on `initialPromptReady` for ALL messages, so `Send()` would hang despite `stable` status. The status must reflect actual send capability.
   - The test `"no initial prompt configured - normal status logic applies"` is **unaffected** because it doesn't set `ReadyForInitialPrompt`, so the default (`return true`) applies and `initialPromptReady` becomes `true` before stability is reached.
   - Classification: Agent-recommended.

## Implementation Flow

1. **Message box detection** — Extend both message box detection functions to also match `╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌` patterns. Add a testdata fixture for the Claude onboarding screen with `╌` characters. Run readiness tests to verify.

2. **Status guard** — Add a check in the status logic: when `initialPromptReady` is false and the screen is otherwise stable, return `changing` instead of `stable`. This prevents `Send()` from enqueueing messages that can never be processed.

3. **Update existing tests** — Fix the two tests that assert `stable` when readiness is false. Update them to expect `changing`. Confirm the third "no initial prompt configured" test is unaffected.

4. **Add reproducing test** — Add a test that demonstrates `Send()` returns an error instead of hanging when `initialPromptReady` is false.

5. **Run full test suite** — Verify no regressions.
