# thegent Warp/Operational Loop Guide

This guide defines the high-standard operational loop for developing `thegent`, inspired by the `Manager Pattern`.

## 1. Operational Loop (Manager Pattern)

1.  **Research**: Use `rg`, `fd`, and `task lint:arch` to understand the impact of your change.
2.  **Plan**: Create a brief implementation plan before touching code.
3.  **Test-First**: Create a failing test in `tests/` that describes the desired behavior.
4.  **Execute**: Implement the minimal change to satisfy the test.
5.  **Audit**: Run `task quality` to ensure no regressions or style violations.
6.  **Refactor**: If the code is working but messy, or if a module exceeds size limits, refactor immediately.

## 2. Code Modularity & Size Mandate

To maintain cognitive ease and high velocity, we enforce strict module size limits:

-   **Target Module Size**: ≤ 350 lines.
-   **Hard Limit**: 500 lines.
-   **Function Limit**: 40 lines.

If a module approaches 350 lines, it **must** be decomposed. Extract cohesive responsibilities into:
-   `thegent.core`: Foundational logic.
-   `thegent.services`: Domain-specific business logic.
-   `thegent.infrastructure`: External adapters and clients.

## 3. Test Naming Convention (Canonical)

Test files must describe **what** is being tested, not how.

✅ **Good (Concern-based)**:
-   `tests/test_run_lifecycle.py`
-   `tests/test_adapter_normalization.py`
-   `tests/test_policy_engine.py`

❌ **Bad (Metadata-based)**:
-   `tests/test_run_fast.py` (Use markers)
-   `tests/test_run_unit.py` (Use markers/fixtures)
-   `tests/test_run_v2.py` (Use git history)

## 4. Architectural Boundaries

Strict layer boundaries are enforced. Refer to `docs/ARCHITECTURE_LAYERS.md` and `tach.toml`.
-   `config` depends on nothing.
-   `contracts` depends only on `config`.
-   `cli` depends on everything except `mcp_server`.

## 5. Automation Commands

| Task | Command |
| :--- | :--- |
| **Setup** | `task setup` |
| **Start Dev** | `task dev` |
| **Run Tests** | `task test` |
| **Full Quality** | `task quality` |
| **Clean** | `task clean` |
