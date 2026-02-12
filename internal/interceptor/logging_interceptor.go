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

func (li *LoggingInterceptor) RequestInterceptor(req *http.Request, _ State) error {
	log.Printf("[%s] Logging request: %s %s", li.Name, req.Method, req.URL.Path)
	return nil
}

func (li *LoggingInterceptor) ResponseInterceptor(resp *http.Response, _ State) error {
	log.Printf("[%s] Logging response: Status %d", li.Name, resp.StatusCode)
	return nil
}

func (li *LoggingInterceptor) ContentInterceptor(content []byte, _ State) ([]byte, error) {
	log.Printf("[%s] Logging content: %d bytes", li.Name, len(content))
	return content, nil
}

func (li *LoggingInterceptor) ChunkInterceptor(chunk []byte, _ State) ([]byte, error) {
	log.Printf("[%s] Logging chunk: %d bytes", li.Name, len(chunk))
	return chunk, nil
}

func (li *LoggingInterceptor) OnComplete(_ State) {
	log.Printf("[%s] Logging completion", li.Name)
}

func (li *LoggingInterceptor) OnError(_ State, _ error) {
	log.Printf("[%s] Logging completion", li.Name)
}
