package httpcontext

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
)

const (
	ContextKey = "gomooth_http_context"

	RequestRawBodyDataKey = "request_raw_body"
)

// MustParse 从 gin 上下文中解析自定义上下文
func MustParse(ctx context.Context) (IHttpContext, error) {
	gtx, ok := ctx.(*gin.Context)
	if !ok {
		return nil, xerror.NewXCode(xcode.InternalServerError, "to GinContext failed")
	}

	v, ok := gtx.Get(ContextKey)
	if !ok {
		return nil, xerror.NewXCode(xcode.InternalServerError, "get HttpCustomContext failed")
	}

	rtx, ok := v.(IHttpContext)
	if !ok {
		return nil, xerror.NewXCode(xcode.InternalServerError, "to HttpContext failed")
	}

	return rtx, nil
}
