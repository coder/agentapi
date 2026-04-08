# Research: Send() blocks indefinitely when ReadyForInitialPrompt returns false (#209)

## Problem Context

When `POST /message` with `type: "user"` is called, `Send()` hangs indefinitely if `ReadyForInitialPrompt` never returns `true` for the current screen content, even though `GET /status` returns `"stable"`.

**Root cause chain:**

1. `statusLocked()` does NOT check `initialPromptReady`. It returns `"stable"` when the screen is stable and the queue is empty.
2. `Send()` checks `statusLocked() != ConversationStatusStable` — this passes, so the message is enqueued.
3. The send loop blocks on `<-c.stableSignal`.
4. The snapshot loop only fires `stableSignal` when `c.initialPromptReady && len(c.outboundQueue) > 0 && c.isScreenStableLocked()`.
5. Since `initialPromptReady` is `false` and never becomes `true`, the signal never fires.
6. `Send()` blocks forever on `<-errCh`.

**Real-world trigger:** Claude Code v2.1.87 shows a theme selection onboarding screen using `╌╌╌` (U+254C, BOX DRAWINGS LIGHT DOUBLE DASH HORIZONTAL) instead of `───` (U+2500, BOX DRAWINGS LIGHT HORIZONTAL). The message box detection fails because `findGreaterThanMessageBox` / `findGenericSlimMessageBox` look for `───────────────` which isn't present. `ReadyForInitialPrompt` stays `false`.

## Code Analysis

### `statusLocked()` — `lib/screentracker/pty_conversation.go:505-537`

```go
func (c *PTYConversation) statusLocked() ConversationStatus {
    // ...sanity checks...
    snapshots := c.snapshotBuffer.GetAll()
    if len(c.messages) > 0 && c.messages[len(c.messages)-1].Role == ConversationRoleUser {
        return ConversationStatusChanging
    }
    if len(snapshots) != c.stableSnapshotsThreshold {
        return ConversationStatusInitializing
    }
    if !c.isScreenStableLocked() {
        return ConversationStatusChanging
    }
    // Handle initial prompt readiness: report "changing" until the queue is drained
    if len(c.outboundQueue) > 0 || c.sendingMessage {
        return ConversationStatusChanging
    }
    return ConversationStatusStable
}
```

**Key observation:** `initialPromptReady` is never consulted. The status can be `"stable"` even when `initialPromptReady` is `false`.

### `Send()` — `lib/screentracker/pty_conversation.go:358-378`

```go
func (c *PTYConversation) Send(messageParts ...MessagePart) error {
    // ...validation...
    c.lock.Lock()
    if c.statusLocked() != ConversationStatusStable {
        c.lock.Unlock()
        return ErrMessageValidationChanging
    }
    c.lock.Unlock()
    errCh := make(chan error, 1)
    c.outboundQueue <- outboundMessage{parts: messageParts, errCh: errCh}
    return <-errCh  // blocks forever if stableSignal never fires
}
```

### Snapshot loop signal logic — `lib/screentracker/pty_conversation.go:229-236`

```go
if c.initialPromptReady && len(c.outboundQueue) > 0 && c.isScreenStableLocked() {
    select {
    case c.stableSignal <- struct{}{}:
        c.sendingMessage = true
    default:
    }
}
```

### `findGreaterThanMessageBox` — `lib/msgfmt/message_box.go:11-22`

```go
func findGreaterThanMessageBox(lines []string) int {
    for i := len(lines) - 1; i >= max(len(lines)-6, 0); i-- {
        if strings.Contains(lines[i], ">") {
            if i > 0 && strings.Contains(lines[i-1], "───────────────") {
                return i - 1
            }
            return i
        }
    }
    return -1
}
```

Only checks for `───────────────` (U+2500). Does not handle `╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌` (U+254C).

### `findGenericSlimMessageBox` — `lib/msgfmt/message_box.go:28-38`

```go
func findGenericSlimMessageBox(lines []string) int {
    for i := len(lines) - 3; i >= max(len(lines)-9, 0); i-- {
        if strings.Contains(lines[i], "───────────────") &&
            (strings.Contains(lines[i+1], "|") || strings.Contains(lines[i+1], "│") || strings.Contains(lines[i+1], "❯")) &&
            strings.Contains(lines[i+2], "───────────────") {
            return i
        }
    }
    return -1
}
```

Same limitation — only checks for U+2500.

### `isGenericAgentReadyForInitialPrompt` — `lib/msgfmt/agent_readiness.go:34-38`

```go
func isGenericAgentReadyForInitialPrompt(message string) bool {
    message = trimEmptyLines(message)
    messageWithoutInputBox := removeMessageBox(message)
    return len(messageWithoutInputBox) != len(message)
}
```

Returns `true` only if `removeMessageBox` actually removes something. If neither message box detector matches, the message is unchanged and readiness returns `false`.

### Existing test: "agent not ready - status is stable" — `pty_conversation_test.go:~1082`

The existing test `TestInitialPromptReadiness/"agent not ready - status is stable until agent becomes ready"` already asserts that status is `stable` when the agent is not ready. This is the **current expected behavior** when there IS an initial prompt configured but the agent hasn't become ready yet.

However, this test ALSO has `InitialPrompt` configured. The issue scenario is different: `Send()` is called by the user (not as initial prompt) while `initialPromptReady` is `false`.

### Existing test: "no initial prompt - normal status logic applies" — `pty_conversation_test.go:~1160`

When `ReadyForInitialPrompt` always returns `false` AND there is no `InitialPrompt` configured, status is currently `stable`. From a pure screen-stability perspective this is correct — the screen IS stable.

However, `Send()` will still block in this state because `stableSignal` requires `initialPromptReady` to fire. This means status says `stable` but the system cannot actually process user messages — an inconsistency that is the root cause of the bug.

### Existing test: "no initial prompt configured - normal status logic applies" — `pty_conversation_test.go:~1207`

When `ReadyForInitialPrompt` is NOT set (nil → defaults to `return true`) and no `InitialPrompt` is configured, status correctly reaches `stable`. This test is UNAFFECTED by the fix because `initialPromptReady` becomes `true` on the first snapshot tick via the default function.

## Approaches

### Approach A: Guard `statusLocked()` with `initialPromptReady` check

**Description:** When `initialPromptReady` is `false`, return `ConversationStatusChanging` (or a new status) from `statusLocked()`. This prevents `Send()` from enqueueing and returns `ErrMessageValidationChanging` immediately.

**Precedent:** `statusLocked()` already returns `ConversationStatusChanging` when there are items in the outbound queue or a message is being sent. This follows the same pattern.

**Strongest argument for:** Fail-fast. Any future detection failures fail immediately with a clear error instead of hanging. This is a general safety net.

**Strongest argument against:** Changes the public status semantics. Currently, `statusLocked()` reports on screen state. Adding `initialPromptReady` couples it to agent detection. Also, callers currently expect `"stable"` to mean "screen is stable" — now it would also mean "agent detection succeeded". This could break the existing test `TestInitialPromptReadiness/"agent not ready - status is stable until agent becomes ready"` which explicitly asserts status is `stable` when readiness is `false`.

**Consideration:** The `stableSignal` only gates the signal when `initialPromptReady` is false. But this is orthogonal to the **user-initiated** `Send()` path. The initial prompt path and the user-message path both go through the same queue and same signal. The real issue is that `initialPromptReady` gates the signal for ALL queued messages, not just the initial prompt.

**Nuance:** We need to be careful about when `ReadyForInitialPrompt` is `nil` (defaults to `func(string) bool { return true }`). When there's no readiness function, `initialPromptReady` becomes `true` on the first snapshot tick. This won't cause regressions.

### Approach B: Decouple `stableSignal` from `initialPromptReady` for user-sent messages

**Description:** Only gate the `stableSignal` on `initialPromptReady` for the initial prompt. For user-sent messages, fire the signal based purely on screen stability. This could be done by tracking whether the queued message is the initial prompt or a user message.

**Strongest argument for:** Precisely targets the bug without changing status semantics. The initial prompt legitimately needs readiness gating; user messages do not.

**Strongest argument against:** Adds complexity to the queue/signal mechanism. The `outboundQueue` currently treats all messages uniformly. Adding message-type awareness complicates the design. Also, if the agent truly isn't ready, sending a user message to it may not work correctly anyway.

**What this makes easy:** Preserves existing status semantics and test assertions.
**What this makes hard:** Complicating the send path and potentially allowing messages to be sent to an unready agent.

### Approach C: Improve message box detection to handle `╌` (U+254C)

**Description:** Add `╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌` as an alternative pattern in `findGreaterThanMessageBox` and `findGenericSlimMessageBox`.

**Strongest argument for:** Fixes the specific real-world trigger. The onboarding screen for Claude Code v2.1.87 uses this character.

**Strongest argument against:** Only fixes this one variant. Future Claude Code versions might use yet another character. Does not prevent the indefinite hang for other detection failures.

**What this makes easy:** Simple, targeted fix.
**What this makes hard:** Doesn't address the systemic issue.

### Approach D: Both A and C (recommended by the issue)

**Description:** Fix the detection for this specific Claude Code version (C) AND add the `statusLocked()` guard (A) so future detection failures fail fast.

**Strongest argument for:** Belt and suspenders. Fixes the immediate problem and prevents the class of bugs.

**Strongest argument against:** Status semantics change (same as A). However, the issue author explicitly recommends this.

## Decisions

### Decision 1: Guard `statusLocked()` with `initialPromptReady` AND fix detection
- **Question:** How to prevent `Send()` from hanging when readiness detection fails?
- **Options:** (A) guard statusLocked only, (B) decouple stableSignal, (C) fix detection only, (D) both A+C
- **Chosen:** D — both guard and detection
- **Classification:** Agent-recommended (issue author also recommends option D)
- **Reasoning:** The `stableSignal` gates ALL outbound messages on `initialPromptReady`. `statusLocked()` must reflect this. The detection fix handles the immediate trigger; the guard prevents the class of bugs.

### Decision 2: Return `ConversationStatusChanging` when `initialPromptReady` is false
- **Question:** What status to return when readiness is false?
- **Options:** `changing` vs `initializing`
- **Chosen:** `changing`
- **Classification:** Agent-recommended
- **Reasoning:** The snapshot buffer IS full (past the `initializing` phase). `changing` matches the error `ErrMessageValidationChanging`.

### Decision 3: Apply the guard unconditionally (not only when `InitialPrompt` is configured)
- **Question:** Should the `initialPromptReady` guard only apply when `InitialPrompt` is configured? (Open Question 2)
- **Options:** Conditional (only when InitialPrompt set) vs unconditional
- **Chosen:** Unconditional
- **Classification:** Agent-recommended
- **Reasoning:** The `stableSignal` gates on `initialPromptReady` for ALL queued messages, not just the initial prompt. If `initialPromptReady` is false and a user calls `Send()`, the message hangs regardless of whether `InitialPrompt` is configured. Status must reflect actual send capability. When `ReadyForInitialPrompt` is nil (default), it auto-returns `true` and `initialPromptReady` becomes `true` on the first snapshot tick — before status could transition to `stable`. So the unconditional guard causes no regressions for the default case.

### Decision 4: Add U+254C only to detection
- **Question:** Which additional Unicode box-drawing characters to support?
- **Options:** Just U+254C, also U+254D, broad set
- **Chosen:** U+254C only
- **Classification:** Agent-recommended
- **Reasoning:** This is the specific character seen in the wild. The `statusLocked()` guard provides the safety net for future unknown characters.

## Open Questions

All open questions have been resolved — see Decisions section above.
