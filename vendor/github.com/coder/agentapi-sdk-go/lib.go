package agentapisdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/coder/agentapi-sdk-go/gen"
	"github.com/tmaxmax/go-sse"
)

type Client struct {
	client gen.ClientWithResponses
}

type ClientOption = gen.ClientOption

// HTTPDoer is an interface for performing HTTP requests.
// The standard http.Client implements this interface.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// WithHTTPClient allows overriding the default HTTP client.
// This is useful for testing with mock clients.
func WithHTTPClient(client HTTPDoer) ClientOption {
	return gen.WithHTTPClient(client)
}

// RequestEditorFn is a function that modifies HTTP requests before they are sent.
type RequestEditorFn = gen.RequestEditorFn

// WithRequestEditorFn adds a function that will be called to modify each request before sending.
// This can be used to add headers, modify query parameters, etc.
func WithRequestEditorFn(fn RequestEditorFn) ClientOption {
	return gen.WithRequestEditorFn(fn)
}

// WithBaseURL overrides the base URL for the client.
// This is useful when you want to use a different server than the one provided to NewClient.
func WithBaseURL(baseURL string) ClientOption {
	return gen.WithBaseURL(baseURL)
}

func NewClient(server string, opts ...ClientOption) (*Client, error) {
	client, err := gen.NewClientWithResponses(server, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{*client}, nil
}

type PostMessageParams = gen.PostMessageJSONRequestBody
type PostMessageResponse = gen.MessageResponseBody

func (c *Client) PostMessage(ctx context.Context, body PostMessageParams) (*PostMessageResponse, error) {
	res, err := c.client.PostMessageWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if res.JSON200 == nil {
		return nil, fmt.Errorf("failed to post message: %s", res.Body)
	}
	return res.JSON200, nil
}

type GetMessagesResponse = gen.MessagesResponseBody

func (c *Client) GetMessages(ctx context.Context) (*GetMessagesResponse, error) {
	res, err := c.client.GetMessagesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if res.JSON200 == nil {
		return nil, fmt.Errorf("failed to get messages: %s", res.Body)
	}
	return res.JSON200, nil
}

type GetStatusResponse = gen.StatusResponseBody

func (c *Client) GetStatus(ctx context.Context) (*GetStatusResponse, error) {
	res, err := c.client.GetStatusWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if res.JSON200 == nil {
		return nil, fmt.Errorf("failed to get status: %s", res.Body)
	}
	return res.JSON200, nil
}

// Event represents a server-sent event from the Agent API
type Event = any

// EventMessageUpdate represents a message update event
type EventMessageUpdate = gen.MessageUpdateBody

// EventStatusChange represents a status change event
type EventStatusChange = gen.StatusChangeBody

// Constants for agent status
const (
	// StatusRunning indicates the agent is actively processing
	StatusRunning AgentStatus = gen.Running
	// StatusStable indicates the agent is idle
	StatusStable AgentStatus = gen.Stable
)

// Constants for conversation roles
const (
	// RoleAgent is the role assigned to agent messages
	RoleAgent ConversationRole = gen.ConversationRoleAgent
	// RoleUser is the role assigned to user messages
	RoleUser ConversationRole = gen.ConversationRoleUser
)

// Constants for message types
const (
	// MessageTypeRaw represents raw keystrokes sent to the agent
	MessageTypeRaw MessageType = gen.MessageTypeRaw
	// MessageTypeUser represents a user message
	MessageTypeUser MessageType = gen.MessageTypeUser
)

// AgentStatus represents the current state of the agent
type AgentStatus = gen.AgentStatus

// ConversationRole defines the sender of a message (agent or user)
type ConversationRole = gen.ConversationRole

// MessageType defines the type of message being sent
type MessageType = gen.MessageType

// Message represents a single message in the conversation
type Message = gen.Message

// ErrorModel represents an error returned by the API
type ErrorModel = gen.ErrorModel

// ErrorDetail provides additional information about an error
type ErrorDetail = gen.ErrorDetail

type SubscribeEventsResponse = gen.SubscribeEventsResponse

type EventType = string

const (
	EventTypeMessageUpdate EventType = "message_update"
	EventTypeStatusChange  EventType = "status_change"
)

func (c *Client) SubscribeEvents(ctx context.Context) (chan Event, chan error, error) {
	resp, err := c.client.SubscribeEvents(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to subscribe to events: status code %d, body: %s", resp.StatusCode, string(body))
	}

	ch := make(chan Event, 16)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)
		defer resp.Body.Close()

		// A message update may be as large as 80,000 bytes if the agent's
		// response takes up the entire terminal screen.
		readCfg := &sse.ReadConfig{
			MaxEventSize: 1024 * 128, // 128KB
		}

		// We don't need to explicitly handle the context cancellation here
		// because the response body will be closed when the context is
		// cancelled.
		for ev, err := range sse.Read(resp.Body, readCfg) {
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				errCh <- err
				return
			}

			switch ev.Type {
			case EventTypeMessageUpdate:
				var messageUpdate EventMessageUpdate
				err = json.Unmarshal([]byte(ev.Data), &messageUpdate)
				if err != nil {
					errCh <- err
					return
				}
				ch <- messageUpdate
			case EventTypeStatusChange:
				var statusChange EventStatusChange
				err = json.Unmarshal([]byte(ev.Data), &statusChange)
				if err != nil {
					errCh <- err
					return
				}
				ch <- statusChange
			default:
				errCh <- fmt.Errorf("unknown event type: %s, data: %s", ev.Type, ev.Data)
				return
			}
		}

	}()

	return ch, errCh, nil
}
