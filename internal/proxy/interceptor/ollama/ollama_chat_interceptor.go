package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	interceptor2 "llm-monitor/internal/proxy/interceptor"
	"llm-monitor/internal/storage"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// ChatInterceptor intercepts chat messages between client and Ollama server
type ChatInterceptor struct {
	interceptor2.SavingInterceptor
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
	request      chatRequest
	response     chatResponse
	startTime    time.Time
	endTime      time.Time
	statusCode   int
	clientHost   string
	upstreamHost string
}

// CreateState creates a new state for the interceptor
func (oi *ChatInterceptor) CreateState() interceptor2.State {
	return &chatState{
		startTime: time.Now(),
	}
}

// RequestInterceptor intercepts the request to extract model and context information
func (oi *ChatInterceptor) RequestInterceptor(req *http.Request, state interceptor2.State) error {
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
	ollamaState.upstreamHost = req.Host
	ollamaState.clientHost = req.Header.Get("X-Forwarded-For")

	// Parse the chat request
	var chatReq chatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse request body", oi.Name)
	} else {
		ollamaState.request = chatReq
	}

	// Store available request information
	oi.saveLog(ollamaState)

	// Create a new body reader
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return nil
}

// ResponseInterceptor intercepts the response to extract response messages
func (oi *ChatInterceptor) ResponseInterceptor(resp *http.Response, state interceptor2.State) error {
	ollamaState, _ := state.(*chatState)
	ollamaState.statusCode = resp.StatusCode
	return nil
}

// ContentInterceptor intercepts content to extract streaming messages
func (oi *ChatInterceptor) ContentInterceptor(content []byte, state interceptor2.State) ([]byte, error) {
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
func (oi *ChatInterceptor) ChunkInterceptor(chunk []byte, state interceptor2.State) ([]byte, error) {
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
func (oi *ChatInterceptor) OnComplete(state interceptor2.State) {
	ollamaState, _ := state.(*chatState)

	logrus.Printf("[%s] Request completed for model: %s", oi.Name, ollamaState.response.Model)
	for _, m := range ollamaState.request.Messages {
		logrus.Printf("[%s] Request [%s]: %s", oi.Name, m.Role, m.Content)
	}
	logrus.Printf("[%s] Response [%s]: %s", oi.Name, ollamaState.response.Message.Role, ollamaState.response.Message.Content)

	oi.saveLog(ollamaState)
}

// OnError handles errors during request processing
func (oi *ChatInterceptor) OnError(state interceptor2.State, err error) {
	ollamaState, _ := state.(*chatState)
	logrus.WithError(err).Warningf("[%s] Error occurred", oi.Name)
	for _, m := range ollamaState.request.Messages {
		logrus.Printf("[%s] Request [%s]: %s", oi.Name, m.Role, m.Content)
	}
	logrus.Printf("[%s] Response [%s]: %s", oi.Name, ollamaState.response.Message.Role, ollamaState.response.Message.Content)

	oi.saveLog(ollamaState)
}

func (oi *ChatInterceptor) saveLog(ollamaState *chatState) {
	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), oi.Timeout)
		defer cancel()

		history := make([]storage.SimpleMessage, len(ollamaState.request.Messages))
		for i, m := range ollamaState.request.Messages {
			history[i] = storage.SimpleMessage{Role: m.Role, Content: m.Content, Model: ollamaState.request.Model, ClientHost: ollamaState.clientHost}
		}
		assistantMsg := storage.SimpleMessage{
			Role:               ollamaState.response.Message.Role,
			Content:            ollamaState.response.Message.Content,
			Model:              ollamaState.response.Model,
			PromptTokens:       ollamaState.response.PromptEvalCount,
			CompletionTokens:   ollamaState.response.EvalCount,
			PromptEvalDuration: time.Duration(ollamaState.response.PromptEvalDuration),
			EvalDuration:       time.Duration(ollamaState.response.EvalDuration),
			UpstreamHost:       ollamaState.upstreamHost,
		}

		oi.SaveToStorage(ctx, history, assistantMsg, ollamaState.statusCode, "chat")
	}
}
