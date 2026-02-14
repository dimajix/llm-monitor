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

// GenerateInterceptor records traffic between a Client and an Ollama server
type GenerateInterceptor struct {
	Name    string
	Storage storage.Storage
}

// generateRequest represents the structure of a request to the /api/generate endpoint
type generateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// generateResponse represents the structure of a response from the /api/generate endpoint
type generateResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	DoneReason         string `json:"done_reason,omitempty"`
	Context            []int  `json:"context,omitempty"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

// generateState holds the state for an Ollama generate request
type generateState struct {
	request    generateRequest
	response   generateResponse
	startTime  time.Time
	endTime    time.Time
	statusCode int
}

// CreateState creates a new generateState for tracking requests
func (oi *GenerateInterceptor) CreateState() interceptor.State {
	return &generateState{
		startTime: time.Now(),
	}
}

// RequestInterceptor intercepts the request to /api/generate
func (oi *GenerateInterceptor) RequestInterceptor(req *http.Request, state interceptor.State) error {
	logrus.Printf("[%s] Intercepting request to %s", oi.Name, req.URL.Path)

	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(req.Body)

	// Store the request body in state
	ollamaState, _ := state.(*generateState)

	// Parse the request to extract model and prompt
	var generateReq generateRequest
	if err := json.Unmarshal(body, &generateReq); err != nil {
		logrus.WithError(err).Warningf("[%s] Could not parse request body: %v", oi.Name, err)
	} else {
		ollamaState.request = generateReq
	}

	// Replace the request body with the original content
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return nil
}

// ResponseInterceptor intercepts the response from /api/generate
func (oi *GenerateInterceptor) ResponseInterceptor(resp *http.Response, state interceptor.State) error {
	ollamaState, _ := state.(*generateState)
	ollamaState.statusCode = resp.StatusCode
	return nil
}

// ContentInterceptor intercepts content (not used for this specific interceptor)
func (oi *GenerateInterceptor) ContentInterceptor(content []byte, state interceptor.State) ([]byte, error) {
	ollamaState, _ := state.(*generateState)

	// Parse the response to extract details
	var generateResp generateResponse
	if err := json.Unmarshal(content, &generateResp); err != nil {
		logrus.WithError(err).Warningf("[%s] Could not parse response body: %v", oi.Name, err)
	} else {
		ollamaState.response = generateResp
	}

	return content, nil
}

// ChunkInterceptor intercepts chunks (not used for this specific interceptor)
func (oi *GenerateInterceptor) ChunkInterceptor(chunk []byte, state interceptor.State) ([]byte, error) {
	ollamaState, _ := state.(*generateState)

	// Parse the response to extract details
	var generateResp generateResponse
	if err := json.Unmarshal(chunk, &generateResp); err != nil {
		logrus.WithError(err).Warningf("[%s] Could not parse response chunk: %v", oi.Name, err)
	} else {
		currentResponse := ollamaState.response.Response + generateResp.Response
		if generateResp.Done {
			ollamaState.response = generateResp
		}
		ollamaState.response.Response = currentResponse
	}

	return chunk, nil
}

// OnComplete is called when the request is completed
func (oi *GenerateInterceptor) OnComplete(state interceptor.State) {
	ollamaState, _ := state.(*generateState)
	ollamaState.endTime = time.Now()

	logrus.Printf("[%s] Request completed for model: %s", oi.Name, ollamaState.response.Model)
	logrus.Printf("[%s] Prompt: %s", oi.Name, ollamaState.request.Prompt)
	logrus.Printf("[%s] Response: %s", oi.Name, ollamaState.response.Response)

	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		oi.saveToStorage(ctx, ollamaState)
	}
}

// OnError is called when an error occurs
func (oi *GenerateInterceptor) OnError(state interceptor.State, err error) {
	ollamaState, _ := state.(*generateState)
	logrus.WithError(err).Warningf("[%s] Error occurred: %v", oi.Name, err)

	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		oi.saveToStorage(ctx, ollamaState)
	}
}

func (oi *GenerateInterceptor) saveToStorage(ctx context.Context, ollamaState *generateState) {
	// For Generate, we have a Prompt (User) and a Response (Assistant)
	// We can't easily find a branch by history if we don't have the context or previous messages.
	// If Ollama provides context, we might be able to use it, but our current Storage
	// expects a history of (Role, Content).

	// Let's create a new conversation for each Generate request for now,
	// or try to match if history is just the prompt.
	history := []struct{ Role, Content string }{
		{Role: "user", Content: ollamaState.request.Prompt},
	}

	branchID, err := oi.Storage.FindBranchByHistory(ctx, "", history)
	if err != nil {
		logrus.WithError(err).Warnf("[%s] Could not find branch by history", oi.Name)
	}

	if branchID != "" {
		// If we found a branch, we add the assistant response to it.
		// Since we have the branch ID, we don't strictly need the conversation ID
		// as AddMessage will look it up if passed as empty.
		_, err = oi.Storage.AddMessage(ctx, "", branchID, "assistant", ollamaState.response.Response, ollamaState.statusCode, "")
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not add assistant response to storage", oi.Name)
		}
	} else {
		conv, branch, err := oi.Storage.CreateConversation(ctx, map[string]any{
			"model":  ollamaState.request.Model,
			"prompt": ollamaState.request.Prompt,
		})
		if err != nil {
			logrus.WithError(err).Warnf("[%s] Could not create conversation in storage", oi.Name)
		} else {
			_, _ = oi.Storage.AddMessage(ctx, conv.ID, branch.ID, "user", ollamaState.request.Prompt, 0, "")
			_, _ = oi.Storage.AddMessage(ctx, conv.ID, branch.ID, "assistant", ollamaState.response.Response, ollamaState.statusCode, "")
		}
	}
}
