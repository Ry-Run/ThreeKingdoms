package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewHttpServer_Healthz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := NewHttpServer(":0", gin.New())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
	s.Handler().ServeHTTP(w, req)

	if w.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status code: got=%d want=%d", w.Code, nethttp.StatusOK)
	}
}
