package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/coder/agentapi/internal/version"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
	"golang.org/x/xerrors"
)

// getLogs handles GET /logs
func (s *Server) getLogs(ctx context.Context, input *struct{}) (*LogsResponse, error) {
	resp := &LogsResponse{}
	resp.Body.Logs = []string{}
	return resp, nil
}

// getRateLimit handles GET /rate-limit
func (s *Server) getRateLimit(ctx context.Context, input *struct{}) (*RateLimitResponse, error) {
	resp := &RateLimitResponse{}
	resp.Body.Enabled = false
	resp.Body.Requests = 100
	return resp, nil
}

// getConfig handles GET /config
func (s *Server) getConfig(ctx context.Context, input *struct{}) (*ConfigResponse, error) {
	resp := &ConfigResponse{}
	resp.Body.AgentType = string(s.agentType)
	resp.Body.Port = s.port
	return resp, nil
}

// getHealth handles GET /health
func (s *Server) getHealth(ctx context.Context, input *struct{}) (*HealthResponse, error) {
	resp := &HealthResponse{}
	resp.Body.Status = "ok"
	return resp, nil
}

// getVersion handles GET /version
func (s *Server) getVersion(ctx context.Context, input *struct{}) (*VersionResponse, error) {
	resp := &VersionResponse{}
	resp.Body.Version = version.Version
	return resp, nil
}

// getReady handles GET /ready
func (s *Server) getReady(ctx context.Context, input *struct{}) (*ReadyResponse, error) {
	resp := &ReadyResponse{}
	resp.Body.Ready = true
	return resp, nil
}

// getInfo handles GET /info
func (s *Server) getInfo(ctx context.Context, input *struct{}) (*InfoResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &InfoResponse{}
	resp.Body.Version = version.Version
	resp.Body.AgentType = s.agentType
	resp.Body.Features = map[string]bool{
		"messages":   true,
		"events":     true,
		"upload":     true,
		"pagination": true,
		"slashCmd":   true,
	}
	return resp, nil
}

// getStatus handles GET /status
func (s *Server) getStatus(ctx context.Context, input *struct{}) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.conversation.Status()
	agentStatus, err := convertStatus(status)
	if err != nil {
		return nil, xerrors.Errorf("failed to convert status: %w", err)
	}

	resp := &StatusResponse{}
	resp.Body.Status = agentStatus
	resp.Body.AgentType = s.agentType

	return resp, nil
}

// getMessages handles GET /messages
//
//	@param after (query) int "Return messages after this ID"
//	@param offset (query) int "Skip the first N messages"
//	@param limit (query) int "Limit number of messages returned"
func (s *Server) getMessages(ctx context.Context, input *struct {
	After  *int `json:"after,optional"`
	Offset *int `json:"offset,optional"`
	Limit  *int `json:"limit,optional"`
}) (*MessagesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allMessages := s.conversation.Messages()

	// Filter by 'after' parameter
	messages := allMessages
	if input.After != nil {
		afterID := *input.After
		filtered := make([]st.ConversationMessage, 0)
		for _, msg := range allMessages {
			if msg.Id > afterID {
				filtered = append(filtered, msg)
			}
		}
		messages = filtered
	}

	// Apply offset
	if input.Offset != nil && *input.Offset > 0 {
		offset := *input.Offset
		if offset >= len(messages) {
			messages = []st.ConversationMessage{}
		} else {
			messages = messages[offset:]
		}
	}

	// Apply limit
	if input.Limit != nil && *input.Limit > 0 {
		limit := *input.Limit
		if len(messages) > limit {
			messages = messages[:limit]
		}
	}

	resp := &MessagesResponse{}
	resp.Body.Messages = make([]Message, len(messages))
	for i, msg := range messages {
		resp.Body.Messages[i] = Message{
			Id:      msg.Id,
			Role:    msg.Role,
			Content: msg.Message,
			Time:    msg.Time,
		}
	}

	return resp, nil
}

// clearMessages handles DELETE /messages
func (s *Server) clearMessages(ctx context.Context, input *struct{}) (*MessagesClearResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := &MessagesClearResponse{}
	count := len(s.conversation.Messages())
	if clearer, ok := any(s.conversation).(interface{ ClearMessages() }); ok {
		clearer.ClearMessages()
	}
	resp.Body.Ok = true
	resp.Body.Count = count
	return resp, nil
}

// getMessagesCount handles GET /messages/count
func (s *Server) getMessagesCount(ctx context.Context, input *struct{}) (*MessagesCountResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &MessagesCountResponse{}
	resp.Body.Count = len(s.conversation.Messages())
	return resp, nil
}

// createMessage handles POST /message
func (s *Server) createMessage(ctx context.Context, input *MessageRequest) (*MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch input.Body.Type {
	case MessageTypeUser:
		if err := s.conversation.Send(FormatMessage(s.agentType, input.Body.Content)...); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case MessageTypeRaw:
		if _, err := s.agentio.Write([]byte(input.Body.Content)); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case MessageTypeCommand:
		// Send slash command directly - add enter at the end
		content := input.Body.Content
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if _, err := s.agentio.Write([]byte(content)); err != nil {
			return nil, xerrors.Errorf("failed to send command: %w", err)
		}
	}

	resp := &MessageResponse{}
	resp.Body.Ok = true

	return resp, nil
}

// uploadFiles handles POST /upload
func (s *Server) uploadFiles(ctx context.Context, input *struct {
	RawBody huma.MultipartFormFiles[UploadRequest]
},
) (*UploadResponse, error) {
	formData := input.RawBody.Data()

	file := formData.File.File

	// Limit file size to 10MB
	const maxFileSize = 10 << 20 // 10MB
	buf, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return nil, xerrors.Errorf("failed to upload file: %w", err)
	}
	if len(buf) > maxFileSize {
		return nil, huma.Error400BadRequest("file size exceeds 10MB limit")
	}

	// Calculate checksum of the uploaded file to create unique subdirectory
	hash := sha256.Sum256(buf)
	checksum := hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)

	// Create checksum-based subdirectory in tempDir
	uploadDir := filepath.Join(s.tempDir, checksum)
	err = os.MkdirAll(uploadDir, 0o755)
	if err != nil {
		return nil, xerrors.Errorf("failed to create upload directory: %w", err)
	}

	// Save individual file with original filename (extract just the base filename for security)
	filename := filepath.Base(formData.File.Filename)

	outPath := filepath.Join(uploadDir, filename)
	err = os.WriteFile(outPath, buf, 0o644)
	if err != nil {
		return nil, xerrors.Errorf("failed to write file: %w", err)
	}

	resp := &UploadResponse{}
	resp.Body.Ok = true
	resp.Body.FilePath = outPath
	return resp, nil
}

// subscribeEvents is an SSE endpoint that sends events to the client
func (s *Server) subscribeEvents(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(subscriberId)
	s.logger.Info("New subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type == EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			s.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
			return
		}
	}

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				s.logger.Info("Channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type == EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				s.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Context done", "subscriberId", subscriberId)
			return
		}
	}
}

func (s *Server) subscribeScreen(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(subscriberId)
	s.logger.Info("New screen subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type != EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			s.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				s.logger.Info("Screen channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type != EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				s.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Screen context done", "subscriberId", subscriberId)
			return
		}
	}
}

func (s *Server) redirectToChat(w http.ResponseWriter, r *http.Request) {
	rdir, err := url.JoinPath(s.chatBasePath, "embed")
	if err != nil {
		s.logger.Error("Failed to construct redirect URL", "error", err)
		http.Error(w, "Failed to redirect", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, rdir, http.StatusTemporaryRedirect)
}
