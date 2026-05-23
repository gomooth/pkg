package xcode

import (
	"net/http"

	xc "github.com/gomooth/xerror/xcode"
)

// DefineXCode creates and registers a new XCode instance
func DefineXCode(code int, httpStatus int, message string) xc.XCode {
	x := xc.NewWithHTTPStatus(code, httpStatus, message)
	xc.Register(x)
	return x
}

// === 项目特有定义（上游无等价） ===

// 通用错误码 10xxx
var (
	ErrUnknown  = DefineXCode(10000, http.StatusInternalServerError, "未知错误")
	ErrConflict = DefineXCode(10005, http.StatusConflict, "资源冲突")
)

// JWT 错误码 11xxx
var (
	ErrJWTSecretNotSet = DefineXCode(11001, http.StatusInternalServerError, "JWT 密钥未设置")
	ErrJWTTokenInvalid = DefineXCode(11002, http.StatusUnauthorized, "JWT 令牌无效")
	ErrJWTTokenExpired = DefineXCode(11003, http.StatusUnauthorized, "JWT 令牌已过期")
	ErrJWTTokenRevoked = DefineXCode(11004, http.StatusUnauthorized, "JWT 令牌已撤销")
)

// 数据库错误码 12xxx
var (
	ErrDBConnect = DefineXCode(12001, http.StatusInternalServerError, "数据库连接失败")
	// 数据库查询失败请使用 xcode.DBFailed (1001)，不再重复定义
)

// 缓存错误码 13xxx
var (
	ErrCacheMiss           = DefineXCode(13001, http.StatusNotFound, "缓存未命中")
	ErrCacheSetFailed      = DefineXCode(13002, http.StatusInternalServerError, "缓存设置失败")
	ErrCacheReadFailed     = DefineXCode(13003, http.StatusInternalServerError, "缓存读取失败")
	ErrCacheNotInitialized = DefineXCode(13004, http.StatusServiceUnavailable, "缓存未初始化")
)

// 消息队列错误码 14xxx
var (
	ErrMQPublish        = DefineXCode(14001, http.StatusInternalServerError, "消息队列发布失败")
	ErrMQConsume        = DefineXCode(14002, http.StatusInternalServerError, "消息队列消费失败")
	ErrMQRetryExhausted = DefineXCode(14003, http.StatusInternalServerError, "消息队列重试次数耗尽")
)

// 存储错误码 15xxx
var (
	ErrStoragePathInvalid  = DefineXCode(15001, http.StatusBadRequest, "存储路径无效")
	ErrStorageFileNotFound = DefineXCode(15002, http.StatusNotFound, "存储文件不存在")
)
