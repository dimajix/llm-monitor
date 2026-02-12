package interceptor

import (
	"log"
	"net/http"
)

// SimpleInterceptor is a basic interceptor for demonstration
type SimpleInterceptor struct {
	Name string
}

func (si *SimpleInterceptor) CreateState() State {
	return &EmptyState{}
}

func (si *SimpleInterceptor) RequestInterceptor(req *http.Request, state State) error {
	log.Printf("[%s] Simple request interceptor", si.Name)
	req.Header.Set("X-Simple-Interceptor", si.Name)
	return nil
}

func (si *SimpleInterceptor) ResponseInterceptor(resp *http.Response, state State) error {
	log.Printf("[%s] Simple response interceptor", si.Name)
	resp.Header.Set("X-Simple-Response", si.Name)
	return nil
}

func (si *SimpleInterceptor) ContentInterceptor(content []byte, state State) ([]byte, error) {
	log.Printf("[%s] Simple content interceptor", si.Name)
	return content, nil
}

func (si *SimpleInterceptor) ChunkInterceptor(chunk []byte, state State) ([]byte, error) {
	log.Printf("[%s] Simple chunk interceptor", si.Name)
	return chunk, nil
}

func (si *SimpleInterceptor) OnComplete(state State) {
	log.Printf("[%s] Simple completion", si.Name)
}

func (li *SimpleInterceptor) OnError(state State, _ error) {
	log.Printf("[%s] Logging completion", li.Name)
}
