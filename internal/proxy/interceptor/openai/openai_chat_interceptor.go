package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	interceptor2 "llm-monitor/internal/proxy/interceptor"
	"llm-monitor/internal/storage"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ChatInterceptor intercepts chat messages between client and OpenAI compatible server
type ChatInterceptor struct {
	interceptor2.SavingInterceptor
}

// chatMessage represents an OpenAI chat message
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest represents the structure of an OpenAI chat request
type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []chatMessage  `json:"messages"`
	Stream        bool           `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitzero"`
}

// chatResponseChoice represents a choice in an OpenAI chat response
type chatResponseChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	Delta        chatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

// chatUsage represents token usage in an OpenAI chat response
type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatResponse represents the structure of an OpenAI chat response
type chatResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []chatResponseChoice `json:"choices"`
	Usage   chatUsage            `json:"usage,omitzero"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// chatState holds the state information for OpenAI requests
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

	// Extract host information
	openAIState, _ := state.(*chatState)
	openAIState.upstreamHost = req.Host
	openAIState.clientHost = req.Header.Get("X-Forwarded-For")

	// Parse the chat request
	var chatReq chatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse request body", oi.Name)
	} else {
		// Always set include_usage to true if streaming is requested
		if chatReq.Stream {
			if chatReq.StreamOptions == nil {
				chatReq.StreamOptions = &streamOptions{}
			}
			chatReq.StreamOptions.IncludeUsage = true

			// Marshal the modified request back to JSON
			newBody, err := json.Marshal(chatReq)
			if err != nil {
				logrus.WithError(err).Errorf("[%s] Error: Could not marshal modified request body", oi.Name)
			} else {
				body = newBody
				req.ContentLength = int64(len(body))
				req.Header.Set("Content-Length", fmt.Sprint(len(body)))
			}
		}
		openAIState.request = chatReq
	}

	// Store available request information
	oi.saveLog(openAIState)

	// Create a new body reader
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return nil
}

// ResponseInterceptor intercepts the response to extract response messages
func (oi *ChatInterceptor) ResponseInterceptor(resp *http.Response, state interceptor2.State) error {
	openAIState, _ := state.(*chatState)
	openAIState.statusCode = resp.StatusCode
	return nil
}

// ContentInterceptor intercepts content to extract response messages (non-streaming)
func (oi *ChatInterceptor) ContentInterceptor(content []byte, state interceptor2.State) ([]byte, error) {
	openAIState, _ := state.(*chatState)

	// Parse the response
	var chatResp chatResponse
	if err := json.Unmarshal(content, &chatResp); err != nil {
		logrus.WithError(err).Warningf("[%s] Warning: Could not parse response body", oi.Name)
	} else {
		openAIState.response = chatResp
	}

	return content, nil
}

// ChunkInterceptor intercepts chunks for streaming responses
func (oi *ChatInterceptor) ChunkInterceptor(chunk []byte, state interceptor2.State) ([]byte, error) {
	openAIState, _ := state.(*chatState)

	// OpenAI Server-Sent Events (SSE) format: data: {...}
	lines := strings.Split(string(chunk), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			var chatResp chatResponse
			if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
				logrus.WithError(err).Warningf("[%s] Warning: Could not parse response chunk", oi.Name)
				continue
			}

			if openAIState.response.ID == "" {
				openAIState.response.ID = chatResp.ID
				openAIState.response.Model = chatResp.Model
				openAIState.response.Created = chatResp.Created
				openAIState.response.Object = chatResp.Object
			}

			for _, choice := range chatResp.Choices {
				if len(openAIState.response.Choices) <= choice.Index {
					// Expand choices if necessary
					newChoices := make([]chatResponseChoice, choice.Index+1)
					copy(newChoices, openAIState.response.Choices)
					openAIState.response.Choices = newChoices
				}

				// OpenAI Delta contains incremental updates
				openAIState.response.Choices[choice.Index].Message.Content += choice.Delta.Content
				if choice.Delta.Role != "" {
					openAIState.response.Choices[choice.Index].Message.Role = choice.Delta.Role
				}
				if choice.FinishReason != "" {
					openAIState.response.Choices[choice.Index].FinishReason = choice.FinishReason
				}
			}

			// Some OpenAI compatible servers might send usage in the last chunk
			if chatResp.Usage.TotalTokens > 0 {
				openAIState.response.Usage = chatResp.Usage
			}
		}
	}

	return chunk, nil
}

// OnComplete handles completion of the request
func (oi *ChatInterceptor) OnComplete(state interceptor2.State) {
	openAIState, _ := state.(*chatState)

	openAIState.endTime = time.Now()

	logrus.Printf("[%s] Request completed for model: %s", oi.Name, openAIState.request.Model)
	oi.logRequestResponse(openAIState)

	oi.saveLog(openAIState)
}

// OnError handles errors during request processing
func (oi *ChatInterceptor) OnError(state interceptor2.State, err error) {
	openAIState, _ := state.(*chatState)
	openAIState.endTime = time.Now()
	logrus.WithError(err).Warningf("[%s] Error occurred", oi.Name)
	oi.logRequestResponse(openAIState)

	oi.saveLog(openAIState)
}

func (oi *ChatInterceptor) logRequestResponse(openAIState *chatState) {
	for _, m := range openAIState.request.Messages {
		logrus.Printf("[%s] Request [%s]: %s", oi.Name, m.Role, m.Content)
	}
	for _, choice := range openAIState.response.Choices {
		logrus.Printf("[%s] Response [%s]: %s", oi.Name, choice.Message.Role, choice.Message.Content)
	}
}

func (oi *ChatInterceptor) saveLog(openAIState *chatState) {
	if oi.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), oi.Timeout)
		defer cancel()

		history := make([]storage.SimpleMessage, len(openAIState.request.Messages))
		for i, m := range openAIState.request.Messages {
			history[i] = storage.SimpleMessage{
				Role:       m.Role,
				Content:    m.Content,
				Model:      openAIState.request.Model,
				ClientHost: openAIState.clientHost,
			}
		}

		// Use the first choice as the assistant response (standard behavior)
		var assistantMsg storage.SimpleMessage
		if len(openAIState.response.Choices) > 0 {
			choice := openAIState.response.Choices[0]
			assistantMsg = storage.SimpleMessage{
				Role:             choice.Message.Role,
				Content:          choice.Message.Content,
				Model:            openAIState.response.Model,
				PromptTokens:     openAIState.response.Usage.PromptTokens,
				CompletionTokens: openAIState.response.Usage.CompletionTokens,
				EvalDuration:     openAIState.endTime.Sub(openAIState.startTime),
				UpstreamHost:     openAIState.upstreamHost,
			}
			if assistantMsg.Role == "" {
				assistantMsg.Role = "assistant"
			}
		}

		oi.SaveToStorage(ctx, history, assistantMsg, openAIState.statusCode, "chat")
	}
}
