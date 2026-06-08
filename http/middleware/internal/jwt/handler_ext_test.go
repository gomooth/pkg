package jwt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/httpcontext"
	pkgjwt "github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/xerror"
	"github.com/stretchr/testify/assert"
)

// --- 无状态 handler 补充测试 ---

// TestHandler_Handle_OptionWithNilRoleConvert option 中 roleConvert 为 nil 时返回错误
func TestHandler_Handle_OptionWithNilRoleConvert(t *testing.T) {
	c := newMiddlewareTestGinContext()
	opt := pkgjwt.NewOption([]byte("test-secret-that-is-long-enough-32b!"), nil)
	h := NewHandler(c, opt)

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestHandler_Handle_ExpiredToken 严重过期的 token 在解析阶段被拒绝，返回 ErrJWTTokenInvalid
func TestHandler_Handle_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	// 构建一个有效 token，然后通过 SetDuration 使其过期
	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	// SetDuration 接受负值，使 ExpiresAt 为 issuedAt - 1h（即过去时间）
	tk.SetDuration(-1 * time.Hour)

	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert, pkgjwt.WithLeeway(0))
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.NotNil(t, err)
	// 严重过期的 token 在 ParseTokenWithGinAndOption 阶段就会被拒绝，返回 ErrJWTTokenInvalid
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestHandler_Handle_StatefulTokenOnStatelessHandler 有状态 token 用于无状态 handler 返回错误
func TestHandler_Handle_StatefulTokenOnStatelessHandler(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	// 构建有状态 token
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(&mockStatefulStore{}).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	assert.Contains(t, err.Error(), "stateless token required")
}

// TestHandler_Handle_ValidTokenWithRefresh 有效 token + 自动续期
func TestHandler_Handle_ValidTokenWithRefresh(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithExpiration(1 * time.Hour).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert,
		pkgjwt.WithRefreshDuration(2*time.Hour), // 刷新临界时长大于 token 有效期，确保触发续期
	)
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.Nil(t, err)

	// 验证续期 token 已写入响应头
	newToken := c.Writer.Header().Get(pkgjwt.TokenHeaderKey)
	assert.NotEmpty(t, newToken, "should set refreshed token in response header")
}

// TestHandler_Handle_ValidTokenWithoutRefresh 有效 token + 无续期配置
func TestHandler_Handle_ValidTokenWithoutRefresh(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert,
		pkgjwt.WithRefreshDuration(0), // 不自动续期
	)
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.Nil(t, err)

	// 不应设置续期 token
	newToken := c.Writer.Header().Get(pkgjwt.TokenHeaderKey)
	assert.Empty(t, newToken)
}

// TestHandler_Handle_ValidToken_SetsUserInContext 有效 token 写入用户信息
func TestHandler_Handle_ValidToken_SetsUserInContext(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      42,
		Account: "test-account",
		Name:    "Test User",
		Roles:   []httpcontext.IRole{mockRole("admin")},
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.Nil(t, err)

	// 验证用户信息已写入 httpcontext
	stx, exists := c.Get(httpcontext.ContextKey)
	assert.True(t, exists)
	httpCtx, ok := stx.(httpcontext.IHttpContext)
	assert.True(t, ok)
	assert.NotNil(t, httpCtx.User())
	assert.Equal(t, uint(42), httpCtx.User().ID)
}

// --- ensureUserInContext 测试 ---

// TestEnsureUserInContext_NewContext 无 httpcontext 时自动创建
func TestEnsureUserInContext_NewContext(t *testing.T) {
	c := newMiddlewareTestGinContext()
	user := &httpcontext.User{
		ID:      1,
		Account: "test",
		Name:    "Test",
	}

	ensureUserInContext(c, user)

	stx, exists := c.Get(httpcontext.ContextKey)
	assert.True(t, exists)
	httpCtx, ok := stx.(httpcontext.IHttpContext)
	assert.True(t, ok)
	assert.NotNil(t, httpCtx.User())
	assert.Equal(t, uint(1), httpCtx.User().ID)
}

// TestEnsureUserInContext_ExistingContext_NoUser httpcontext 存在但无用户，设置用户
func TestEnsureUserInContext_ExistingContext_NoUser(t *testing.T) {
	c := newMiddlewareTestGinContext()
	existingCtx := httpcontext.NewContext()
	existingCtx.StorageTo(c)

	user := &httpcontext.User{
		ID:      2,
		Account: "test2",
		Name:    "Test2",
	}

	ensureUserInContext(c, user)

	stx, _ := c.Get(httpcontext.ContextKey)
	httpCtx := stx.(httpcontext.IHttpContext)
	assert.NotNil(t, httpCtx.User())
	assert.Equal(t, uint(2), httpCtx.User().ID)
}

// TestEnsureUserInContext_ExistingContext_WithUser 已有用户信息，不覆盖（幂等）
func TestEnsureUserInContext_ExistingContext_WithUser(t *testing.T) {
	c := newMiddlewareTestGinContext()
	existingCtx := httpcontext.NewContext()
	existingCtx.SetUser(httpcontext.User{
		ID:      1,
		Account: "original",
		Name:    "Original",
	})
	existingCtx.StorageTo(c)

	newUser := &httpcontext.User{
		ID:      2,
		Account: "new",
		Name:    "New",
	}

	ensureUserInContext(c, newUser)

	stx, _ := c.Get(httpcontext.ContextKey)
	httpCtx := stx.(httpcontext.IHttpContext)
	// 应保留原始用户，不被覆盖
	assert.Equal(t, uint(1), httpCtx.User().ID)
	assert.Equal(t, "original", httpCtx.User().Account)
}

// --- 有状态 handler 补充测试 ---

// TestStatefulHandler_Handle_NilStoreAndSkipCheck store 为 nil 时 skipCheck=true，跳过 store 检查
func TestStatefulHandler_Handle_NilStoreAndSkipCheck(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	// 构建有状态 token
	store := &mockStatefulStore{}
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(store).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)

	// 传入 nil store，skipCheckStateful=true
	h := NewStatefulHandler(c, opt, nil)

	err = h.Handle()
	assert.Nil(t, err, "skipCheckStateful=true should bypass store check")
}

// TestStatefulHandler_Handle_ValidStatefulToken 有效有状态 token 通过 store 检查
func TestStatefulHandler_Handle_ValidStatefulToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	store := &mockStatefulStore{}
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(store).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewStatefulHandler(c, opt, store)

	err = h.Handle()
	assert.Nil(t, err)

	// 验证用户信息已写入
	stx, exists := c.Get(httpcontext.ContextKey)
	assert.True(t, exists)
	httpCtx := stx.(httpcontext.IHttpContext)
	assert.Equal(t, uint(1), httpCtx.User().ID)
}

// TestStatefulHandler_Handle_StoreCheckFailed store 检查失败返回错误
func TestStatefulHandler_Handle_StoreCheckFailed(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	store := &mockStatefulStore{
		checkErr: errors.New("token revoked"),
	}
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(store).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewStatefulHandler(c, opt, store)

	err = h.Handle()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "token revoked")
}

// TestStatefulHandler_Handle_ExpiredToken 严重过期的有状态 token 在解析阶段被拒绝
func TestStatefulHandler_Handle_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	store := &mockStatefulStore{}
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(store).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	// 通过 SetDuration 使 token 过期
	tk.SetDuration(-1 * time.Hour)
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert, pkgjwt.WithLeeway(0))
	h := NewStatefulHandler(c, opt, store)

	err = h.Handle()
	assert.NotNil(t, err)
	// 严重过期的 token 在解析阶段被拒绝
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestStatefulHandler_Handle_WithRefresh 有状态 token 自动续期（解析后的 token 因无 statefulHandler，ToString 会失败）
func TestStatefulHandler_Handle_WithRefresh(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	store := &mockStatefulStore{}
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(store).
		WithExpiration(1 * time.Hour).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert,
		pkgjwt.WithRefreshDuration(2*time.Hour),
	)
	h := NewStatefulHandler(c, opt, store)

	err = h.Handle()
	assert.Nil(t, err)

	// 解析后的有状态 token 在 ToString 时因缺少 statefulHandler 而失败，
	// 所以不会设置续期 token 头（但 handler 本身不会报错）
	newToken := c.Writer.Header().Get(pkgjwt.TokenHeaderKey)
	assert.Empty(t, newToken, "stateful token refresh should fail due to missing statefulHandler after parse")
}

// TestStatefulHandler_Handle_WithoutRefresh 无续期配置时不续期
func TestStatefulHandler_Handle_WithoutRefresh(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	store := &mockStatefulStore{}
	tk, err := pkgjwt.NewTokenBuilder(secret, user).
		WithStatefulStore(store).
		Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert,
		pkgjwt.WithRefreshDuration(0),
	)
	h := NewStatefulHandler(c, opt, store)

	err = h.Handle()
	assert.Nil(t, err)

	newToken := c.Writer.Header().Get(pkgjwt.TokenHeaderKey)
	assert.Empty(t, newToken)
}

// TestStatefulHandler_Handle_OptionWithNilRoleConvert 有状态 handler option 中 roleConvert 为 nil
func TestStatefulHandler_Handle_OptionWithNilRoleConvert(t *testing.T) {
	c := newMiddlewareTestGinContext()
	opt := pkgjwt.NewOption([]byte("test-secret-that-is-long-enough-32b!"), nil)
	h := NewStatefulHandler(c, opt, &mockStatefulStore{})

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestStatefulHandler_Handle_StoreNilButNotSkipCheck 直接构造 statefulHandler，store 为 nil 且 skipCheck=false
func TestStatefulHandler_Handle_StoreNilButNotSkipCheck(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)

	// 直接构造 statefulHandler，模拟 store 被外部移除但 skipCheck 未重置
	h := &statefulHandler{
		ctx:               c,
		opt:               opt,
		store:             nil,
		skipCheckStateful: false,
	}

	err = h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
	assert.Contains(t, err.Error(), "store not configured")
}

// TestNewHandler_ReturnsIHandler 验证 NewHandler 返回 IHandler 接口
func TestNewHandler_ReturnsIHandler(t *testing.T) {
	c := newMiddlewareTestGinContext()
	h := NewHandler(c, nil)

	var _ IHandler = h // 编译时接口检查
}

// TestNewStatefulHandler_ReturnsIHandler 验证 NewStatefulHandler 返回 IHandler 接口
func TestNewStatefulHandler_ReturnsIHandler(t *testing.T) {
	c := newMiddlewareTestGinContext()
	h := NewStatefulHandler(c, nil, nil)

	var _ IHandler = h // 编译时接口检查
}
