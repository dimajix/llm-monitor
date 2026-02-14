package ollama

import (
	"bytes"
	"encoding/json"
	"io"
	"llm-monitor/internal/interceptor"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// ChatInterceptor intercepts chat messages between client and Ollama server
type ChatInterceptor struct {
	Name string
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the structure of a chat request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatResponse represents the structure of a chat response
type ChatResponse struct {
	Model              string      `json:"model"`
	CreatedAt          string      `json:"created_at"`
	Message            ChatMessage `json:"message"`
	Done               bool        `json:"done"`
	DoneReason         string      `json:"done_reason,omitempty"`
	TotalDuration      int64       `json:"total_duration,omitempty"`
	LoadDuration       int64       `json:"load_duration,omitempty"`
	PromptEvalCount    int         `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64       `json:"prompt_eval_duration,omitempty"`
	EvalCount          int         `json:"eval_count,omitempty"`
	EvalDuration       int64       `json:"eval_duration,omitempty"`
}

// ChatState holds the state information for Ollama requests
type ChatState struct {
	request   ChatRequest
	response  ChatResponse
	startTime time.Time
	endTime   time.Time
}

// CreateState creates a new state for the interceptor
func (oi *ChatInterceptor) CreateState() interceptor.State {
	return &ChatState{
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
	defer req.Body.Close()

	// Extract model name
	ollamaState, _ := state.(*ChatState)

	// Parse the chat request
	var chatReq ChatRequest
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
	return nil
}

// ContentInterceptor intercepts content to extract streaming messages
func (oi *ChatInterceptor) ContentInterceptor(content []byte, state interceptor.State) ([]byte, error) {
	ollamaState, _ := state.(*ChatState)

	// Parse the streaming response
	var chatResp ChatResponse
	if err := json.Unmarshal(content, &chatResp); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse response body", oi.Name)
	} else {
		ollamaState.response = chatResp
	}

	return content, nil
}

// ChunkInterceptor intercepts chunks for streaming responses
func (oi *ChatInterceptor) ChunkInterceptor(chunk []byte, state interceptor.State) ([]byte, error) {
	ollamaState, _ := state.(*ChatState)

	// Parse the response to extract details
	var chatResp ChatResponse
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
	ollamaState, _ := state.(*ChatState)
	logrus.Printf("[%s] Request completed for model: %s", oi.Name, ollamaState.response.Model)
	for _, m := range ollamaState.request.Messages {
		logrus.Printf("[%s] Request [%s]: %s", oi.Name, m.Role, m.Content)
	}
	logrus.Printf("[%s] Response [%s]: %s", oi.Name, ollamaState.response.Message.Role, ollamaState.response.Message.Content)
}

// OnError handles errors during request processing
func (oi *ChatInterceptor) OnError(state interceptor.State, err error) {
	logrus.WithError(err).Warningf("[%s] Error occurred", oi.Name)
}
