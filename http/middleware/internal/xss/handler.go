package xss

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/xss"
	"github.com/gomooth/xerror"
	"github.com/microcosm-cc/bluemonday"
)

const maxMultipartPartSize = 32 << 20 // 32MB

const defaultMaxBodySize = 10 << 20 // 10MB

type handler struct {
	xssRuleItem

	debug             bool
	trimSpaceEnabled  bool
	passwordFieldName []string
	maxBodySize       int64 // 请求体最大字节数，0 表示使用默认值

	// 路由特殊规则
	routePolicies map[string]*xssRuleItem
	// 直接跳过的路由
	skipRoutes map[string]struct{}
}

func New(opts ...Option) gin.HandlerFunc {
	xf := &handler{
		xssRuleItem: xssRuleItem{
			skipField: make(map[string]struct{}, 0),
		},
		debug:            false,
		trimSpaceEnabled: false,
		passwordFieldName: []string{
			"password", "newPassword", "oldPassword", "confirmedPassword",
			"pwd", "newPwd", "oldPwd", "confirmedPwd",
		},
		routePolicies: make(map[string]*xssRuleItem),
		skipRoutes:    make(map[string]struct{}),
	}
	// 默认启用严格策略，防止 XSS 注入。可通过 WithPolicy(PolicyNone) 显式关闭
	xf.policy = xf.makePolicy(xss.PolicyStrict)

	for _, opt := range opts {
		opt(xf)
	}

	return xf.filter()
}

func (h *handler) makePolicy(p xss.Policy) *bluemonday.Policy {
	switch p {
	case xss.PolicyNone:
		return nil
	case xss.PolicyStrict:
		return xss.DefaultStrictPolicy()
	case xss.PolicyUGC:
		return xss.DefaultUGCPolicy()
	default:
		return xss.DefaultStrictPolicy()
	}
}

func (h *handler) makeSkipFields(fields []string) map[string]struct{} {
	vals := make(map[string]struct{})

	// 默认跳过密码字段
	for _, s := range h.passwordFieldName {
		vals[s] = struct{}{}
	}

	for _, field := range fields {
		vals[field] = struct{}{}
	}

	return vals
}

// matchRoute 检查请求路径是否匹配指定路由
// 支持精确匹配和尾斜杠前缀匹配
func matchRoute(requestPath, route string) bool {
	if requestPath == route {
		return true
	}
	return strings.HasPrefix(requestPath, route+"/")
}

func (h *handler) filter() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 指定了忽略路由，直接跳过
		for u := range h.skipRoutes {
			if matchRoute(ctx.FullPath(), u) {
				h.debugf("xss handler hit skip route, skip\n")
				ctx.Next()
				return
			}
		}

		// 未指定全局规则，而且未指定路由规则，直接跳过
		if h.policy == nil {
			skip := true
			for u := range h.routePolicies {
				if matchRoute(ctx.FullPath(), u) {
					skip = false
					break
				}
			}
			if skip {
				h.debugf("xss handler no global policy, not hit route rule, skip\n")
				ctx.Next()
				return
			}
		}

		var err error

		switch ctx.Request.Method {
		case http.MethodGet:
			err = h.filterQueryString(ctx)
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			ct := ctx.Request.Header.Get("Content-Type")
			mediaType, _, _ := mime.ParseMediaType(ct)
			switch mediaType {
			case "application/json":
				err = h.filterJSON(ctx)
			case "application/x-www-form-urlencoded":
				err = h.filterFormData(ctx)
			case "multipart/form-data":
				err = h.filterMultiPartFormData(ctx)
			}
		}

		if err != nil {
			if xe, ok := err.(xerror.XError); ok {
				err = xe.Unwrap()
			}
			slog.Error("xss handler error", slog.String("component", "xss"), slog.String("error", err.Error()))
			_ = ctx.AbortWithError(http.StatusBadRequest, fmt.Errorf("request content filtered"))
			return
		}

		ctx.Next()
	}
}

func (h *handler) filterXSS(fullPath, key, val string) string {
	isPassword := false
	for _, s := range h.passwordFieldName {
		if s == key {
			isPassword = true
			break
		}
	}
	if h.trimSpaceEnabled && !isPassword {
		val = strings.TrimSpace(val)
	}

	for route, item := range h.routePolicies {
		if matchRoute(fullPath, route) {
			if _, ok := item.skipField[key]; ok {
				h.debugf("xss handler hit route skip field, return origin value\n")
				return val
			}

			fieldPolicy, ok := item.fieldRules[key]
			if ok && fieldPolicy != nil {
				h.debugf("xss handler hit route field rule, return sanitize value\n")
				return fieldPolicy.Sanitize(val)
			}

			if item.policy == nil {
				h.debugf("xss handler hit route rule, none policy, return origin value\n")
				return val
			}

			h.debugf("xss handler hit route rule, return sanitize value\n")
			return item.policy.Sanitize(val)
		}
	}

	if _, ok := h.skipField[key]; ok {
		h.debugf("xss handler hit global skip field, return origin value\n")
		return val
	}

	fieldPolicy, ok := h.fieldRules[key]
	if ok && fieldPolicy != nil {
		h.debugf("xss handler hit global field rule, return sanitize value\n")
		return fieldPolicy.Sanitize(val)
	}

	if h.policy == nil {
		h.debugf("xss handler hit global policy, return sanitize value\n")
		return val
	}

	h.debugf("xss handler hit global policy, return sanitize value\n")
	return h.policy.Sanitize(val)
}

func (h *handler) debugf(format string, vals ...any) {
	if h.debug {
		slog.Debug(fmt.Sprintf(format, vals...), slog.String("component", "xss"))
	}
}

func (h *handler) filterQueryString(ctx *gin.Context) error {
	params := ctx.Request.URL.Query()
	h.debugf("xss handler input query string: %s\n", params.Encode())
	for key, items := range params {
		params.Del(key)
		for _, val := range items {
			val = h.filterXSS(ctx.FullPath(), key, val)
			if params.Has(key) {
				params.Add(key, val)
			} else {
				params.Set(key, val)
			}
		}
	}

	h.debugf("xss handler output query string: %s\n", params.Encode())
	ctx.Request.URL.RawQuery = params.Encode()
	return nil
}

func (h *handler) filterJSON(ctx *gin.Context) error {
	body := ctx.Request.Body
	if body == nil || body == http.NoBody {
		return nil
	}

	maxSize := h.maxBodySize
	if maxSize <= 0 {
		maxSize = defaultMaxBodySize
	}

	// 限制读取大小，防止 OOM
	bs, err := io.ReadAll(io.LimitReader(body, maxSize+1))
	if err != nil {
		return xerror.Wrap(err, "read body failed")
	}
	if int64(len(bs)) > maxSize {
		return xerror.New("request body exceeds size limit")
	}

	d := json.NewDecoder(bytes.NewReader(bs))
	d.UseNumber()

	var val any
	if err := d.Decode(&val); err != nil {
		return xerror.Wrap(err, "json decode failed")
	}

	h.debugf("xss handler input json: %s\n", val)

	var data any
	switch val.(type) {
	case map[string]any:
		vals := make(map[string]any, 0)
		for k, v := range val.(map[string]any) {
			vals[k] = h.filterJsonValue(ctx.FullPath(), k, v)
		}
		data = vals
	case []any:
		vals := make([]any, 0)
		for _, v := range val.([]any) {
			vals = append(vals, h.filterJsonValue(ctx.FullPath(), "", v))
		}
		data = vals
	default:
		data = val
	}

	var bf bytes.Buffer
	encode := json.NewEncoder(&bf)
	encode.SetEscapeHTML(false)
	if err := encode.Encode(data); err != nil {
		return xerror.Wrap(err, "json encode failed")
	}

	h.debugf("xss handler output json: %s\n", bf.String())
	ctx.Request.Body = io.NopCloser(&bf)
	return nil
}

func (h *handler) filterJsonValue(fullPath, key string, val any) any {
	switch val.(type) {
	case map[string]any:
		vals := make(map[string]any, 0)
		for k, v := range val.(map[string]any) {
			vals[k] = h.filterJsonValue(fullPath, key, v)
		}
		return vals
	case []any:
		vals := make([]any, 0)
		for _, v := range val.([]any) {
			vals = append(vals, h.filterJsonValue(fullPath, key, v))
		}
		return vals
	case string:
		return h.filterXSS(fullPath, key, val.(string))
	default:
		return val
	}
}

func (h *handler) filterFormData(ctx *gin.Context) error {
	body := ctx.Request.Body
	if body == nil || body == http.NoBody {
		return nil
	}

	maxSize := h.maxBodySize
	if maxSize <= 0 {
		maxSize = defaultMaxBodySize
	}

	// 限制读取大小，防止 OOM
	var buf bytes.Buffer
	n, err := buf.ReadFrom(io.LimitReader(body, maxSize+1))
	if err != nil {
		return xerror.Wrap(err, "read from failed")
	}
	if n > maxSize {
		return xerror.New("request body exceeds size limit")
	}

	h.debugf("xss handler input x-form-data: %s\n", buf.String())

	m, err := url.ParseQuery(buf.String())
	if err != nil {
		return xerror.Wrap(err, "parse query failed")
	}

	var bf bytes.Buffer
	for key, v := range m {
		var filteredVals []string
		for _, item := range v {
			filteredVals = append(filteredVals, url.QueryEscape(h.filterXSS(ctx.FullPath(), key, item)))
		}

		if bf.Len() > 0 {
			bf.WriteByte('&')
		}
		bf.WriteString(key)
		bf.WriteByte('=')
		bf.WriteString(strings.Join(filteredVals, "&"+key+"="))
	}

	h.debugf("xss handler output x-form-data: %s\n", bf.String())
	ctx.Request.Body = io.NopCloser(&bf)
	return nil
}

func (h *handler) filterMultiPartFormData(ctx *gin.Context) error {
	body := ctx.Request.Body
	if body == nil || body == http.NoBody {
		return nil
	}

	ct := ctx.Request.Header.Get("Content-Type")
	_, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return xerror.Wrap(err, "parse content-type failed")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return xerror.New("multipart: missing boundary in Content-Type")
	}
	reader := multipart.NewReader(body, boundary)

	h.debugf("xss handler enter multi-part-form-data\n")

	var bf bytes.Buffer
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		// https://golang.org/src/mime/multipart/multipart_test.go line 230
		bf.WriteString("--")
		bf.WriteString(boundary)
		bf.WriteString("\r\n")

		//val := make([]byte, 0)
		//_, err = part.Read(val)
		var buf bytes.Buffer
		n, err := io.CopyN(&buf, part, maxMultipartPartSize+1)
		if err != nil && err != io.EOF {
			return xerror.Wrap(err, "copy body failed")
		}
		if n > maxMultipartPartSize {
			return xerror.New("multipart part size exceeds limit")
		}
		val := buf.String()

		bf.WriteString(`Content-Disposition: form-data; name="`)
		bf.WriteString(part.FormName())
		bf.WriteString(`";`)

		if part.FileName() != "" {
			// Content-Disposition: form-data; name="file"; filename="文件.zip"
			bf.WriteString(` filename="`)
			bf.WriteString(part.FileName())
			bf.WriteString("\";\r\n")

			// Content-Type: application/octet-stream
			partCt := part.Header.Get("Content-Type")
			if partCt == "" {
				partCt = `application/octet-stream`
			}
			bf.WriteString("Content-Type: ")
			bf.WriteString(partCt)
			bf.WriteString("\r\n\r\n")
		} else {
			// Content-Disposition: form-data; name="file"
			bf.WriteString("\r\n\r\n")

			if _, ok := h.skipField[part.FormName()]; !ok {
				val = h.filterXSS(ctx.FullPath(), part.FormName(), val)
			}
		}

		bf.WriteString(val)
		bf.WriteString("\r\n")
	}

	bf.WriteString("--")
	bf.WriteString(boundary)
	bf.WriteString("--\r\n")

	ctx.Request.Body = io.NopCloser(&bf)
	return nil
}
