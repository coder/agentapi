package attach

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coder/agentapi/lib/httpapi"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/spf13/cobra"
	sse "github.com/tmaxmax/go-sse"
	"golang.org/x/term"
	"golang.org/x/xerrors"
)

type ChannelWriter struct {
	ch chan []byte
}

func (c *ChannelWriter) Write(p []byte) (n int, err error) {
	c.ch <- p
	return len(p), nil
}

func (c *ChannelWriter) Receive() ([]byte, bool) {
	data, ok := <-c.ch
	return data, ok
}

type model struct {
	screen string
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

type screenMsg struct {
	screen string
}

type finishMsg struct{}

//lint:ignore U1000 The Update function is used by the Bubble Tea framework
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case screenMsg:
		m.screen = msg.screen
		if m.screen != "" && m.screen[len(m.screen)-1] != '\n' {
			m.screen += "\n"
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case finishMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() string {
	return m.screen
}

// StatusBody matches the /status response structure
type StatusBody struct {
	Status    string `json:"status"`
	AgentType string `json:"agent_type"`
	Transport string `json:"transport"`
}

func GetStatus(ctx context.Context, url string) (StatusBody, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/status", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return StatusBody{}, xerrors.Errorf("failed to get status: %w", err)
	}
	defer res.Body.Close()

	var status StatusBody
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		return StatusBody{}, xerrors.Errorf("failed to decode status: %w", err)
	}
	return status, nil
}

func ReadScreenOverHTTP(ctx context.Context, url string, ch chan<- httpapi.ScreenUpdateBody) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	for ev, err := range sse.Read(res.Body, &sse.ReadConfig{
		// 256KB: screen can be big. The default terminal size is 80x1000,
		// which can be over 80000 bytes.
		MaxEventSize: 256 * 1024,
	}) {
		if err != nil {
			return xerrors.Errorf("failed to read sse: %w", err)
		}
		var screen httpapi.ScreenUpdateBody
		if err := json.Unmarshal([]byte(ev.Data), &screen); err != nil {
			return xerrors.Errorf("failed to unmarshal screen: %w", err)
		}
		ch <- screen
	}
	return nil
}

func WriteRawInputOverHTTP(ctx context.Context, url string, msg string) error {
	messageRequest := httpapi.MessageRequestBody{
		Type:    httpapi.MessageTypeRaw,
		Content: msg,
	}
	messageRequestBytes, err := json.Marshal(messageRequest)
	if err != nil {
		return xerrors.Errorf("failed to marshal message request: %w", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(messageRequestBytes))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK {
		return xerrors.Errorf("failed to write raw input: %w", errors.New(res.Status))
	}

	return nil
}

func WriteUserMessageOverHTTP(ctx context.Context, url string, msg string) error {
	messageRequest := httpapi.MessageRequestBody{
		Type:    httpapi.MessageTypeUser,
		Content: msg,
	}
	messageRequestBytes, err := json.Marshal(messageRequest)
	if err != nil {
		return xerrors.Errorf("failed to marshal message request: %w", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(messageRequestBytes))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed to do request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return xerrors.Errorf("failed to write user message: %w", errors.New(res.Status))
	}
	return nil
}

// ACPEvent represents an event from the /events SSE stream
type ACPEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ReadEventsOverHTTP reads events from /events SSE stream and sends message updates to the channel
func ReadEventsOverHTTP(ctx context.Context, url string, ch chan<- httpapi.MessageUpdateBody) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "text/event-stream")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf("failed to connect to events stream: %w", err)
	}
	defer res.Body.Close()

	for ev, err := range sse.Read(res.Body, &sse.ReadConfig{
		MaxEventSize: 256 * 1024,
	}) {
		if err != nil {
			return xerrors.Errorf("failed to read sse: %w", err)
		}
		if ev.Type == string(httpapi.EventTypeMessageUpdate) {
			var msg httpapi.MessageUpdateBody
			if err := json.Unmarshal([]byte(ev.Data), &msg); err != nil {
				continue
			}
			ch <- msg
		}
	}
	return nil
}

func runAttach(remoteUrl string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdin := int(os.Stdin.Fd())

	oldState, err := term.MakeRaw(stdin)
	if err != nil {
		return xerrors.Errorf("failed to make raw: %w", err)
	}
	defer func() {
		_ = term.Restore(stdin, oldState)
	}()

	stdinWriter := &ChannelWriter{
		ch: make(chan []byte, 4096),
	}
	tee := io.TeeReader(os.Stdin, stdinWriter)
	p := tea.NewProgram(model{}, tea.WithInput(tee), tea.WithAltScreen())
	screenCh := make(chan httpapi.ScreenUpdateBody, 64)

	readScreenErrCh := make(chan error, 1)
	go func() {
		defer close(readScreenErrCh)
		if err := ReadScreenOverHTTP(ctx, remoteUrl+"/internal/screen", screenCh); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			readScreenErrCh <- xerrors.Errorf("failed to read screen: %w", err)
		}
	}()
	writeRawInputErrCh := make(chan error, 1)
	go func() {
		defer close(writeRawInputErrCh)
		for {
			select {
			case <-ctx.Done():
				return
			case buf, ok := <-stdinWriter.ch:
				if !ok {
					return
				}
				input := string(buf)
				// Don't send Ctrl+C to the agent
				if input == "\x03" {
					continue
				}
				if err := WriteRawInputOverHTTP(ctx, remoteUrl+"/message", input); err != nil {
					writeRawInputErrCh <- xerrors.Errorf("failed to write raw input: %w", err)
					return
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case screenUpdate, ok := <-screenCh:
				if !ok {
					return
				}
				p.Send(screenMsg{
					screen: screenUpdate.Screen,
				})
			}
		}
	}()
	pErrCh := make(chan error, 1)
	go func() {
		_, err := p.Run()
		pErrCh <- err
		close(pErrCh)
	}()

	select {
	case err = <-readScreenErrCh:
	case err = <-writeRawInputErrCh:
	case err = <-pErrCh:
	case <-ctx.Done():
		err = nil
	}

	p.Send(finishMsg{})
	select {
	case <-pErrCh:
	case <-time.After(1 * time.Second):
	}

	return err
}

// runAttachACP handles attach for ACP transport using the /events stream
func runAttachACP(remoteUrl string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track messages by ID for rendering
	messages := make(map[int]httpapi.MessageUpdateBody)
	var maxMsgID int

	msgCh := make(chan httpapi.MessageUpdateBody, 64)
	readEventsErrCh := make(chan error, 1)
	go func() {
		defer close(readEventsErrCh)
		if err := ReadEventsOverHTTP(ctx, remoteUrl+"/events", msgCh); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			readEventsErrCh <- err
		}
	}()

	// Render messages to terminal
	renderMessages := func() {
		// Clear screen and move cursor to top
		fmt.Print("\033[2J\033[H")
		for i := 0; i <= maxMsgID; i++ {
			if msg, ok := messages[i]; ok {
				role := "User"
				if msg.Role == st.ConversationRoleAgent {
					role = "Agent"
				}
				fmt.Printf("── %s ──\n%s\n\n", role, msg.Message)
			}
		}
		fmt.Print("Press Ctrl+C to exit, or type a message and press Enter:\n> ")
	}

	// Initial render
	renderMessages()

	// Handle input in a separate goroutine
	inputCh := make(chan string, 1)
	inputErrCh := make(chan error, 1)
	go func() {
		defer close(inputErrCh)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				inputCh <- line
			}
		}
		if err := scanner.Err(); err != nil {
			inputErrCh <- err
		}
	}()

	for {
		select {
		case msg := <-msgCh:
			messages[msg.Id] = msg
			if msg.Id > maxMsgID {
				maxMsgID = msg.Id
			}
			renderMessages()
		case line := <-inputCh:
			if err := WriteUserMessageOverHTTP(ctx, remoteUrl+"/message", line); err != nil {
				return xerrors.Errorf("failed to send message: %w", err)
			}
		case err := <-readEventsErrCh:
			return err
		case err := <-inputErrCh:
			return err
		}
	}
}

var remoteUrlArg string

var AttachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to a running agent",
	Long:  `Attach to a running agent`,
	Run: func(cmd *cobra.Command, args []string) {
		remoteUrl := remoteUrlArg
		if remoteUrl == "" {
			fmt.Fprintln(os.Stderr, "URL is required")
			os.Exit(1)
		}
		if !strings.HasPrefix(remoteUrl, "http") {
			remoteUrl = "http://" + remoteUrl
		}
		remoteUrl = strings.TrimRight(remoteUrl, "/")

		// Check transport type
		status, err := GetStatus(context.Background(), remoteUrl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get server status: %+v\n", err)
			os.Exit(1)
		}

		if status.Transport == "acp" {
			err = runAttachACP(remoteUrl)
		} else {
			err = runAttach(remoteUrl)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Attach failed: %+v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	AttachCmd.Flags().StringVarP(&remoteUrlArg, "url", "u", "localhost:3284", "URL of the agentapi server to attach to. May optionally include a protocol and a path.")
}
