package jwt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/xerror"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type testRole string

func (r testRole) String() string { return string(r) }

func testRoleConvert(role string) (httpcontext.IRole, error) {
	return testRole(role), nil
}

func testRoleConvertErr(role string) (httpcontext.IRole, error) {
	return nil, errors.New("role convert error")
}

func newTestGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
		URL:    &url.URL{Path: "/"},
	}
	return c
}

func newTestGinContextWithQuery(path, query string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
		URL:    &url.URL{Path: path, RawQuery: query},
	}
	return c
}

// mockStatefulStore is an in-memory StatefulStore for testing
type mockStatefulStore struct {
	saveErr  error
	saveFunc func(ctx context.Context, userID uint, token string, expireTs int64) error
	store    map[uint]string
}

func newMockStatefulStore() *mockStatefulStore {
	return &mockStatefulStore{store: make(map[uint]string)}
}

func (m *mockStatefulStore) Save(ctx context.Context, userID uint, token string, expireTs int64) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.saveFunc != nil {
		return m.saveFunc(ctx, userID, token, expireTs)
	}
	m.store[userID] = token
	return nil
}

func (m *mockStatefulStore) Check(ctx context.Context, userID uint, token string) error {
	return nil
}

func (m *mockStatefulStore) Remove(ctx context.Context, userID uint, token string) error {
	delete(m.store, userID)
	return nil
}

func (m *mockStatefulStore) Clean(ctx context.Context, userID uint) error {
	delete(m.store, userID)
	return nil
}

// ---------------------------------------------------------------------------
// NewToken / NewStatefulToken edge cases
// ---------------------------------------------------------------------------

func TestNewToken_EmptySecret(t *testing.T) {
	_, err := NewTokenBuilder(nil, httpcontext.User{ID: 1, Name: "test"}).Build()
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))

	_, err = NewTokenBuilder([]byte{}, httpcontext.User{ID: 1, Name: "test"}).Build()
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

func TestNewStatefulToken_EmptySecret(t *testing.T) {
	_, err := NewTokenBuilder(nil, httpcontext.User{ID: 1, Name: "test"}).WithStatefulStore(nil).Build()
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))

	_, err = NewTokenBuilder([]byte{}, httpcontext.User{ID: 1, Name: "test"}).WithStatefulStore(nil).Build()
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

// ---------------------------------------------------------------------------
// ToString edge cases
// ---------------------------------------------------------------------------

func TestToken_ToString_NoSecret(t *testing.T) {
	// Create a token with secret then clear it to test the error path
	tk, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 1, Name: "test"}).Build()
	require.Nil(t, err)

	// Clear the secret via internal access (token is returned as IToken but underlying is *token)
	tokenImpl := tk.(*token)
	tokenImpl.secret = nil

	_, err = tokenImpl.ToString(context.Background())
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

func TestToken_ToString_StatefulNoStore(t *testing.T) {
	store := newMockStatefulStore()
	tk, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 1, Name: "test"}).WithStatefulStore(store).Build()
	require.Nil(t, err)

	// Clear the stateful handler to test the error path
	tokenImpl := tk.(*token)
	tokenImpl.statefulHandler = nil

	_, err = tk.ToString(context.Background())
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

func TestToken_ToString_StatefulWithStore(t *testing.T) {
	store := newMockStatefulStore()
	tk, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 1, Name: "test"}).WithStatefulStore(store).Build()
	require.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)
	assert.NotEmpty(t, tokenStr)

	// Verify the store was called
	_, exists := store.store[1]
	assert.True(t, exists)
}

func TestToken_ToString_StatefulStoreError(t *testing.T) {
	store := newMockStatefulStore()
	store.saveErr = errors.New("redis error")

	tk, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 1, Name: "test"}).WithStatefulStore(store).Build()
	require.Nil(t, err)

	_, err = tk.ToString(context.Background())
	assert.NotNil(t, err)
	assert.Equal(t, "redis error", err.Error())
}

// ---------------------------------------------------------------------------
// SetSigningMethod
// ---------------------------------------------------------------------------

func TestToken_SetSigningMethod(t *testing.T) {
	secret := []byte("test-secret-key")

	t.Run("valid signing method", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)

		result := tk.SetSigningMethod(jwt.SigningMethodHS512)
		assert.Equal(t, tk, result) // returns IToken for chaining

		tokenStr, err := tk.ToString(context.Background())
		assert.Nil(t, err)
		assert.NotEmpty(t, tokenStr)

		// Parse back with HS512 allowed
		c, err := parseToken(tokenStr, secret, nil, 0, []string{"HS512"})
		assert.Nil(t, err)
		assert.Equal(t, uint(1), c.UserID)
	})

	t.Run("nil signing method is ignored", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)

		result := tk.SetSigningMethod(nil)
		assert.Equal(t, tk, result)

		// Should still use HS256
		tokenStr, err := tk.ToString(context.Background())
		assert.Nil(t, err)

		c, err := parseToken(tokenStr, secret, nil, 0, nil)
		assert.Nil(t, err)
		assert.Equal(t, uint(1), c.UserID)
	})
}

// ---------------------------------------------------------------------------
// GetUser / rolesBy
// ---------------------------------------------------------------------------

func TestToken_GetUser(t *testing.T) {
	secret := []byte("test-secret-key")

	t.Run("with roles", func(t *testing.T) {
		user := httpcontext.User{
			ID:      42,
			Account: "admin",
			Name:    "Admin",
			Roles: []httpcontext.IRole{
				testRole("admin"),
				testRole("editor"),
			},
		}
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)

		parsedUser, err := tk.GetUser(testRoleConvert)
		assert.Nil(t, err)
		assert.Equal(t, uint(42), parsedUser.ID)
		assert.Equal(t, "Admin", parsedUser.Name)
		assert.Len(t, parsedUser.Roles, 2)
	})

	t.Run("without roles", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)

		parsedUser, err := tk.GetUser(testRoleConvert)
		assert.Nil(t, err)
		assert.Empty(t, parsedUser.Roles)
	})

	t.Run("role convert error", func(t *testing.T) {
		user := httpcontext.User{
			ID:    1,
			Name:  "test",
			Roles: []httpcontext.IRole{testRole("admin")},
		}
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)

		_, err = tk.GetUser(testRoleConvertErr)
		assert.NotNil(t, err)
		assert.Equal(t, "role convert error", err.Error())
	})

	t.Run("with extend data", func(t *testing.T) {
		user := httpcontext.User{
			ID:   1,
			Name: "test",
			Extend: map[string]string{
				"dept": "engineering",
			},
		}
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)

		parsedUser, err := tk.GetUser(testRoleConvert)
		assert.Nil(t, err)
		assert.Equal(t, "engineering", parsedUser.Extend["dept"])
	})
}

// ---------------------------------------------------------------------------
// IsExpired edge cases
// ---------------------------------------------------------------------------

func TestToken_IsExpired_NilExpiresAt(t *testing.T) {
	tk, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 1, Name: "test"}).Build()
	require.Nil(t, err)

	// Manually set ExpiresAt to nil
	tokenImpl := tk.(*token)
	tokenImpl.claims.ExpiresAt = nil

	assert.True(t, tk.IsExpired())
}

// ---------------------------------------------------------------------------
// RefreshNear
// ---------------------------------------------------------------------------

func TestToken_RefreshNear(t *testing.T) {
	secret := []byte("test-secret-key")

	t.Run("not near expiry - no refresh", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)

		// Default is 24h, refreshNear with 1 minute should NOT trigger
		originalExpiry := tk.(*token).claims.ExpiresAt.Time
		tk.RefreshNear(1 * time.Minute)
		newExpiry := tk.(*token).claims.ExpiresAt.Time
		assert.Equal(t, originalExpiry, newExpiry)
	})

	t.Run("near expiry - should refresh", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)

		// Set to expire in 5 minutes from now by adjusting the claims directly
		tokenImpl := tk.(*token)
		tokenImpl.claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(5 * time.Minute))
		tokenImpl.duration = 2 * time.Hour // refresh duration

		originalExpiry := tokenImpl.claims.ExpiresAt.Time

		// RefreshNear with 10 minute threshold - should trigger since expiry < now+10min
		tk.RefreshNear(10 * time.Minute)
		newExpiry := tokenImpl.claims.ExpiresAt.Time
		assert.True(t, newExpiry.After(originalExpiry))
	})

	t.Run("nil ExpiresAt - no panic", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)

		tokenImpl := tk.(*token)
		tokenImpl.claims.ExpiresAt = nil

		// Should not panic
		tk.RefreshNear(10 * time.Minute)
	})
}

// ---------------------------------------------------------------------------
// SetData edge cases
// ---------------------------------------------------------------------------

func TestToken_SetData_EmptyKey(t *testing.T) {
	tk, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 1, Name: "test"}).Build()
	require.Nil(t, err)

	// Empty key should be a no-op
	result := tk.SetData("", "value")
	assert.Equal(t, tk, result)

	// SetData on token with no Extend map should create one
	tk2, err := NewTokenBuilder([]byte("secret"), httpcontext.User{ID: 2, Name: "test2"}).Build()
	require.Nil(t, err)
	tokenImpl := tk2.(*token)
	tokenImpl.claims.Extend = nil

	tk2.SetData("key", "val")
	assert.Equal(t, "val", tokenImpl.claims.Extend["key"])
}

// ---------------------------------------------------------------------------
// rolesFromUser
// ---------------------------------------------------------------------------

func TestRolesFromUser_NilRoles(t *testing.T) {
	user := httpcontext.User{ID: 1, Name: "test", Roles: nil}
	roles := rolesFromUser(user)
	assert.Empty(t, roles)
}

func TestRolesFromUser_WithRoles(t *testing.T) {
	user := httpcontext.User{
		ID:   1,
		Name: "test",
		Roles: []httpcontext.IRole{
			testRole("admin"),
			testRole("user"),
		},
	}
	roles := rolesFromUser(user)
	assert.Equal(t, []string{"admin", "user"}, roles)
}

// ---------------------------------------------------------------------------
// ParseTokenWithSecret
// ---------------------------------------------------------------------------

func TestParseTokenWithSecret(t *testing.T) {
	secret := []byte("test-secret-key")
	user := httpcontext.User{ID: 42, Account: "user1", Name: "User One"}

	t.Run("valid token from header", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)

		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		rawStr, parsedTk, err := ParseTokenWithSecret(c, secret)
		assert.Nil(t, err)
		assert.Equal(t, tokenStr, rawStr)
		assert.NotNil(t, parsedTk)
		assert.False(t, parsedTk.IsExpired())
	})

	t.Run("empty token string returns error", func(t *testing.T) {
		c := newTestGinContext()
		_, _, err := ParseTokenWithSecret(c, secret)
		assert.NotNil(t, err)
	})

	t.Run("invalid token string returns error", func(t *testing.T) {
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, "invalid.token.string")
		_, _, err := ParseTokenWithSecret(c, secret)
		assert.NotNil(t, err)
	})
}

// ---------------------------------------------------------------------------
// getTokenStr
// ---------------------------------------------------------------------------

func TestGetTokenStr(t *testing.T) {
	t.Run("from header", func(t *testing.T) {
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, "  my-token  ")

		tokenStr := getTokenStr(c, false)
		assert.Equal(t, "my-token", tokenStr)
	})

	t.Run("from query string when allowed", func(t *testing.T) {
		c := newTestGinContextWithQuery("/", "token=query-token")

		tokenStr := getTokenStr(c, true)
		assert.Equal(t, "query-token", tokenStr)
	})

	t.Run("query string not allowed - ignored", func(t *testing.T) {
		c := newTestGinContextWithQuery("/", "token=query-token")

		tokenStr := getTokenStr(c, false)
		assert.Empty(t, tokenStr)
	})

	t.Run("header takes priority over query", func(t *testing.T) {
		c := newTestGinContextWithQuery("/", "token=query-token")
		c.Request.Header.Set(TokenHeaderKey, "header-token")

		tokenStr := getTokenStr(c, true)
		assert.Equal(t, "header-token", tokenStr)
	})

	t.Run("empty both sources", func(t *testing.T) {
		c := newTestGinContext()

		tokenStr := getTokenStr(c, true)
		assert.Empty(t, tokenStr)
	})
}

// ---------------------------------------------------------------------------
// getTokenStrWithPaths
// ---------------------------------------------------------------------------

func TestGetTokenStrWithPaths(t *testing.T) {
	t.Run("from header only - no query allowed", func(t *testing.T) {
		opt := NewOption([]byte("s"), testRoleConvert)
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, "header-token")

		tokenStr := getTokenStrWithPaths(c, opt)
		assert.Equal(t, "header-token", tokenStr)
	})

	t.Run("query string with path whitelist - matching path", func(t *testing.T) {
		opt := NewOption([]byte("s"), testRoleConvert,
			WithAllowQueryStringToken(true, "/api/v1/events"),
		)
		c := newTestGinContextWithQuery("/api/v1/events", "token=query-token")

		tokenStr := getTokenStrWithPaths(c, opt)
		assert.Equal(t, "query-token", tokenStr)
	})

	t.Run("query string with path whitelist - non-matching path", func(t *testing.T) {
		opt := NewOption([]byte("s"), testRoleConvert,
			WithAllowQueryStringToken(true, "/api/v1/events"),
		)
		c := newTestGinContextWithQuery("/api/v1/other", "token=query-token")

		tokenStr := getTokenStrWithPaths(c, opt)
		assert.Empty(t, tokenStr)
	})

	t.Run("query string globally allowed - no paths", func(t *testing.T) {
		opt := NewOption([]byte("s"), testRoleConvert,
			WithAllowQueryStringToken(true),
		)
		c := newTestGinContextWithQuery("/any/path", "token=query-token")

		tokenStr := getTokenStrWithPaths(c, opt)
		assert.Equal(t, "query-token", tokenStr)
	})

	t.Run("nil option - header only", func(t *testing.T) {
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, "header-token")

		tokenStr := getTokenStrWithPaths(c, nil)
		assert.Equal(t, "header-token", tokenStr)
	})

	t.Run("header takes priority over query string", func(t *testing.T) {
		opt := NewOption([]byte("s"), testRoleConvert,
			WithAllowQueryStringToken(true, "/api/v1/events"),
		)
		c := newTestGinContextWithQuery("/api/v1/events", "token=query-token")
		c.Request.Header.Set(TokenHeaderKey, "header-token")

		tokenStr := getTokenStrWithPaths(c, opt)
		assert.Equal(t, "header-token", tokenStr)
	})
}

// ---------------------------------------------------------------------------
// ParseTokenWithGinAndOption
// ---------------------------------------------------------------------------

func TestParseTokenWithGinAndOption(t *testing.T) {
	secret := []byte("option-level-secret-32bytes-ok!")

	t.Run("valid token from header", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert)

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 7, Account: "opt-user", Name: "Opt User"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		rawStr, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
		assert.Nil(t, err)
		assert.Equal(t, tokenStr, rawStr)
		assert.NotNil(t, parsedTk)
	})

	t.Run("nil option returns error", func(t *testing.T) {
		c := newTestGinContext()
		_, _, err := ParseTokenWithGinAndOption(c, nil)
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
	})

	t.Run("empty secret in option returns error", func(t *testing.T) {
		c := newTestGinContext()
		opt := NewOption(nil, testRoleConvert)
		_, _, err := ParseTokenWithGinAndOption(c, opt)
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert)
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, "invalid.token.string")

		_, _, err := ParseTokenWithGinAndOption(c, opt)
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	})

	t.Run("with leeway", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert, WithLeeway(30*time.Second))

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
		assert.Nil(t, err)
		assert.NotNil(t, parsedTk)
	})

	t.Run("with custom signing methods", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert, WithSigningMethods("HS256"))

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
		assert.Nil(t, err)
		assert.NotNil(t, parsedTk)
	})

	t.Run("with legacy secrets", func(t *testing.T) {
		oldSecret := []byte("old-secret-key-32-bytes-long!!!")
		newSecret := []byte("new-secret-key-32-bytes-long!!!")

		// Sign with old secret
		tk, err := NewTokenBuilder(oldSecret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		opt := NewOption(newSecret, testRoleConvert, WithLegacySecrets(oldSecret))
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
		assert.Nil(t, err)
		assert.NotNil(t, parsedTk)
	})
}

// ---------------------------------------------------------------------------
// ParseJWTUser
// ---------------------------------------------------------------------------

func TestParseJWTUser(t *testing.T) {
	secret := []byte("parse-jwt-user-secret-key-ok!")

	t.Run("valid token with roles", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert)

		user := httpcontext.User{
			ID:      42,
			Account: "admin",
			Name:    "Admin",
			Roles:   []httpcontext.IRole{testRole("admin")},
		}
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		parsedUser, err := ParseJWTUser(c, opt)
		assert.Nil(t, err)
		assert.Equal(t, uint(42), parsedUser.ID)
		assert.Equal(t, "Admin", parsedUser.Name)
	})

	t.Run("nil option returns error", func(t *testing.T) {
		c := newTestGinContext()
		_, err := ParseJWTUser(c, nil)
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	})

	t.Run("option with nil roleConvert returns error", func(t *testing.T) {
		c := newTestGinContext()
		opt := NewOption(secret, nil)
		_, err := ParseJWTUser(c, opt)
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	})

	t.Run("expired token returns error", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert)

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tk.SetDuration(-1 * time.Second)

		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		_, err = ParseJWTUser(c, opt)
		// The token is expired so parsing fails; it's wrapped as ErrJWTTokenInvalid
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert)
		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, "invalid.token.string")

		_, err := ParseJWTUser(c, opt)
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	})

	t.Run("silent mode with role convert error returns nil error", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvertErr, WithSilentMode(true))

		user := httpcontext.User{
			ID:    1,
			Name:  "test",
			Roles: []httpcontext.IRole{testRole("admin")},
		}
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		parsedUser, err := ParseJWTUser(c, opt)
		assert.Nil(t, err)
		// In silent mode, user might be nil when role conversion fails
		// because GetUser returns nil user on error
		assert.Nil(t, parsedUser)
	})

	t.Run("non-silent mode with role convert error returns error", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvertErr) // silent mode defaults to false

		user := httpcontext.User{
			ID:    1,
			Name:  "test",
			Roles: []httpcontext.IRole{testRole("admin")},
		}
		tk, err := NewTokenBuilder(secret, user).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContext()
		c.Request.Header.Set(TokenHeaderKey, tokenStr)

		_, err = ParseJWTUser(c, opt)
		assert.NotNil(t, err)
		assert.Equal(t, "role convert error", err.Error())
	})
}

// ---------------------------------------------------------------------------
// parseToken / parseTokenWithSecret deeper coverage
// ---------------------------------------------------------------------------

func TestParseToken_EmptySecret(t *testing.T) {
	_, err := parseToken("some.token.string", nil, nil, 0, nil)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

func TestParseToken_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key")
	_, err := parseToken("not-a-valid-token", secret, nil, 0, nil)
	assert.NotNil(t, err)
}

func TestParseTokenWithSecret_Coverage(t *testing.T) {
	secret := []byte("test-secret-key")

	t.Run("tokenClaims nil returns error", func(t *testing.T) {
		// An empty string should produce nil tokenClaims
		_, err := parseTokenWithSecret("", secret, 0, nil)
		assert.NotNil(t, err)
	})

	t.Run("with leeway", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c, err := parseTokenWithSecret(tokenStr, secret, 30*time.Second, nil)
		assert.Nil(t, err)
		assert.Equal(t, uint(1), c.UserID)
	})

	t.Run("with custom signing methods", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c, err := parseTokenWithSecret(tokenStr, secret, 0, []string{"HS256"})
		assert.Nil(t, err)
		assert.Equal(t, uint(1), c.UserID)
	})
}

// ---------------------------------------------------------------------------
// DefaultHashFunc / IdentityHash
// ---------------------------------------------------------------------------

func TestDefaultHashFunc(t *testing.T) {
	result := DefaultHashFunc("test-token")
	assert.NotEmpty(t, result)
	// SHA256 produces 64 hex chars
	assert.Len(t, result, 64)

	// Same input produces same output
	result2 := DefaultHashFunc("test-token")
	assert.Equal(t, result, result2)

	// Different input produces different output
	result3 := DefaultHashFunc("other-token")
	assert.NotEqual(t, result, result3)
}

func TestIdentityHash(t *testing.T) {
	result := IdentityHash("test-token")
	assert.Equal(t, "test-token", result)
}

// ---------------------------------------------------------------------------
// GenerateSecret
// ---------------------------------------------------------------------------

func TestGenerateSecretCoverage(t *testing.T) {
	t.Run("normal size", func(t *testing.T) {
		secret := GenerateSecret(32)
		assert.NotEmpty(t, secret)
		assert.True(t, len(secret) >= 32)
	})

	t.Run("below minimum size uses 32", func(t *testing.T) {
		secret := GenerateSecret(5)
		assert.NotEmpty(t, secret)
		// base64-encoded 32 bytes = 44 chars
		assert.True(t, len(secret) >= 32)
	})

	t.Run("generates different values", func(t *testing.T) {
		secret1 := GenerateSecret(32)
		secret2 := GenerateSecret(32)
		assert.NotEqual(t, secret1, secret2)
	})
}

// ---------------------------------------------------------------------------
// Integration: full round-trip with all features
// ---------------------------------------------------------------------------

func TestToken_FullRoundTrip(t *testing.T) {
	secret := []byte("round-trip-secret-key-32b-ok!")

	user := httpcontext.User{
		ID:      100,
		Account: "admin",
		Name:    "Admin User",
		Roles:   []httpcontext.IRole{testRole("admin"), testRole("editor")},
		Extend: map[string]string{
			"dept": "engineering",
		},
	}

	tk, err := NewTokenBuilder(secret, user).Build()
	require.Nil(t, err)

	// Customize
	tk.SetIssuer("my-app")
	tk.SetDuration(2 * time.Hour)
	tk.SetData("extra", "data")

	// Serialize
	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)
	assert.NotEmpty(t, tokenStr)

	// Parse back
	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	require.Nil(t, err)

	assert.Equal(t, "my-app", c.Issuer)
	assert.Equal(t, uint(100), c.UserID)
	assert.Equal(t, "admin", c.Account)
	assert.Equal(t, "Admin User", c.Name)
	assert.Equal(t, []string{"admin", "editor"}, c.Roles)
	assert.Equal(t, "engineering", c.Extend["dept"])
	assert.Equal(t, "data", c.Extend["extra"])
	assert.False(t, c.Stateful)
}

// ---------------------------------------------------------------------------
// Table-driven tests for parseToken with legacy secrets
// ---------------------------------------------------------------------------

func TestParseToken_LegacySecrets_Table(t *testing.T) {
	oldSecret := []byte("old-secret-key-32-bytes-long!!!")
	newSecret := []byte("new-secret-key-32-bytes-long!!!")
	otherSecret := []byte("other-secret-key-32-bytes-long!")

	tk, err := NewTokenBuilder(oldSecret, httpcontext.User{ID: 1, Name: "test"}).Build()
	require.Nil(t, err)
	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)

	tests := []struct {
		name          string
		secret        []byte
		legacySecrets [][]byte
		wantErr       bool
	}{
		{
			name:          "matching primary secret",
			secret:        oldSecret,
			legacySecrets: nil,
			wantErr:       false,
		},
		{
			name:          "primary fails, legacy succeeds",
			secret:        newSecret,
			legacySecrets: [][]byte{oldSecret},
			wantErr:       false,
		},
		{
			name:          "primary fails, multiple legacy - second matches",
			secret:        newSecret,
			legacySecrets: [][]byte{otherSecret, oldSecret},
			wantErr:       false,
		},
		{
			name:          "all secrets fail",
			secret:        newSecret,
			legacySecrets: [][]byte{otherSecret},
			wantErr:       true,
		},
		{
			name:          "no legacy secrets, primary fails",
			secret:        newSecret,
			legacySecrets: nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := parseToken(tokenStr, tt.secret, tt.legacySecrets, 0, nil)
			if tt.wantErr {
				assert.NotNil(t, err)
				assert.Nil(t, c)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, c)
				assert.Equal(t, uint(1), c.UserID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Option coverage: WithAllowQueryStringToken with no paths
// ---------------------------------------------------------------------------

func TestWithAllowQueryStringToken_NoPaths(t *testing.T) {
	opt := NewOption([]byte("s"), testRoleConvert,
		WithAllowQueryStringToken(true),
	)
	assert.True(t, opt.AllowQueryStringToken())
	assert.Nil(t, opt.QueryStringTokenPaths())
}

// ---------------------------------------------------------------------------
// ParseTokenWithGinAndOption with query string token
// ---------------------------------------------------------------------------

func TestParseTokenWithGinAndOption_QueryStringToken(t *testing.T) {
	secret := []byte("query-string-test-secret-key-ok!")

	t.Run("query string with matching path", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert,
			WithAllowQueryStringToken(true, "/api/v1/events"),
		)

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContextWithQuery("/api/v1/events", "token="+tokenStr)
		_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
		assert.Nil(t, err)
		assert.NotNil(t, parsedTk)
	})

	t.Run("query string with non-matching path returns error", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert,
			WithAllowQueryStringToken(true, "/api/v1/events"),
		)

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContextWithQuery("/api/v1/other", "token="+tokenStr)
		_, _, err = ParseTokenWithGinAndOption(c, opt)
		assert.NotNil(t, err)
	})

	t.Run("query string globally allowed", func(t *testing.T) {
		opt := NewOption(secret, testRoleConvert,
			WithAllowQueryStringToken(true),
		)

		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
		require.Nil(t, err)
		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c := newTestGinContextWithQuery("/any/path", "token="+tokenStr)
		_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
		assert.Nil(t, err)
		assert.NotNil(t, parsedTk)
	})
}

// ---------------------------------------------------------------------------
// Error code verification using fmt.Stringer
// ---------------------------------------------------------------------------

func TestXCodeMessages(t *testing.T) {
	// Verify error code constants have expected messages
	tests := []struct {
		name   string
		xcode  interface{ String() string }
		expect string
	}{
		{"ErrJWTSecretNotSet", xcode.ErrJWTSecretNotSet, "JWT 密钥未设置"},
		{"ErrJWTTokenInvalid", xcode.ErrJWTTokenInvalid, "JWT 令牌无效"},
		{"ErrJWTTokenExpired", xcode.ErrJWTTokenExpired, "JWT 令牌已过期"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.xcode.String(), tt.expect)
		})
	}
}

// ---------------------------------------------------------------------------
// Ensure internal token type assertions
// ---------------------------------------------------------------------------

func TestNewTokenDefaults(t *testing.T) {
	secret := []byte("test-secret-key")
	tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
	require.Nil(t, err)

	tokenImpl := tk.(*token)

	// Check defaults
	assert.Equal(t, "gomooth/pkg", tokenImpl.issuer)
	assert.Equal(t, jwt.SigningMethodHS256, tokenImpl.signingMethod)
	assert.Equal(t, 24*time.Hour, tokenImpl.duration)
	assert.False(t, tk.IsStateful())
	assert.False(t, tk.IsExpired())
}

func TestNewStatefulTokenDefaults(t *testing.T) {
	secret := []byte("test-secret-key")
	store := newMockStatefulStore()

	tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).WithStatefulStore(store).Build()
	require.Nil(t, err)

	tokenImpl := tk.(*token)
	assert.True(t, tk.IsStateful())
	assert.Equal(t, store, tokenImpl.statefulHandler)
}

// ---------------------------------------------------------------------------
// Claims IP is set by parser, not during token creation
// ---------------------------------------------------------------------------

func TestToken_IPSetByParser(t *testing.T) {
	secret := []byte("test-secret-key")
	tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test", IP: "192.168.1.1"}).Build()
	require.Nil(t, err)

	// IP from user is NOT stored in claims during creation
	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)

	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	require.Nil(t, err)
	assert.Empty(t, c.IP) // IP is not in the signed token
}

// ---------------------------------------------------------------------------
// Format the IP injection test via ParseTokenWithSecret
// ---------------------------------------------------------------------------

func TestParseTokenWithSecret_SetsClientIP(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).Build()
	require.Nil(t, err)
	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)

	c := newTestGinContext()
	c.Request.Header.Set(TokenHeaderKey, tokenStr)
	c.Request.RemoteAddr = "10.0.0.1:1234"

	_, parsedTk, err := ParseTokenWithSecret(c, secret)
	require.Nil(t, err)

	tokenImpl := parsedTk.(*token)
	assert.Equal(t, "10.0.0.1", tokenImpl.claims.IP)
}

// ---------------------------------------------------------------------------
// Ensure fmt.Stringer is used correctly with the IToken interface
// ---------------------------------------------------------------------------

func TestToken_Account(t *testing.T) {
	secret := []byte("test-secret-key")
	user := httpcontext.User{ID: 1, Account: "admin", Name: "Admin"}
	tk, err := NewTokenBuilder(secret, user).Build()
	require.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)

	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	require.Nil(t, err)
	assert.Equal(t, "admin", c.Account)
}

// ---------------------------------------------------------------------------
// TokenBuilder
// ---------------------------------------------------------------------------

func TestNewTokenBuilder(t *testing.T) {
	secret := []byte("builder-test-secret-key-ok!")

	t.Run("with options", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).
			WithIssuer("test-issuer").
			WithExpiration(2 * time.Hour).
			Build()
		require.Nil(t, err)
		assert.NotNil(t, tk)

		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)
		assert.NotEmpty(t, tokenStr)

		// Parse back and verify
		c, err := parseToken(tokenStr, secret, nil, 0, nil)
		require.Nil(t, err)
		assert.Equal(t, "test-issuer", c.Issuer)
		assert.Equal(t, uint(1), c.UserID)
	})

	t.Run("defaults only", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 2, Name: "default"}).Build()
		require.Nil(t, err)
		assert.NotNil(t, tk)
		assert.False(t, tk.IsExpired())
		assert.False(t, tk.IsStateful())
	})

	t.Run("empty secret returns error", func(t *testing.T) {
		_, err := NewTokenBuilder(nil, httpcontext.User{ID: 1, Name: "test"}).Build()
		assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
	})

	t.Run("with extend data", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).
			WithExtendData("dept", "engineering").
			Build()
		require.Nil(t, err)

		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c, err := parseToken(tokenStr, secret, nil, 0, nil)
		require.Nil(t, err)
		assert.Equal(t, "engineering", c.Extend["dept"])
	})

	t.Run("with stateful store", func(t *testing.T) {
		store := newMockStatefulStore()
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).
			WithStatefulStore(store).
			Build()
		require.Nil(t, err)
		assert.True(t, tk.IsStateful())

		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)
		assert.NotEmpty(t, tokenStr)
	})

	t.Run("with signing method", func(t *testing.T) {
		tk, err := NewTokenBuilder(secret, httpcontext.User{ID: 1, Name: "test"}).
			WithSigningMethod(jwt.SigningMethodHS512).
			Build()
		require.Nil(t, err)

		tokenStr, err := tk.ToString(context.Background())
		require.Nil(t, err)

		c, err := parseToken(tokenStr, secret, nil, 0, []string{"HS512"})
		require.Nil(t, err)
		assert.Equal(t, uint(1), c.UserID)
	})
}
