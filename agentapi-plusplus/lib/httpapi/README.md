The `chat/marker` file allows the agentapi binary to be built without
populating the chat directory with real files. If the directory was empty,
`go build` would fail.
