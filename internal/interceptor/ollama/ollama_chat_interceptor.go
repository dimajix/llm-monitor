package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"llm-monitor/internal/interceptor"
	"llm-monitor/internal/storage"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// ChatInterceptor intercepts chat messages between client and Ollama server
type ChatInterceptor struct {
	Name    string
	Storage storage.Storage
}

// chatMessage represents a chat message
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest represents the structure of a chat request
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// chatResponse represents the structure of a chat response
type chatResponse struct {
	Model              string      `json:"model"`
	CreatedAt          string      `json:"created_at"`
	Message            chatMessage `json:"message"`
	Done               bool        `json:"done"`
	DoneReason         string      `json:"done_reason,omitempty"`
	TotalDuration      int64       `json:"total_duration,omitempty"`
	LoadDuration       int64       `json:"load_duration,omitempty"`
	PromptEvalCount    int         `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64       `json:"prompt_eval_duration,omitempty"`
	EvalCount          int         `json:"eval_count,omitempty"`
	EvalDuration       int64       `json:"eval_duration,omitempty"`
}

// chatState holds the state information for Ollama requests
type chatState struct {
	request    chatRequest
	response   chatResponse
	startTime  time.Time
	endTime    time.Time
	statusCode int
}

// CreateState creates a new state for the interceptor
func (oi *ChatInterceptor) CreateState() interceptor.State {
	return &chatState{
		startTime: time.Now(),
	}
}

// RequestInterceptor intercepts the request to extract model and context information
func (oi *ChatInterceptor) RequestInterceptor(req *http.Request, state interceptor.State) error {
	logrus.Printf("[%s] Intercepting request to %s", oi.Name, req.URL.Path)

	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(req.Body)

	// Extract model name
	ollamaState, _ := state.(*chatState)

	// Parse the chat request
	var chatReq chatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse request body", oi.Name)
	} else {
		ollamaState.request = chatReq
	}

	// Create a new body reader
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return nil
}

// ResponseInterceptor intercepts the response to extract response messages
func (oi *ChatInterceptor) ResponseInterceptor(resp *http.Response, state interceptor.State) error {
	ollamaState, _ := state.(*chatState)
	ollamaState.statusCode = resp.StatusCode
	return nil
}

// ContentInterceptor intercepts content to extract streaming messages
func (oi *ChatInterceptor) ContentInterceptor(content []byte, state interceptor.State) ([]byte, error) {
	ollamaState, _ := state.(*chatState)

	// Parse the streaming response
	var chatResp chatResponse
	if err := json.Unmarshal(content, &chatResp); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse response body", oi.Name)
	} else {
		ollamaState.response = chatResp
	}

	return content, nil
}

// ChunkInterceptor intercepts chunks for streaming responses
func (oi *ChatInterceptor) ChunkInterceptor(chunk []byte, state interceptor.State) ([]byte, error) {
	ollamaState, _ := state.(*chatState)

	// Parse the response to extract details
	var chatResp chatResponse
	if err := json.Unmarshal(chunk, &chatResp); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse response chunk", oi.Name)
	} else {
		currentResponse := ollamaState.response.Message.Content + chatResp.Message.Content
		if chatResp.Done {
			ollamaState.response = chatResp
		}
		ollamaState.response.Message.Content = currentResponse
	}

	return chunk, nil
}

// OnComplete handles completion of the request
func (oi *ChatInterceptor) OnComplete(state interceptor.State) {
	ollamaState, _ := state.(*chatState)

	logrus.Printf("[%s] Request completed for model: %s", oi.Name, ollamaState.response.Model)
	for _, m := range ollamaState.request.Messages {
		logrus.Printf("[%s] Request [%s]: %s", oi.Name, m.Role, m.Content)
	}
	logrus.Printf("[%s] Response [%s]: %s", oi.Name, ollamaState.response.Message.Role, ollamaState.response.Message.Content)

	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		oi.saveToStorage(ctx, ollamaState)
	}
}

// OnError handles errors during request processing
func (oi *ChatInterceptor) OnError(state interceptor.State, err error) {
	ollamaState, _ := state.(*chatState)
	logrus.WithError(err).Warningf("[%s] Error occurred", oi.Name)

	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		oi.saveToStorage(ctx, ollamaState)
	}
}

func (oi *ChatInterceptor) saveToStorage(ctx context.Context, ollamaState *chatState) {
	// 1. Convert messages for FindMessageByHistory
	history := make([]struct{ Role, Content string }, len(ollamaState.request.Messages))
	for i, m := range ollamaState.request.Messages {
		history[i] = struct{ Role, Content string }{Role: m.Role, Content: m.Content}
	}

	// 2. Try to find the deepest matching message ID
	var currentParentID string
	var currentBranchID string

	var curHistory = history
	for len(curHistory) > 0 {
		pid, err := oi.Storage.FindMessageByHistory(ctx, curHistory)
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not find message by history", oi.Name)
			return
		}
		if pid != "" {
			currentParentID = pid
			break
		}
		newLen := len(curHistory) - 1
		if newLen <= 0 {
			currentParentID = ""
			break
		}
		curHistory = curHistory[0:newLen]
	}

	// Create new conversation if no message is found
	if currentParentID == "" {
		// New conversation
		_, branch, err := oi.Storage.CreateConversation(ctx, map[string]any{"model": ollamaState.request.Model})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not create conversation in storage", oi.Name)
			return
		}
		currentBranchID = branch.ID
	}

	// 3. Add missing messages from history
	// We need to find where history diverges from what's already in the DB.
	// FindMessageByHistory only gives us the last matched ID.
	// To be efficient, we can check how many messages from the beginning are already in the DB.
	// For now, let's just iterate through history and let AddMessage handle idempotency via hash.
	for i, m := range ollamaState.request.Messages[len(curHistory):] {
		msg, err := oi.Storage.AddMessage(ctx, currentParentID, &storage.Message{
			Role:     m.Role,
			Content:  m.Content,
			BranchID: currentBranchID,
		})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not add history message %d to storage", oi.Name, i)
			return
		}
		currentParentID = msg.ID
		currentBranchID = "" // Only need it for the first message if no parent
	}

	// 4. Add the assistant response
	_, err := oi.Storage.AddMessage(ctx, currentParentID, &storage.Message{
		Role:               ollamaState.response.Message.Role,
		Content:            ollamaState.response.Message.Content,
		UpstreamStatusCode: ollamaState.statusCode,
	})
	if err != nil {
		logrus.WithError(err).Warnf("[%s] Could not add assistant message to storage", oi.Name)
	}
}
