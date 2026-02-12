package interceptor

import (
	"log"
	"net/http"
)

// LoggingInterceptor demonstrates a stateful interceptor that logs information
type LoggingInterceptor struct {
	Name string
}

func (li *LoggingInterceptor) CreateState() State {
	return &EmptyState{}
}

func (li *LoggingInterceptor) RequestInterceptor(req *http.Request, state State) error {
	log.Printf("[%s] Logging request: %s %s", li.Name, req.Method, req.URL.Path)
	return nil
}

func (li *LoggingInterceptor) ResponseInterceptor(resp *http.Response, state State) error {
	log.Printf("[%s] Logging response: Status %d", li.Name, resp.StatusCode)
	return nil
}

func (li *LoggingInterceptor) ContentInterceptor(content []byte, state State) ([]byte, error) {
	log.Printf("[%s] Logging content: %d bytes", li.Name, len(content))
	return content, nil
}

func (li *LoggingInterceptor) ChunkInterceptor(chunk []byte, state State) ([]byte, error) {
	log.Printf("[%s] Logging chunk: %d bytes", li.Name, len(chunk))
	return chunk, nil
}

func (li *LoggingInterceptor) OnComplete(state State) {
	log.Printf("[%s] Logging completion", li.Name)
}

func (li *LoggingInterceptor) OnError(state State, _ error) {
	log.Printf("[%s] Logging completion", li.Name)
}
