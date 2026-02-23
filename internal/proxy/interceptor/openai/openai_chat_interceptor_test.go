package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	interceptor2 "llm-monitor/internal/proxy/interceptor"
	"llm-monitor/internal/storage"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestChatInterceptor_RequestInterceptor_PreservesTools(t *testing.T) {
	interceptor := &ChatInterceptor{}
	state := interceptor.CreateState()

	requestBody := `{
		"model": "gpt-3.5-turbo",
		"messages": [{"role": "user", "content": "What's the weather like in Boston?"}],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_current_weather",
					"description": "Get the current weather in a given location",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {
								"type": "string",
								"description": "The city and state, e.g. San Francisco, CA"
							},
							"unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
						},
						"required": ["location"]
					}
				}
			}
		],
		"tool_choice": "auto",
		"stream": true
	}`

	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")

	err := interceptor.RequestInterceptor(req, state)
	assert.NoError(t, err)

	// Read the modified request body
	modifiedBody, _ := io.ReadAll(req.Body)
	
	var result map[string]interface{}
	err = json.Unmarshal(modifiedBody, &result)
	assert.NoError(t, err)

	// Check if tools and tool_choice are preserved
	assert.Contains(t, result, "tools", "Request body should contain 'tools'")
	assert.Contains(t, result, "tool_choice", "Request body should contain 'tool_choice'")
	assert.Equal(t, "auto", result["tool_choice"])
}

func TestChatInterceptor_RequestInterceptor_PreservesUnknownFields(t *testing.T) {
	interceptor := &ChatInterceptor{}
	state := interceptor.CreateState()

	requestBody := `{
		"model": "gpt-3.5-turbo",
		"messages": [{"role": "user", "content": "Hello"}],
		"unknown_field": "some_value",
		"nested_unknown": {
			"key": "value"
		},
		"stream": true
	}`

	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")

	err := interceptor.RequestInterceptor(req, state)
	assert.NoError(t, err)

	// Read the modified request body
	modifiedBody, _ := io.ReadAll(req.Body)
	
	var result map[string]interface{}
	err = json.Unmarshal(modifiedBody, &result)
	assert.NoError(t, err)

	// Check if unknown fields are preserved
	assert.Equal(t, "some_value", result["unknown_field"])
	assert.NotNil(t, result["nested_unknown"])
	assert.Equal(t, "value", result["nested_unknown"].(map[string]any)["key"])
	
	// Check if stream_options.include_usage was added/modified
	assert.NotNil(t, result["stream_options"])
	assert.Equal(t, true, result["stream_options"].(map[string]any)["include_usage"])
}

func TestChatInterceptor_ContentInterceptor_PreservesToolCalls(t *testing.T) {
	interceptor := &ChatInterceptor{}
	state := interceptor.CreateState()

	responseBody := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-3.5-turbo-0613",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [
					{
						"id": "call_abc123",
						"type": "function",
						"function": {
							"name": "get_current_weather",
							"arguments": "{\n\"location\": \"Boston, MA\"\n}"
						}
					}
				]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {
			"prompt_tokens": 81,
			"completion_tokens": 12,
			"total_tokens": 93
		}
	}`

	_, err := interceptor.ContentInterceptor([]byte(responseBody), state)
	assert.NoError(t, err)

	openAIState := state.(*chatState)
	
	// Check if tool_calls were captured in the state
	assert.NotEmpty(t, openAIState.response.Choices)
	assert.NotEmpty(t, openAIState.response.Choices[0].Message.ToolCalls)
	assert.Equal(t, "call_abc123", openAIState.response.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "get_current_weather", openAIState.response.Choices[0].Message.ToolCalls[0].Function.Name)
}

func TestChatInterceptor_ChunkInterceptor_AggregatesToolCalls(t *testing.T) {
	interceptor := &ChatInterceptor{}
	state := interceptor.CreateState()

	chunks := []string{
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"get_current_weather","arguments":""}}]}}]}`,
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\""}}]}}]}`,
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":": \"Boston\"}"}}]}}]}`,
		`data: [DONE]`,
	}

	for _, chunk := range chunks {
		_, err := interceptor.ChunkInterceptor([]byte(chunk), state)
		assert.NoError(t, err)
	}

	openAIState := state.(*chatState)
	assert.NotEmpty(t, openAIState.response.Choices)
	assert.NotEmpty(t, openAIState.response.Choices[0].Message.ToolCalls)
	assert.Equal(t, "call_abc123", openAIState.response.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "get_current_weather", openAIState.response.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"location": "Boston"}`, openAIState.response.Choices[0].Message.ToolCalls[0].Function.Arguments)
}

func TestChatInterceptor_SaveLog_PreservesToolCallsInMetadata(t *testing.T) {
	mockStorage := &mockStorage{}
	interceptor := &ChatInterceptor{
		SavingInterceptor: interceptor2.SavingInterceptor{
			Storage: mockStorage,
			Timeout: 1 * time.Second,
		},
	}
	state := &chatState{
		statusCode: 200,
		request: chatRequest{
			Model: "gpt-3.5-turbo",
			Messages: []chatMessage{
				{Role: "user", Content: "What's the weather?"},
			},
		},
		response: chatResponse{
			Model: "gpt-3.5-turbo",
			Choices: []chatResponseChoice{
				{
					Message: chatMessage{
						Role: "assistant",
						ToolCalls: []chatToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: chatToolFunction{
									Name:      "get_weather",
									Arguments: `{"location":"London"}`,
								},
							},
						},
					},
				},
			},
		},
	}

	interceptor.saveLog(state)

	assert.NotNil(t, mockStorage.lastAssistantMsg.Metadata)
	assert.Contains(t, mockStorage.lastAssistantMsg.Metadata, "tool_calls")
}

type mockStorage struct {
	storage.Storage
	lastAssistantMsg storage.SimpleMessage
}

func (m *mockStorage) FindMessageByHistory(ctx context.Context, history []storage.SimpleMessage, requestType string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (m *mockStorage) CreateConversation(ctx context.Context, metadata map[string]interface{}, requestType string) (*storage.Conversation, *storage.Branch, error) {
	return &storage.Conversation{ID: uuid.New()}, &storage.Branch{ID: uuid.New()}, nil
}

func (m *mockStorage) AddMessage(ctx context.Context, parentMessageID uuid.UUID, message *storage.Message) (*storage.Message, error) {
	if message.Role == "assistant" || message.UpstreamStatusCode != 0 {
		m.lastAssistantMsg = message.SimpleMessage
	}
	return &storage.Message{ID: uuid.New(), SimpleMessage: message.SimpleMessage}, nil
}
