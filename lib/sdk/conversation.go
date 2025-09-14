package sdk

import "github.com/coder/agentapi/lib/types"

type SDK interface {
	QueryAgent(userInput string) (string, error)
	InitializeAgent(interface{}) error
	GetStatus() (*types.StatusResponse, error)
}
