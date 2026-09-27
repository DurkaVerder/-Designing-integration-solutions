package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type idempotentResponse struct {
	status int
	body   []byte
}
type IdempotencyStore struct {
	mu     sync.Mutex
	values map[string]idempotentResponse
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{values: make(map[string]idempotentResponse)}
}
func (s *IdempotencyStore) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}
		storeKey := c.Request.Method + ":" + c.Request.URL.Path + ":" + key
		s.mu.Lock()
		response, ok := s.values[storeKey]
		s.mu.Unlock()
		if ok {
			c.Data(response.status, "application/json", response.body)
			c.Abort()
			return
		}
		writer := &captureWriter{ResponseWriter: c.Writer, body: make([]byte, 0)}
		c.Writer = writer
		c.Next()
		if writer.status >= 200 && writer.status < 300 {
			s.mu.Lock()
			s.values[storeKey] = idempotentResponse{status: writer.status, body: writer.body}
			s.mu.Unlock()
		}
	}
}

type captureWriter struct {
	gin.ResponseWriter
	body   []byte
	status int
}

func (w *captureWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *captureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}
