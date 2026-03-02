package harness

// export_test.go — re-exports unexported symbols for white-box testing.
// This file is only compiled during `go test`.

var ParseTokens = parseTokens
var ParseCost = parseCost
