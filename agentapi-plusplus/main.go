package main

//go:generate sh -c "go run main.go server --print-openapi dummy > openapi.json"
//go:generate ./version.sh
import "github.com/coder/agentapi/cmd"

func main() {
	cmd.Execute()
}
