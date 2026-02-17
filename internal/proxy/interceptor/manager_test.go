package interceptor

import (
	"net/http"
	"testing"
)

type MockInterceptor struct {
	EmptyState
	called bool
}

func (m *MockInterceptor) CreateState() State { return &EmptyState{} }
func (m *MockInterceptor) RequestInterceptor(_ *http.Request, _ State) error {
	m.called = true
	return nil
}
func (m *MockInterceptor) ResponseInterceptor(_ *http.Response, _ State) error { return nil }
func (m *MockInterceptor) ContentInterceptor(content []byte, _ State) ([]byte, error) {
	return content, nil
}
func (m *MockInterceptor) ChunkInterceptor(chunk []byte, _ State) ([]byte, error) {
	return chunk, nil
}
func (m *MockInterceptor) OnComplete(_ State)       {}
func (m *MockInterceptor) OnError(_ State, _ error) {}

func TestManager_GetInterceptor(t *testing.T) {
	m := NewInterceptorManager()

	intcpt1 := &MockInterceptor{}
	intcpt2 := &MockInterceptor{}
	intcptWildcard := &MockInterceptor{}

	m.RegisterInterceptor("/api/test", "POST", intcpt1)
	m.RegisterInterceptor("/api/test", "GET", intcpt2)
	m.RegisterInterceptor("/api/wildcard", "*", intcptWildcard)

	tests := []struct {
		name     string
		endpoint string
		method   string
		expected Interceptor
	}{
		{"Exact match POST", "/api/test", "POST", intcpt1},
		{"Exact match GET", "/api/test", "GET", intcpt2},
		{"No match method", "/api/test", "PUT", nil},
		{"Wildcard match GET", "/api/wildcard", "GET", intcptWildcard},
		{"Wildcard match POST", "/api/wildcard", "POST", intcptWildcard},
		{"No match endpoint", "/api/other", "POST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.GetInterceptor(tt.endpoint, tt.method)
			if got != tt.expected {
				t.Errorf("GetInterceptor() = %v, want %v", got, tt.expected)
			}
		})
	}
}
