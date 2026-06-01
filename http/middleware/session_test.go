package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSession_PanicOnEmptySecret(t *testing.T) {
	assert.Panics(t, func() {
		Session("session", "", SessionOption{})
	}, "should panic when secret is empty")
}

func TestSession_CreatesMiddleware(t *testing.T) {
	mw := Session("session", "my-secret-key-at-least-32bytes!", SessionOption{})
	assert.NotNil(t, mw)
}

func TestSession_WithOptionDefaults(t *testing.T) {
	r := gin.New()
	r.Use(Session("session", "my-secret-key-at-least-32bytes!", SessionOption{}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSession_WithCustomOptions(t *testing.T) {
	secure := true
	httpOnly := false
	r := gin.New()
	r.Use(Session("session", "my-secret-key-at-least-32bytes!", SessionOption{
		Path:     "/",
		Domain:   "example.com",
		Secure:   &secure,
		HttpOnly: &httpOnly,
		SameSite: http.SameSiteStrictMode,
	}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSessionWithSecretFromEnv_PanicOnEmptyEnv(t *testing.T) {
	assert.Panics(t, func() {
		SessionWithSecretFromEnv("session", "NONEXISTENT_ENV_KEY_12345", SessionOption{})
	}, "should panic when env variable is not set")
}

func TestSessionWithSecretFromEnv_WithEnvSet(t *testing.T) {
	t.Setenv("TEST_SESSION_SECRET", "my-secret-key-at-least-32bytes!")

	mw := SessionWithSecretFromEnv("session", "TEST_SESSION_SECRET", SessionOption{})
	assert.NotNil(t, mw)
}

func TestResolveSessionDefaults(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		secure, httpOnly, sameSite := resolveSessionDefaults(SessionOption{})
		assert.False(t, secure)
		assert.True(t, httpOnly)
		assert.Equal(t, http.SameSiteLaxMode, sameSite)
	})

	t.Run("custom secure", func(t *testing.T) {
		val := true
		secure, _, _ := resolveSessionDefaults(SessionOption{Secure: &val})
		assert.True(t, secure)
	})

	t.Run("custom httpOnly", func(t *testing.T) {
		val := false
		_, httpOnly, _ := resolveSessionDefaults(SessionOption{HttpOnly: &val})
		assert.False(t, httpOnly)
	})

	t.Run("custom sameSite", func(t *testing.T) {
		_, _, sameSite := resolveSessionDefaults(SessionOption{SameSite: http.SameSiteStrictMode})
		assert.Equal(t, http.SameSiteStrictMode, sameSite)
	})
}

func TestSessionWithStore_CreatesMiddleware(t *testing.T) {
	store := cookie.NewStore([]byte("my-secret-key-at-least-32bytes!"))
	mw := SessionWithStore("session", store, SessionOption{})
	assert.NotNil(t, mw)
}

func TestSessionWithStore_WithRequest(t *testing.T) {
	store := cookie.NewStore([]byte("my-secret-key-at-least-32bytes!"))

	r := gin.New()
	r.Use(SessionWithStore("session", store, SessionOption{}))
	r.GET("/test", func(c *gin.Context) {
		session := sessions.Default(c)
		assert.NotNil(t, session)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
