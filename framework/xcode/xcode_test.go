package xcode

import (
	"net/http"
	"testing"

	"github.com/gomooth/xerror/xcode"
	"github.com/stretchr/testify/assert"
)

func TestProjectSpecificCodes(t *testing.T) {
	// 项目特有错误码
	assertCode(t, ErrUnknown, 10000, http.StatusInternalServerError, "未知错误")
	assertCode(t, ErrConflict, 10005, http.StatusConflict, "资源冲突")

	// JWT 错误码 11xxx
	assertCode(t, ErrJWTSecretNotSet, 11001, http.StatusInternalServerError, "JWT 密钥未设置")
	assertCode(t, ErrJWTTokenInvalid, 11002, http.StatusUnauthorized, "JWT 令牌无效")
	assertCode(t, ErrJWTTokenExpired, 11003, http.StatusUnauthorized, "JWT 令牌已过期")
	assertCode(t, ErrJWTTokenRevoked, 11004, http.StatusUnauthorized, "JWT 令牌已撤销")

	// 数据库错误码 12xxx
	assertCode(t, ErrDBConnect, 12001, http.StatusInternalServerError, "数据库连接失败")

	// 缓存错误码 13xxx
	assertCode(t, ErrCacheMiss, 13001, http.StatusNotFound, "缓存未命中")
	assertCode(t, ErrCacheSetFailed, 13002, http.StatusInternalServerError, "缓存设置失败")
	assertCode(t, ErrCacheReadFailed, 13003, http.StatusInternalServerError, "缓存读取失败")
	assertCode(t, ErrCacheNotInitialized, 13004, http.StatusServiceUnavailable, "缓存未初始化")

	// 消息队列错误码 14xxx
	assertCode(t, ErrMQPublish, 14001, http.StatusInternalServerError, "消息队列发布失败")
	assertCode(t, ErrMQConsume, 14002, http.StatusInternalServerError, "消息队列消费失败")
	assertCode(t, ErrMQRetryExhausted, 14003, http.StatusInternalServerError, "消息队列重试次数耗尽")

	// 存储错误码 15xxx
	assertCode(t, ErrStoragePathInvalid, 15001, http.StatusBadRequest, "存储路径无效")
	assertCode(t, ErrStorageFileNotFound, 15002, http.StatusNotFound, "存储文件不存在")
}

func TestAllErrorCodesAreUnique(t *testing.T) {
	codes := make(map[int]struct{})

	assertUniqueCode(t, codes, ErrUnknown)
	assertUniqueCode(t, codes, ErrConflict)

	assertUniqueCode(t, codes, ErrJWTSecretNotSet)
	assertUniqueCode(t, codes, ErrJWTTokenInvalid)
	assertUniqueCode(t, codes, ErrJWTTokenExpired)
	assertUniqueCode(t, codes, ErrJWTTokenRevoked)

	assertUniqueCode(t, codes, ErrDBConnect)

	assertUniqueCode(t, codes, ErrCacheMiss)
	assertUniqueCode(t, codes, ErrCacheSetFailed)
	assertUniqueCode(t, codes, ErrCacheReadFailed)
	assertUniqueCode(t, codes, ErrCacheNotInitialized)

	assertUniqueCode(t, codes, ErrMQPublish)
	assertUniqueCode(t, codes, ErrMQConsume)
	assertUniqueCode(t, codes, ErrMQRetryExhausted)

	assertUniqueCode(t, codes, ErrStoragePathInvalid)
	assertUniqueCode(t, codes, ErrStorageFileNotFound)
}

func TestErrorCodesImplementXCodeInterface(t *testing.T) {
	var _ xcode.XCode = ErrUnknown
	var _ xcode.XCode = ErrJWTTokenExpired
	var _ xcode.XCode = ErrDBConnect
	var _ xcode.XCode = ErrCacheMiss
	var _ xcode.XCode = ErrMQPublish
	var _ xcode.XCode = ErrStoragePathInvalid
}

func assertCode(t *testing.T, xc xcode.XCode, code int, httpStatus int, message string) {
	assert.Equal(t, code, xc.Code(), "code mismatch")
	assert.Equal(t, httpStatus, xc.HttpStatus(), "http status mismatch")
	assert.Equal(t, message, xc.String(), "message mismatch")
}

func assertUniqueCode(t *testing.T, codes map[int]struct{}, xc xcode.XCode) {
	code := xc.Code()
	if _, exists := codes[code]; exists {
		t.Errorf("Duplicate error code found: %d (%s)", code, xc.String())
	}
	codes[code] = struct{}{}
}
