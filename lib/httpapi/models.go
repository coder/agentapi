package httpapi

import (
	"time"

	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/agentapi/lib/util"
	"github.com/danielgtaylor/huma/v2"
)

type MessageType string

const (
	MessageTypeUser MessageType = "user"
	MessageTypeRaw  MessageType = "raw"
)

var MessageTypeValues = []MessageType{
	MessageTypeUser,
	MessageTypeRaw,
}

func (m MessageType) Schema(r huma.Registry) *huma.Schema {
	return util.OpenAPISchema(r, "MessageType", MessageTypeValues)
}

// Message represents a message
type Message struct {
	Id      int                 `json:"id" doc:"Unique identifier for the message. This identifier also represents the order of the message in the conversation history."`
	Content string              `json:"content" example:"Hello world" doc:"Message content. The message is formatted as it appears in the agent's terminal session, meaning that, by default, it consists of lines of text with 80 characters per line."`
	Role    st.ConversationRole `json:"role" doc:"Role of the message author"`
	Time    time.Time           `json:"time" doc:"Timestamp of the message"`
}

// StatusResponse represents the server status
type StatusResponse struct {
	Body struct {
		Status AgentStatus `json:"status" doc:"Current agent status. 'running' means that the agent is processing a message, 'stable' means that the agent is idle and waiting for input."`
	}
}

// MessagesResponse represents the list of messages
type MessagesResponse struct {
	Body struct {
		Messages []Message `json:"messages" nullable:"false" doc:"List of messages"`
	}
}

type MessageRequestBody struct {
	Content string      `json:"content" example:"Hello, agent!" doc:"Message content"`
	Type    MessageType `json:"type" doc:"A 'user' type message will be logged as a user message in the conversation history and submitted to the agent. AgentAPI will wait until the agent starts carrying out the task described in the message before responding. A 'raw' type message will be written directly to the agent's terminal session as keystrokes and will not be saved in the conversation history. 'raw' messages are useful for sending escape sequences to the terminal."`
}

// MessageRequest represents a request to create a new message
type MessageRequest struct {
	Body MessageRequestBody `json:"body" doc:"Message content and type"`
}

// MessageResponse represents a newly created message
type MessageResponse struct {
	Body struct {
		Ok bool `json:"ok" doc:"Indicates whether the message was sent successfully. For messages of type 'user', success means detecting that the agent began executing the task described. For messages of type 'raw', success means the keystrokes were sent to the terminal."`
	}
}

type UploadResponse struct {
	Body struct {
		Ok       bool   `json:"ok" doc:"Indicates whether the files were uploaded successfully."`
		FilePath string `json:"filePath" doc:"Path of the file"`
	}
}

type UploadRequest struct {
	File huma.FormFile `form:"file" required:"true" doc:"file that needs to be uploaded"`
}
