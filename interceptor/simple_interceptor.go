package interceptor

import (
	"log"
	"net/http"
)

// SimpleInterceptor is a basic interceptor for demonstration
type SimpleInterceptor struct {
	Name string
}

func (si *SimpleInterceptor) CreateState() InterceptorState {
	return &BaseInterceptorState{ID: si.Name}
}

func (si *SimpleInterceptor) RequestInterceptor(req *http.Request, state InterceptorState) error {
	log.Printf("[%s] Simple request interceptor", si.Name)
	req.Header.Set("X-Simple-Interceptor", si.Name)
	return nil
}

func (si *SimpleInterceptor) ResponseInterceptor(resp *http.Response, state InterceptorState) error {
	log.Printf("[%s] Simple response interceptor", si.Name)
	resp.Header.Set("X-Simple-Response", si.Name)
	return nil
}

func (si *SimpleInterceptor) ContentInterceptor(content []byte, state InterceptorState) ([]byte, error) {
	log.Printf("[%s] Simple content interceptor", si.Name)
	return content, nil
}

func (si *SimpleInterceptor) ChunkInterceptor(chunk []byte, state InterceptorState) ([]byte, error) {
	log.Printf("[%s] Simple chunk interceptor", si.Name)
	return chunk, nil
}

func (si *SimpleInterceptor) OnComplete(state InterceptorState) error {
	log.Printf("[%s] Simple completion", si.Name)
	return nil
}
