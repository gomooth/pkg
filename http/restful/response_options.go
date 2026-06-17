package restful

import (
	"log/slog"

	"github.com/gomooth/xerror/xcode"

	"golang.org/x/text/language"
)

// WithResponseLanguageHeaderKey 设置多国语言 header 参数 key， 默认为 `X-Language`。
// 主要应用于多国语言展示错误信息。
func WithResponseLanguageHeaderKey(languageHeaderKey string) ResponseOption {
	return func(o *responseOption) {
		o.languageHeaderKey = languageHeaderKey
	}
}

// WithResponseErrorMsgHandler 指定错误消息处理器。
// 主要应用于多国语言展示错误信息。
// 其中，`code` 为错误码；`language` 为标准的 i18n 标识
func WithResponseErrorMsgHandler(supported []language.Tag, handle func(code int, lang language.Tag) string) ResponseOption {
	return func(o *responseOption) {
		o.supportedLanguages = supported
		o.languageHeaderKey = LangHeaderKey
		o.msgHandler = handle
	}
}

// WithResponseDefaultLanguage 设置默认语言
func WithResponseDefaultLanguage(lang language.Tag) ResponseOption {
	return func(o *responseOption) {
		o.defaultedLanguage = lang
	}
}

// WithResponseLogger 指定 Logger
func WithResponseLogger(logger *slog.Logger) ResponseOption {
	return func(o *responseOption) {
		o.logger = logger
	}
}

// WithResponseShowXCode 设置允许向客户端展示的错误码白名单。
// 默认情况下（未调用此函数），所有错误码均向客户端展示。
// 调用此函数后，仅白名单内的错误码会展示原始消息，其余展示通用 "Internal Server Error"。
// 无参数调用会 panic，防止意外将白名单设为空（语义歧义）。
// 如需允许所有错误码，请使用 WithResponseShowAllXCode()。
func WithResponseShowXCode(xcodes ...xcode.XCode) ResponseOption {
	if len(xcodes) == 0 {
		panic("restful: WithResponseShowXCode requires at least one xcode, use WithResponseShowAllXCode() to allow all")
	}
	return func(o *responseOption) {
		o.visibleErrorCodes = make([]int, 0, len(xcodes))
		for _, err := range xcodes {
			o.visibleErrorCodes = append(o.visibleErrorCodes, err.Code())
		}
	}
}

// WithResponseShowAllXCode 显式允许所有错误码向客户端展示。
// 此函数用于在之前调用 WithResponseShowXCode 后恢复默认行为。
func WithResponseShowAllXCode() ResponseOption {
	return func(o *responseOption) {
		o.visibleErrorCodes = nil
	}
}

// WithResponseDebugError 设置是否在非 xerror 错误中展示原始错误信息。
// 默认 false：非 xerror 错误返回通用 "Internal Server Error" 消息，防止泄露内部细节。
// 开发/测试环境可设为 true 以便调试。
func WithResponseDebugError(debug bool) ResponseOption {
	return func(o *responseOption) {
		o.debugError = debug
	}
}

// WithResponseRelaxedHeaders 放松 SetHeader 的 X- 前缀限制。
// 默认情况下，SetHeader 仅允许设置以 X- 开头的自定义头；
// 启用此选项后，SetHeader 允许设置任意请求头。
func WithResponseRelaxedHeaders() ResponseOption {
	return func(o *responseOption) {
		o.strictHeaders = false
	}
}