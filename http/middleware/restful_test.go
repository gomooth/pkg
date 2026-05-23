package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRESTFul_ValidAccept(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFul("v1"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	tests := []struct {
		name   string
		accept string
	}{
		{"empty accept", ""},
		{"wildcard", "*/*"},
		{"json", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRESTFul_UnsupportedAccept(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFul("v1"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "text/xml")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRESTFul_CustomMediaType(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFul("v1"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/vnd.server.v1.raw+json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRESTFulWithIgnores_SkipsPath(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFulWithIgnores("v1", IgnorePath{Path: "/download", Method: "GET"}))
	r.GET("/download", func(c *gin.Context) {
		c.String(http.StatusOK, "download")
	})

	// 被忽略的路径，即使 Accept 不合法也应通过
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/download", nil)
	req.Header.Set("Accept", "text/xml")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
