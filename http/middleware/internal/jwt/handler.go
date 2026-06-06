package jwt

import (
	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/xerror"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	jwtOperationCounter    metric.Int64Counter
	jwtTokenRefreshCounter metric.Int64Counter
)

func init() {
	telemetry.OnProviderSet(func() {
		m := telemetry.Meter("jwt")
		jwtOperationCounter, _ = m.Int64Counter("jwt.operation")
		jwtTokenRefreshCounter, _ = m.Int64Counter("jwt.token.refresh")
	})
}

type IHandler interface {
	Handle() error
}

type handler struct {
	ctx *gin.Context
	opt *jwt.Option
}

func NewHandler(ctx *gin.Context, opt *jwt.Option) IHandler {
	return &handler{
		ctx: ctx,
		opt: opt,
	}
}

// Handle 鉴权处理
// 只负责验证是否登陆，不处理其他事务
func (h *handler) Handle() error {
	if h.opt == nil || h.opt.RoleConvert() == nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			attribute.String("handler", "stateless"),
			attribute.String("result", "parse_error"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: option is empty")
	}

	_, token, err := jwt.ParseTokenWithGinAndOption(h.ctx, h.opt)
	if err != nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			attribute.String("handler", "stateless"),
			attribute.String("result", "parse_error"),
		))
		return xerror.WrapWithXCode(err, xcode.ErrJWTTokenInvalid)
	}

	if token.IsExpired() {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			attribute.String("handler", "stateless"),
			attribute.String("result", "expired"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenExpired, "jwt: token expired")
	}

	if token.IsStateful() {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			attribute.String("handler", "stateless"),
			attribute.String("result", "parse_error"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: stateless token required, use JWTWith middleware")
	}

	user, err := token.GetUser(h.opt.RoleConvert())
	if err != nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			attribute.String("handler", "stateless"),
			attribute.String("result", "parse_error"),
		))
		return err
	}

	if h.opt.RefreshDuration() > 0 {
		token.RefreshNear(h.opt.RefreshDuration())
		if newToken, err := token.ToString(h.ctx.Request.Context()); err == nil {
			h.ctx.Header(jwt.TokenHeaderKey, newToken)
			jwtTokenRefreshCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
				attribute.String("handler", "stateless"),
				attribute.String("result", "success"),
			))
		} else {
			jwtTokenRefreshCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
				attribute.String("handler", "stateless"),
				attribute.String("result", "failure"),
			))
		}
	}

	ensureUserInContext(h.ctx, user)

	jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
		attribute.String("handler", "stateless"),
		attribute.String("result", "success"),
	))
	return nil
}

// ensureUserInContext 将用户信息写入 httpcontext，若 context 不存在则自动创建。
// 若 stx 中已有用户信息则跳过（幂等），避免多个 JWT 中间件重复覆盖。
func ensureUserInContext(ctx *gin.Context, user *httpcontext.User) {
	var stx httpcontext.IHttpContext
	if v, ok := ctx.Get(httpcontext.ContextKey); ok {
		stx, _ = v.(httpcontext.IHttpContext)
	}
	if stx == nil {
		stx = httpcontext.NewContext()
	} else if stx.User() != nil {
		return
	}
	stx.SetUser(*user).StorageTo(ctx)
}
