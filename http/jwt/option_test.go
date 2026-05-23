package jwt

import (
	"testing"
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/stretchr/testify/assert"
)

func mockTestRoleConvert(role string) (httpcontext.IRole, error) {
	return mockTestRole(role), nil
}

type mockTestRole string

func (r mockTestRole) String() string { return string(r) }

func TestNewOption_Defaults(t *testing.T) {
	secret := []byte("test-secret")
	opt := NewOption(secret, mockTestRoleConvert)

	assert.Equal(t, secret, opt.Secret())
	assert.NotNil(t, opt.RoleConvert())
	assert.False(t, opt.SilentMode())
	assert.Equal(t, time.Duration(0), opt.RefreshDuration())
	assert.False(t, opt.AllowQueryStringToken())
	assert.Nil(t, opt.QueryStringTokenPaths())
	assert.Equal(t, time.Duration(0), opt.Leeway())
	assert.Nil(t, opt.LegacySecrets())
	assert.Nil(t, opt.SigningMethods())
}

func TestNewOption_WithRefreshDuration(t *testing.T) {
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithRefreshDuration(5*time.Minute),
	)
	assert.Equal(t, 5*time.Minute, opt.RefreshDuration())
}

func TestNewOption_WithSilentMode(t *testing.T) {
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithSilentMode(true),
	)
	assert.True(t, opt.SilentMode())
}

func TestNewOption_WithAllowQueryStringToken(t *testing.T) {
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithAllowQueryStringToken(true, "/api/v1/events", "/api/v1/stream"),
	)
	assert.True(t, opt.AllowQueryStringToken())
	assert.Equal(t, []string{"/api/v1/events", "/api/v1/stream"}, opt.QueryStringTokenPaths())
}

func TestNewOption_WithLeeway(t *testing.T) {
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithLeeway(30*time.Second),
	)
	assert.Equal(t, 30*time.Second, opt.Leeway())
}

func TestNewOption_WithLegacySecrets(t *testing.T) {
	old := []byte("old-secret")
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithLegacySecrets(old),
	)
	assert.Equal(t, [][]byte{old}, opt.LegacySecrets())
}

func TestNewOption_WithSigningMethods(t *testing.T) {
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithSigningMethods("RS256", "ES256"),
	)
	assert.Equal(t, []string{"RS256", "ES256"}, opt.SigningMethods())
}

func TestNewOption_MultipleOptions(t *testing.T) {
	opt := NewOption([]byte("s"), mockTestRoleConvert,
		WithRefreshDuration(5*time.Minute),
		WithSilentMode(true),
		WithLeeway(30*time.Second),
		WithLegacySecrets([]byte("old")),
		WithSigningMethods("RS256"),
	)
	assert.Equal(t, 5*time.Minute, opt.RefreshDuration())
	assert.True(t, opt.SilentMode())
	assert.Equal(t, 30*time.Second, opt.Leeway())
	assert.Equal(t, [][]byte{[]byte("old")}, opt.LegacySecrets())
	assert.Equal(t, []string{"RS256"}, opt.SigningMethods())
}
