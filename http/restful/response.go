package restful

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/framework/pager"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/utils/valutil"

	"github.com/gin-gonic/gin"

	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"

	"golang.org/x/text/language"
)

type response struct {
	ctx    *gin.Context
	logger *slog.Logger

	languageHeaderKey  string
	supportedLanguages []language.Tag
	msgHandler         func(code int, lang language.Tag) string
	defaultedLanguage  language.Tag

	// visibleErrorCodes 允许向客户端展示的错误码白名单
	// 空=全部可见，非空=仅白名单内的错误码可见
	visibleErrorCodes []int

	debugError    bool // 调试模式：非 xerror 错误显示原始信息
	strictHeaders bool // 严格头部模式：仅允许 X- 前缀的自定义头，默认 true
}

// NewResponse 创建 Restful 标准响应生成器
//
// usage:
// 基础用法
// rru := restful.NewResponse(ctx)
//
// 启用语言包
//
//	rru := restful.NewResponse(
//		ctx,
//		restful.WithResponseErrorMsgHandler(lang.Handle()),
//	)
func NewResponse(ctx *gin.Context, opts ...func(*response)) IResponse {
	resp := &response{
		ctx:               ctx,
		visibleErrorCodes: make([]int, 0),
		strictHeaders:     true,
	}

	for _, opt := range opts {
		opt(resp)
	}

	if resp.logger == nil {
		resp.logger = logger.NewConsoleLogger()
	}

	return resp
}

// allowedStandardHeaders 严格模式下允许设置的标准 HTTP 头白名单
var allowedStandardHeaders = map[string]bool{
	"Content-Type":        true,
	"Content-Disposition": true,
	"Location":            true,
	"Cache-Control":       true,
}

// SetHeader 设置请求头
func (r *response) SetHeader(key, value string) IResponse {
	// 严格模式下，仅允许 X- 前缀自定义头和白名单内的标准头
	if r.strictHeaders && !strings.HasPrefix(key, "X-") && !strings.HasPrefix(key, "x-") && !allowedStandardHeaders[key] {
		return r
	}

	r.ctx.Header(key, value)
	return r
}

// Retrieve 查询单个资源的响应
func (r *response) Retrieve(entity any) {
	//r.ctx.Header("Content-MD5", fmt.Sprintf("%x", md5.Sum([]byte())))
	if entity == nil {
		r.ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusOK, entity)
}

// TableWithPagination 表格分页响应
func (r *response) TableWithPagination(resp *TableResponse) {
	// 写响应页码
	r.writeResponsePagination(resp.TotalRow)

	rows := make(map[string]map[string]any)
	for _, item := range resp.Items {
		row, ok := rows[item.RowKey]
		if !ok {
			row = make(map[string]any)
		}

		row[item.Column] = item.Data
		rows[item.RowKey] = row
	}

	extends := make(map[string]any)
	for _, item := range resp.Extends {
		if _, ok := extends[item.RowKey]; !ok {
			extends[item.RowKey] = item.Data
		}
	}

	//r.ctx.Header("Content-MD5", "")

	r.ctx.AbortWithStatusJSON(http.StatusOK, map[string]any{
		"columns": resp.Columns,
		"rowKeys": resp.RowKeys,
		"data":    rows,
		"extends": extends,
	})
}

// writeResponsePagination 写响应的分页数据
func (r *response) writeResponsePagination(totalRow uint) {
	// 设置总记录数
	r.ctx.Header(TotalCountHeaderKey, strconv.Itoa(int(totalRow)))

	// 解析URL，Query string
	currentUri := r.ctx.Request.RequestURI
	urls, err := url.Parse(currentUri)
	if err != nil {
		r.WithError(xerror.Wrap(err, "parse uri failed"))
		return
	}

	qs := urls.Query()
	start := valutil.Int(qs.Get("start"))
	if start < 0 {
		start = 0
	}
	limit := valutil.IntWith(qs.Get("limit"), pager.DefaultPageSize)
	if limit <= 0 {
		limit = pager.DefaultPageSize
	}

	// 计算分页信息（纯整数运算，limit 已保证 > 0）
	page := uint(start/limit) + 1
	var count uint
	if totalRow == 0 {
		count = 0
	} else {
		count = (totalRow + uint(limit) - 1) / uint(limit)
	}

	// 设置分页信息
	r.ctx.Header(PageInfoHeaderKey, fmt.Sprintf(
		`count="%d", rows="%d", current="%d", size="%d"`,
		count,
		totalRow,
		page,
		limit,
	))

	// 计算分页url
	firstUri := r.ComputePaginateUri(urls, 0)

	var prevStart int
	if start >= limit {
		prevStart = start - limit
	}
	prevUri := r.ComputePaginateUri(urls, prevStart)

	var nextStart int
	if count > 0 && page < count {
		nextStart = int(page * uint(limit))
	} else if count > 0 {
		nextStart = int((count - 1) * uint(limit))
	}
	nextUri := r.ComputePaginateUri(urls, nextStart)

	var lastStart int
	if count > 0 {
		lastStart = int((count - 1) * uint(limit))
	}
	lastUri := r.ComputePaginateUri(urls, lastStart)

	links := fmt.Sprintf(
		`<%s>; rel="self", <%s>; rel="previous", <%s>; rel="next", <%s>; rel="first", <%s>; rel="last"`,
		currentUri,
		prevUri,
		nextUri,
		firstUri,
		lastUri,
	)
	r.ctx.Header(PageLinkHeaderKey, links)
}

// ListWithPagination 分页列表的响应
func (r *response) ListWithPagination(totalRow uint, entities any) {
	r.writeResponsePagination(totalRow)
	r.ctx.AbortWithStatusJSON(http.StatusOK, nilSafeSlice(entities))
}

func (r *response) ComputePaginateUri(urls *url.URL, start int) string {
	qs := urls.Query()
	qs.Set("start", strconv.Itoa(start))
	if start == 0 {
		qs.Del("start")
	}

	if len(qs.Encode()) == 0 {
		return urls.Path
	}

	return fmt.Sprintf("%s?%s", urls.Path, qs.Encode())
}

// ListWithMoreFlag 查询列表的响应
func (r *response) ListWithMoreFlag(hasMore bool, entities any) {
	r.ctx.Header(HasMoreHeaderKey, strconv.FormatBool(hasMore))
	r.ctx.AbortWithStatusJSON(http.StatusOK, nilSafeSlice(entities))
}

// ListWithCursor 游标分页列表的响应，通过 X-Next-Cursor header 返回下一页游标
func (r *response) ListWithCursor(nextCursor string, entities any) {
	if len(nextCursor) > 0 {
		r.ctx.Header(NextCursorHeaderKey, nextCursor)
	}
	r.ctx.AbortWithStatusJSON(http.StatusOK, nilSafeSlice(entities))
}

// Post 新增请求的响应
func (r *response) Post(entity any) {
	if entity == nil {
		r.WithError(xerror.New("post must has response entity"))
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusCreated, entity)
}

// Put 全量更新资源的响应
func (r *response) Put(entity any) {
	if entity == nil {
		r.WithError(xerror.New("put must has response entity"))
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusOK, entity)
}

// Patch 部分更新资源的响应
// 部分 cdn 服务商不支持 http patch 方法，如 阿里云
func (r *response) Patch(entity any) {
	if entity == nil {
		r.ctx.AbortWithStatus(http.StatusNoContent)
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusOK, entity)
}

// Delete 删除的响应
func (r *response) Delete(err error) {
	if err != nil {
		r.WithError(err)
		return
	}

	r.ctx.AbortWithStatus(http.StatusNoContent)
}

// WithMessage 通过 json 响应文本消息: {"message": "something..."}
func (r *response) WithMessage(msg string) {
	if len(msg) == 0 {
		msg = "success"
	}

	r.ctx.AbortWithStatusJSON(http.StatusOK, gin.H{
		"message": msg,
	})
}

// WithBody 响应文本消息
func (r *response) WithBody(body string) {
	r.ctx.String(http.StatusOK, "%s", body)
}

// WithError 响应错误消息(HttpStatus!=200)
func (r *response) WithError(err error) {
	if err == nil {
		r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "error not defined",
		})
		return
	}

	// 重建 Request.Body，使后续日志中间件能读取请求体（默认 Body 只能读一次）
	rebuildRequestBody(r.ctx)

	// 设置错误
	_ = r.ctx.Error(err)

	var e xerror.XError
	if errors.As(err, &e) {
		// 设置错误码，方便前端使用
		r.ctx.Header(ErrorCodeHeaderKey, strconv.Itoa(e.ErrorCode()))

		r.ctx.AbortWithStatusJSON(e.HttpStatus(), gin.H{
			"message": r.getErrorMsg(e),
		})
		return
	}

	if r.debugError {
		r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
	} else {
		r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": http.StatusText(http.StatusInternalServerError),
		})
	}
}

// rebuildRequestBody 重建 Request.Body，使后续日志中间件能读取请求体
// 仅当 httpcontext 中存在缓存的原始 body 数据时才重建，避免不必要的操作
func rebuildRequestBody(c *gin.Context) {
	stx, se := httpcontext.MustParse(c)
	if se != nil {
		return
	}
	val := stx.Value(httpcontext.RequestRawBodyDataKey)
	bs, ok := val.([]byte)
	if !ok || len(bs) == 0 {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bs))
}

// isErrorCodeVisible 判断错误码是否允许向客户端展示
// 空 visibleErrorCodes = 全部可见，非空 = 仅白名单内可见
func (r *response) isErrorCodeVisible(code int) bool {
	if len(r.visibleErrorCodes) == 0 {
		return true
	}

	for _, c := range r.visibleErrorCodes {
		if code == c {
			return true
		}
	}

	return false
}

func (r *response) getErrorMsg(err xerror.XError) string {
	// 默认展示统一响应为 InternalServerError(500)
	msg := xcode.InternalServerError.String()

	// 如果该错误码不需要向用户展示，则展示默认
	// 否则，展示错误码的内容
	if !r.isErrorCodeVisible(err.ErrorCode()) {
		return msg
	}

	msg = err.Message()
	// 未定义语言key，或未定义消息处理器，则返回原始消息
	if r.msgHandler == nil {
		return msg
	}

	// 获取访问语言，并处理对应语言
	lang := r.detectLanguage()
	if lang == language.Und {
		return msg
	}

	str := r.msgHandler(err.ErrorCode(), lang)
	if len(str) > 0 {
		return str
	}

	return msg
}

func (r *response) detectLanguage() language.Tag {
	if len(r.supportedLanguages) == 0 {
		return r.defaultedLanguage
	}

	var lang string
	if len(r.languageHeaderKey) != 0 {
		lang = r.ctx.GetHeader(r.languageHeaderKey)
	} else {
		lang = r.ctx.GetHeader("Accept-Language")
	}
	if lang == "" {
		return r.defaultedLanguage
	}

	tags, _, err := language.ParseAcceptLanguage(lang)
	if err != nil || len(tags) == 0 {
		return r.defaultedLanguage
	}

	// 匹配支持的语言
	matcher := language.NewMatcher(r.supportedLanguages)
	tag, _, _ := matcher.Match(tags...)
	return tag
}

// WithErrorData 响应错误消息(HttpStatus!=200)，并在 header 中返回错误数据
func (r *response) WithErrorData(err error, data any) {
	bs, err1 := json.Marshal(data)
	if err1 != nil {
		r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "error data marshal failed: " + err1.Error(),
		})
		return
	}

	r.ctx.Header(ErrorDataHeaderKey, string(bs))

	r.WithError(err)
}

// nilSafeSlice 确保 nil slice 序列化为 [] 而非 null
func nilSafeSlice(v any) any {
	if v == nil {
		return []any{}
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		return []any{}
	}
	return v
}
