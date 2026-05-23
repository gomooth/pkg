package jwt

import (
	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/metrics"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/xerror"

	"go.opentelemetry.io/otel/metric"
)

type statefulHandler struct {
	ctx   *gin.Context
	opt   *jwt.Option
	store jwt.StatefulStore

	skipCheckStateful bool
}

func NewStatefulHandler(ctx *gin.Context, opt *jwt.Option, store jwt.StatefulStore) IHandler {
	return &statefulHandler{
		ctx:   ctx,
		opt:   opt,
		store: store,

		skipCheckStateful: store == nil,
	}
}

// Handle 鉴权处理
// 只负责验证是否登陆，不处理其他事务
func (h *statefulHandler) Handle() error {
	if h.opt == nil || h.opt.RoleConvert() == nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			metrics.Attr("handler", "stateful"),
			metrics.Attr("result", "parse_error"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: option is empty")
	}

	if !h.skipCheckStateful && h.store == nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			metrics.Attr("handler", "stateful"),
			metrics.Attr("result", "parse_error"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: stateful token required, but store not configured")
	}

	tokenStr, token, err := jwt.ParseTokenWithGinAndOption(h.ctx, h.opt)
	if err != nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			metrics.Attr("handler", "stateful"),
			metrics.Attr("result", "parse_error"),
		))
		return xerror.WrapWithXCode(err, xcode.ErrJWTTokenInvalid)
	}

	if token.IsExpired() {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			metrics.Attr("handler", "stateful"),
			metrics.Attr("result", "expired"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenExpired, "jwt: token expired")
	}

	if !token.IsStateful() {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			metrics.Attr("handler", "stateful"),
			metrics.Attr("result", "parse_error"),
		))
		return xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: stateful token required, use JWTStatefulWith middleware")
	}

	user, err := token.GetUser(h.opt.RoleConvert())
	if err != nil {
		jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
			metrics.Attr("handler", "stateful"),
			metrics.Attr("result", "parse_error"),
		))
		return err
	}

	if !h.skipCheckStateful {
		if err := h.store.Check(h.ctx.Request.Context(), user.GetID(), tokenStr); err != nil {
			jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
				metrics.Attr("handler", "stateful"),
				metrics.Attr("result", "stateful_check_error"),
			))
			return err
		}

		if h.opt.RefreshDuration() > 0 {
			token.RefreshNear(h.opt.RefreshDuration())
			if newToken, err := token.ToString(h.ctx.Request.Context()); err == nil {
				h.ctx.Header(jwt.TokenHeaderKey, newToken)
				jwtTokenRefreshCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
					metrics.Attr("handler", "stateful"),
					metrics.Attr("result", "success"),
				))
			} else {
				jwtTokenRefreshCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
					metrics.Attr("handler", "stateful"),
					metrics.Attr("result", "failure"),
				))
			}
		}
	}

	ensureUserInContext(h.ctx, user)

	jwtOperationCounter.Add(h.ctx.Request.Context(), 1, metric.WithAttributes(
		metrics.Attr("handler", "stateful"),
		metrics.Attr("result", "success"),
	))
	return nil
}
