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
	request   chatRequest
	response  chatResponse
	startTime time.Time
	endTime   time.Time
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
func (oi *ChatInterceptor) ResponseInterceptor(_ *http.Response, _ interceptor.State) error {
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

	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 1. Convert messages for FindBranchByHistory
		history := make([]struct{ Role, Content string }, len(ollamaState.request.Messages))
		for i, m := range ollamaState.request.Messages {
			history[i] = struct{ Role, Content string }{Role: m.Role, Content: m.Content}
		}

		// 2. Try to find existing branch or create new conversation
		// For simplicity, we'll use a fixed conversation ID or handle it based on some session ID if available.
		// Since we don't have a session ID yet, let's see how AddMessage is supposed to work.
		// storage.AddMessage(ctx, conversationID, branchID, role, content)
		// Actually, FindBranchByHistory should find the branch that corresponds to the history.

		// Let's just log for now if storage is present, and maybe implement a basic save.
		// If it's a new conversation, we'd need to create it.
		// For now, let's try to find a branch that matches the history.
		branchID, err := oi.Storage.FindBranchByHistory(ctx, "", history)
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not find branch by history", oi.Name)
		}

		// If we found a branch, we add the assistant response to it.
		if branchID != "" {
			_, err = oi.Storage.AddMessage(ctx, "", branchID, ollamaState.response.Message.Role, ollamaState.response.Message.Content)
			if err != nil {
				logrus.WithError(err).Warnf("[%s] Could not add assistant message to storage", oi.Name)
			}
		} else {
			// New conversation
			conv, branch, err := oi.Storage.CreateConversation(ctx, map[string]any{"model": ollamaState.request.Model})
			if err != nil {
				logrus.WithError(err).Warnf("[%s] Could not create conversation in storage", oi.Name)
			} else {
				// Add all messages to the new branch
				for _, m := range ollamaState.request.Messages {
					_, _ = oi.Storage.AddMessage(ctx, conv.ID, branch.ID, m.Role, m.Content)
				}
				// Add the assistant response
				_, _ = oi.Storage.AddMessage(ctx, conv.ID, branch.ID, ollamaState.response.Message.Role, ollamaState.response.Message.Content)
			}
		}
	}

	for _, m := range ollamaState.request.Messages {
		logrus.Printf("[%s] Request [%s]: %s", oi.Name, m.Role, m.Content)
	}
	logrus.Printf("[%s] Response [%s]: %s", oi.Name, ollamaState.response.Message.Role, ollamaState.response.Message.Content)
}

// OnError handles errors during request processing
func (oi *ChatInterceptor) OnError(_ interceptor.State, err error) {
	logrus.WithError(err).Warningf("[%s] Error occurred", oi.Name)
}
