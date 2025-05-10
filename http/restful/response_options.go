package restful

import (
	"github.com/save95/xerror/xcode"
	"github.com/save95/xlog"

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
func WithResponseLogger(logger xlog.XLog) func(*response) {
	return func(r *response) {
		r.logger = logger
	}
}

// WithResponseShowXCode 设置需要对客户端展示的错误码。
// 默认情况下，该值为空，则表示所有错误码均向用户展示（设置为空，亦如此）；
func WithResponseShowXCode(xcodes ...xcode.XCode) func(*response) {
	return func(r *response) {
		if len(xcodes) == 0 {
			r.showErrorCodes = make([]int, 0)
			return
		}

		for _, err := range xcodes {
			if r.showErrorCodes == nil {
				r.showErrorCodes = make([]int, 0)
			}
			r.showErrorCodes = append(r.showErrorCodes, err.Code())
		}
	}
}
