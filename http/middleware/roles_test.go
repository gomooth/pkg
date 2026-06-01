package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/stretchr/testify/assert"
)

type testRole string

func (r testRole) String() string { return string(r) }

func setupRoleRouter() *gin.Engine {
	r := gin.New()
	r.Use(HttpContext())
	return r
}

func withUserContext(c *gin.Context, roles ...httpcontext.IRole) {
	stx := httpcontext.NewContext()
	stx.SetUser(httpcontext.User{
		ID:      1,
		Account: "test",
		Roles:   roles,
	})
	stx.StorageTo(c)
}

func TestWithRole_MatchingRole(t *testing.T) {
	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("admin"))
		c.Next()
	})
	r.Use(WithRole(testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithRole_MultipleRoles(t *testing.T) {
	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("editor"))
		c.Next()
	})
	r.Use(WithRole(testRole("admin"), testRole("editor")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithRole_NoMatchingRole(t *testing.T) {
	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("viewer"))
		c.Next()
	})
	r.Use(WithRole(testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWithRole_NoHttpContext(t *testing.T) {
	r := gin.New()
	r.Use(WithRole(testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRoleFunc_MatchingRole(t *testing.T) {
	called := false
	handler := func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "handled")
	}

	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("admin"))
		c.Next()
	})
	r.Use(RoleFunc(handler, testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "next")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.True(t, called, "handler should be called when role matches")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleFunc_NoMatchingRole(t *testing.T) {
	called := false
	handler := func(c *gin.Context) {
		called = true
	}

	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("viewer"))
		c.Next()
	})
	r.Use(RoleFunc(handler, testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "next")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.False(t, called, "handler should not be called when role does not match")
	// 应进入下一个中间件
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleFunc_NoHttpContext(t *testing.T) {
	called := false
	handler := func(c *gin.Context) {
		called = true
	}

	r := gin.New()
	r.Use(RoleFunc(handler, testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "next")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.False(t, called, "handler should not be called when no httpcontext")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleFuncAbort_MatchingRole(t *testing.T) {
	called := false
	handler := func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "handled")
	}

	nextCalled := false
	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("admin"))
		c.Next()
	})
	r.Use(RoleFuncAbort(handler, testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		c.String(http.StatusOK, "next")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.True(t, called, "handler should be called when role matches")
	assert.True(t, nextCalled, "next handler should also be called")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleFuncAbort_NoMatchingRole(t *testing.T) {
	called := false
	handler := func(c *gin.Context) {
		called = true
	}

	r := setupRoleRouter()
	r.Use(func(c *gin.Context) {
		withUserContext(c, testRole("viewer"))
		c.Next()
	})
	r.Use(RoleFuncAbort(handler, testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "next")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.False(t, called, "handler should not be called when role does not match")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRoleFuncAbort_NoHttpContext(t *testing.T) {
	called := false
	handler := func(c *gin.Context) {
		called = true
	}

	r := gin.New()
	r.Use(RoleFuncAbort(handler, testRole("admin")))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "next")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.False(t, called, "handler should not be called when no httpcontext")
	assert.Equal(t, http.StatusForbidden, w.Code)
}
