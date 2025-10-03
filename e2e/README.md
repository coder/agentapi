# End-to-End Testing Framework

This directory contains the end-to-end (E2E) testing framework for the AgentAPI project. The framework simulates realistic agent interactions using a script-based approach with JSON configuration files.

## TL;DR

```shell
go test ./e2e
```

## How it Works

The testing framework (`echo_test.go`) does the following:
- Reads a file in `testdata/`.
- Starts the AgentAPI server with a fake agent (`echo.go`). This fake agent reads the scripted conversation from the specified JSON file.
- The testing framework then sends messages to the fake agent.
- The fake agent validates the expected messages and sends predefined responses.
- The testing framework validates the the actual responses against expected outcomes.

## Adding a new test

1. Create a new JSON file in `testdata/` with a unique name.
2. Define the scripted conversation in the JSON file. Each message must have the following fields:
   - `expectMessage`: The message from the user that the fake agent expects.
   - `thinkDurationMS`: How long the fake agent should 'think' before responding.
   - `responseMessage`: The message the fake agent should respond with.
3. Add a new test case in `echo_test.go` that references the newly created JSON file.
  > Be sure that the name of the test case exactly matches the name of the JSON file.
4. Run the E2E tests to verify the new test case.
