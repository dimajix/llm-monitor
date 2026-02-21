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

// GenerateInterceptor records traffic between a Client and an Ollama server
type GenerateInterceptor struct {
	interceptor2.SavingInterceptor
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
	TotalDuration      int64  `json:"total_duration,omitempty"`
	LoadDuration       int64  `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int64  `json:"eval_duration,omitempty"`
}

// generateState holds the state for an Ollama generate request
type generateState struct {
	request      generateRequest
	response     generateResponse
	startTime    time.Time
	endTime      time.Time
	statusCode   int
	clientHost   string
	upstreamHost string
}

// CreateState creates a new generateState for tracking requests
func (oi *GenerateInterceptor) CreateState() interceptor2.State {
	return &generateState{
		startTime: time.Now(),
	}
}

// RequestInterceptor intercepts the request to /api/generate
func (oi *GenerateInterceptor) RequestInterceptor(req *http.Request, state interceptor2.State) error {
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
	ollamaState.upstreamHost = req.Host
	ollamaState.clientHost = req.Header.Get("X-Forwarded-For")

	// Parse the request to extract model and prompt
	var generateReq generateRequest
	if err := json.Unmarshal(body, &generateReq); err != nil {
		logrus.WithError(err).Warningf("[%s] Could not parse request body: %v", oi.Name, err)
	} else {
		ollamaState.request = generateReq
	}

	// Store available request information
	oi.saveLog(ollamaState)

	// Replace the request body with the original content
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return nil
}

// ResponseInterceptor intercepts the response from /api/generate
func (oi *GenerateInterceptor) ResponseInterceptor(resp *http.Response, state interceptor2.State) error {
	ollamaState, _ := state.(*generateState)
	ollamaState.statusCode = resp.StatusCode
	return nil
}

// ContentInterceptor intercepts content (not used for this specific interceptor)
func (oi *GenerateInterceptor) ContentInterceptor(content []byte, state interceptor2.State) ([]byte, error) {
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
func (oi *GenerateInterceptor) ChunkInterceptor(chunk []byte, state interceptor2.State) ([]byte, error) {
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
func (oi *GenerateInterceptor) OnComplete(state interceptor2.State) {
	ollamaState, _ := state.(*generateState)
	ollamaState.endTime = time.Now()

	logrus.Printf("[%s] Request completed for model: %s", oi.Name, ollamaState.response.Model)
	logrus.Printf("[%s] Prompt: %s", oi.Name, ollamaState.request.Prompt)
	logrus.Printf("[%s] Response: %s", oi.Name, ollamaState.response.Response)

	oi.saveLog(ollamaState)
}

// OnError is called when an error occurs
func (oi *GenerateInterceptor) OnError(state interceptor2.State, err error) {
	ollamaState, _ := state.(*generateState)
	logrus.WithError(err).Warningf("[%s] Error occurred: %v", oi.Name, err)
	logrus.Printf("[%s] Prompt: %s", oi.Name, ollamaState.request.Prompt)
	logrus.Printf("[%s] Response: %s", oi.Name, ollamaState.response.Response)

	oi.saveLog(ollamaState)
}

func (oi *GenerateInterceptor) saveLog(ollamaState *generateState) {
	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), oi.Timeout)
		defer cancel()

		history := []storage.SimpleMessage{
			{Role: "user", Content: ollamaState.request.Prompt, Model: ollamaState.request.Model, ClientHost: ollamaState.clientHost},
		}
		assistantMsg := storage.SimpleMessage{
			Role:               "assistant",
			Content:            ollamaState.response.Response,
			Model:              ollamaState.response.Model,
			PromptTokens:       ollamaState.response.PromptEvalCount,
			CompletionTokens:   ollamaState.response.EvalCount,
			PromptEvalDuration: time.Duration(ollamaState.response.PromptEvalDuration),
			EvalDuration:       time.Duration(ollamaState.response.EvalDuration),
			UpstreamHost:       ollamaState.upstreamHost,
		}

		oi.SaveToStorage(ctx, history, assistantMsg, ollamaState.statusCode, "generate")
	}
}
