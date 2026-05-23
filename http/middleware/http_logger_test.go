package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHttpLogger_RecordsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := gin.New()
	r.Use(HttpLogger(HttpLoggerOption{Logger: logger}))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, buf.String(), "logger should produce output")
	assert.Contains(t, buf.String(), "/test")
}

func TestHttpLogger_OnlyError_SkipsSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := gin.New()
	r.Use(HttpLogger(HttpLoggerOption{Logger: logger, OnlyError: true}))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, buf.String(), "OnlyError=true should not log successful requests")
}

func TestHttpLogger_OnlyError_LogsFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := gin.New()
	r.Use(HttpLogger(HttpLoggerOption{Logger: logger, OnlyError: true}))
	r.POST("/test", func(c *gin.Context) {
		_ = c.Error(assert.AnError)
		c.String(http.StatusInternalServerError, "fail")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, buf.String(), "OnlyError=true should log failed requests")
}

func TestDisableHttpLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := gin.New()
	r.Use(HttpLogger(HttpLoggerOption{Logger: logger}))
	r.POST("/test", func(c *gin.Context) {
		DisableHttpLogger(c)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Empty(t, buf.String(), "DisableHttpLogger should suppress output")
}
