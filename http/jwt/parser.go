package jwt

import (
	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
)

// ParseJWTUser 解析 jwt User 信息。如果是静默模式（SilentMode=true）， User.ID 可能为零
func ParseJWTUser(ctx *gin.Context, opt *Option) (*httpcontext.User, error) {
	if opt == nil || opt.RoleConvert() == nil {
		return nil, xerror.NewXCode(xcode.ErrJWTTokenInvalid, "jwt: option is empty")
	}

	_, tk, err := ParseTokenWithSecret(ctx, opt.Secret(), opt.LegacySecrets()...)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.ErrJWTTokenInvalid)
	}

	if tk.IsExpired() {
		return nil, xerror.NewXCode(xcode.ErrJWTTokenExpired, "jwt: token expired")
	}

	user, err := tk.GetUser(opt.RoleConvert())
	if err != nil {
		if !opt.SilentMode() {
			return nil, err
		}
	}

	return user, nil
}
