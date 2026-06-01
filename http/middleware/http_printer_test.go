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

func TestHttpPrinter_RecordsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := gin.New()
	r.Use(HttpPrinter(logger))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, buf.String(), "printer should produce output")
	assert.Contains(t, buf.String(), "/test")
}

func TestHttpPrinter_DisablesHttpLogger(t *testing.T) {
	var printerBuf bytes.Buffer
	var loggerBuf bytes.Buffer

	printerLog := slog.New(slog.NewTextHandler(&printerBuf, nil))
	loggerLog := slog.New(slog.NewTextHandler(&loggerBuf, nil))

	r := gin.New()
	r.Use(HttpLogger(HttpLoggerOption{Logger: loggerLog}))
	r.Use(HttpPrinter(printerLog))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// HttpPrinter 应禁用 HttpLogger 输出
	assert.Empty(t, loggerBuf.String(), "HttpPrinter should disable HttpLogger")
	// HttpPrinter 自身应有输出
	assert.NotEmpty(t, printerBuf.String())
}
