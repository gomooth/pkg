package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRESTFulWithIgnores_NonIgnoredPathStillChecked(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFulWithIgnores("v1", IgnorePath{Path: "/download", Method: "GET"}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// /test 不在忽略列表中，非法 Accept 应被拒绝
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "text/xml")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRESTFulWithIgnores_MethodMismatch(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFulWithIgnores("v1", IgnorePath{Path: "/download", Method: "POST"}))
	r.GET("/download", func(c *gin.Context) {
		c.String(http.StatusOK, "download")
	})

	// 忽略规则是 POST /download，但请求是 GET /download，应检查 Accept
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/download", nil)
	req.Header.Set("Accept", "text/xml")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRESTFulWithIgnores_IgnoredPathWithValidAccept(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFulWithIgnores("v1", IgnorePath{Path: "/download", Method: "GET"}))
	r.GET("/download", func(c *gin.Context) {
		c.String(http.StatusOK, "download")
	})

	// 被忽略的路径，合法 Accept 也应通过
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/download", nil)
	req.Header.Set("Accept", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRESTFulWithIgnores_MultipleIgnorePaths(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(RESTFulWithIgnores("v1",
		IgnorePath{Path: "/download", Method: "GET"},
		IgnorePath{Path: "/callback", Method: "POST"},
	))
	r.GET("/download", func(c *gin.Context) {
		c.String(http.StatusOK, "download")
	})
	r.POST("/callback", func(c *gin.Context) {
		c.String(http.StatusOK, "callback")
	})

	// 两个忽略路径应都生效
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/download", nil)
	req1.Header.Set("Accept", "text/xml")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/callback", nil)
	req2.Header.Set("Accept", "text/xml")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}
