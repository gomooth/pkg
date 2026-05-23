package restful

import (
	"log/slog"

	"github.com/gomooth/xerror/xcode"

	"golang.org/x/text/language"
)

// WithResponseLanguageHeaderKey 设置多国语言 header 参数 key， 默认为 `X-Language`。
// 主要应用于多国语言展示错误信息。
func WithResponseLanguageHeaderKey(languageHeaderKey string) func(*response) {
	return func(r *response) {
		r.languageHeaderKey = languageHeaderKey
	}
}

// WithResponseErrorMsgHandler 指定错误消息处理器。
// 主要应用于多国语言展示错误信息。
// 其中，`code` 为错误码；`language` 为标准的 i18n 标识
func WithResponseErrorMsgHandler(supported []language.Tag, handle func(code int, lang language.Tag) string) func(*response) {
	return func(r *response) {
		r.supportedLanguages = supported
		r.languageHeaderKey = LangHeaderKey
		r.msgHandler = handle
	}
}

// WithResponseDefaultLanguage 设置默认语言
func WithResponseDefaultLanguage(lang language.Tag) func(*response) {
	return func(r *response) {
		r.defaultedLanguage = lang
	}
}

// WithResponseLogger 指定 Logger
func WithResponseLogger(logger *slog.Logger) func(*response) {
	return func(r *response) {
		r.logger = logger
	}
}

// WithResponseShowXCode 设置允许向客户端展示的错误码白名单。
// 默认情况下，该值为空，表示所有错误码均向客户端展示；
// 设置非空后，仅白名单内的错误码会展示原始消息，其余展示通用 "Internal Server Error"。
func WithResponseShowXCode(xcodes ...xcode.XCode) func(*response) {
	return func(r *response) {
		if len(xcodes) == 0 {
			r.visibleErrorCodes = make([]int, 0)
			return
		}

		for _, err := range xcodes {
			if r.visibleErrorCodes == nil {
				r.visibleErrorCodes = make([]int, 0)
			}
			r.visibleErrorCodes = append(r.visibleErrorCodes, err.Code())
		}
	}
}

// WithResponseDebugError 设置是否在非 xerror 错误中展示原始错误信息。
// 默认 false：非 xerror 错误返回通用 "Internal Server Error" 消息，防止泄露内部细节。
// 开发/测试环境可设为 true 以便调试。
func WithResponseDebugError(debug bool) func(*response) {
	return func(r *response) {
		r.debugError = debug
	}
}

// WithResponseRelaxedHeaders 放松 SetHeader 的 X- 前缀限制。
// 默认情况下，SetHeader 仅允许设置以 X- 开头的自定义头；
// 启用此选项后，SetHeader 允许设置任意请求头。
func WithResponseRelaxedHeaders() func(*response) {
	return func(r *response) {
		r.strictHeaders = false
	}
}
