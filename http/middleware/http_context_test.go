package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/stretchr/testify/assert"
)

func TestHttpContext_InjectsContext(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.POST("/test", func(c *gin.Context) {
		stx, err := httpcontext.MustParse(c)
		assert.NoError(t, err)
		assert.NotNil(t, stx)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpContext_BodyReplay(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.POST("/test", func(c *gin.Context) {
		// body 应可再次读取
		body, err := io.ReadAll(c.Request.Body)
		assert.NoError(t, err)
		assert.Equal(t, `{"key":"value"}`, string(body))
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpContext_RawBodyInContext(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.POST("/test", func(c *gin.Context) {
		stx, err := httpcontext.MustParse(c)
		assert.NoError(t, err)
		// 原始请求体应存储在 context 中
		rawBody, ok := stx.Value(httpcontext.RequestRawBodyDataKey).([]byte)
		if ok {
			assert.Equal(t, `{"key":"value"}`, string(rawBody))
		}
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpContext_EmptyBody(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.GET("/test", func(c *gin.Context) {
		stx, err := httpcontext.MustParse(c)
		assert.NoError(t, err)
		assert.NotNil(t, stx)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpContext_StatusCodes(t *testing.T) {
	tests := []struct {
		name         string
		handler      gin.HandlerFunc
		expectedCode int
	}{
		{
			name: "200 OK",
			handler: func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "404 Not Found",
			handler: func(c *gin.Context) {
				c.String(http.StatusNotFound, "not found")
			},
			expectedCode: http.StatusNotFound,
		},
		{
			name: "500 Internal Server Error",
			handler: func(c *gin.Context) {
				c.String(http.StatusInternalServerError, "error")
			},
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(HttpContext())
			r.GET("/test", tt.handler)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}
