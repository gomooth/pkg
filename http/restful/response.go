package restful

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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

	"github.com/save95/xerror"
	"github.com/save95/xerror/xcode"
	"github.com/save95/xlog"

	"golang.org/x/text/language"
)

type response struct {
	ctx    *gin.Context
	logger xlog.XLog

	languageHeaderKey  string
	supportedLanguages []language.Tag
	msgHandler         func(code int, lang language.Tag) string
	defaultedLanguage  language.Tag

	showErrorCodes []int
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
		ctx:            ctx,
		showErrorCodes: make([]int, 0),
	}

	for _, opt := range opts {
		opt(resp)
	}

	if resp.logger == nil {
		resp.logger = logger.NewConsoleLogger()
	}

	return resp
}

// SetHeader 设置请求头
func (r *response) SetHeader(key, value string) IResponse {
	// 必须使用自定义头 X- 开始才设置，否则跳过
	if !strings.HasPrefix(key, "X-") && !strings.HasPrefix(key, "x-") {
		return r
	}

	r.ctx.Header(key, value)
	return r
}

// Retrieve 查询单个资源的响应
func (r *response) Retrieve(entity interface{}) {
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

	rows := make(map[string]map[string]interface{}, 0)
	for _, item := range resp.Items {
		row, ok := rows[item.RowKey]
		if !ok {
			row = make(map[string]interface{}, 0)
		}

		row[item.Column] = item.Data
		rows[item.RowKey] = row
	}

	extends := make(map[string]interface{}, 0)
	for _, item := range resp.Extends {
		if _, ok := extends[item.RowKey]; !ok {
			extends[item.RowKey] = item.Data
		}
	}

	//r.ctx.Header("Content-MD5", "")

	r.ctx.AbortWithStatusJSON(http.StatusOK, map[string]interface{}{
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
	if nil != err {
		r.WithError(xerror.Wrap(err, "parse uri failed"))
		return
	}

	qs := urls.Query()
	start := valutil.Int(qs.Get("start"))
	limit := valutil.IntWith(qs.Get("limit"), pager.DefaultPageSize)

	// 计算分页信息
	page := uint(math.Ceil(float64(start/limit)) + 1)
	count := uint(math.Max(1, float64(totalRow/uint(limit))))

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

	prevStart := int(math.Max(0, float64(start-limit)))
	prevUri := r.ComputePaginateUri(urls, prevStart)

	nextStart := int(math.Min(float64((count-1)*uint(limit)), float64(page*uint(limit))))
	nextUri := r.ComputePaginateUri(urls, nextStart)

	lastStart := int(math.Max(0, float64((count-1)*uint(limit))))
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
func (r *response) ListWithPagination(totalRow uint, entities interface{}) {
	tk := reflect.TypeOf(entities).Kind()
	if tk != reflect.Slice && tk != reflect.Array {
		r.WithError(xerror.New("response data type error"))
		return
	}

	// 写响应页码
	r.writeResponsePagination(totalRow)

	if reflect.ValueOf(entities).IsNil() {
		entities = make([]interface{}, 0)
	}
	r.ctx.AbortWithStatusJSON(http.StatusOK, entities)
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
func (r *response) ListWithMoreFlag(hasMore bool, entities interface{}) {
	tk := reflect.TypeOf(entities).Kind()
	if tk != reflect.Slice && tk != reflect.Array {
		r.WithError(xerror.New("response data type error"))
		return
	}

	r.ctx.Header(HasMoreHeaderKey, strconv.FormatBool(hasMore))

	if reflect.ValueOf(entities).IsNil() {
		entities = make([]interface{}, 0)
	}
	r.ctx.AbortWithStatusJSON(http.StatusOK, entities)
}

// Post 新增请求的响应
func (r *response) Post(entity interface{}) {
	if nil == entity {
		r.WithError(xerror.New("post must has response entity"))
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusCreated, entity)
}

// Put 全量更新资源的响应
func (r *response) Put(entity interface{}) {
	if nil == entity {
		r.WithError(xerror.New("put must has response entity"))
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusCreated, entity)
}

// Patch 部分更新资源的响应
// 部分 cdn 服务商不支持 http patch 方法，如 阿里云
func (r *response) Patch(entity interface{}) {
	if nil == entity {
		r.ctx.AbortWithStatus(http.StatusNoContent)
		return
	}

	r.ctx.AbortWithStatusJSON(http.StatusCreated, entity)
}

// Delete 删除的响应
func (r *response) Delete(err error) {
	if nil != err {
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
	if nil == err {
		r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "error not defined",
		})
		return
	}

	rq := r.ctx.Request
	if stx, se := httpcontext.MustParse(r.ctx); nil == se {
		bs := stx.Value(httpcontext.RequestRawBodyDataKey).([]byte)
		rq.Body = io.NopCloser(bytes.NewBuffer(bs))
	}

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

	r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
		"message": err.Error(),
	})
}

func (r *response) inShowErrorCodes(code int) bool {
	if len(r.showErrorCodes) == 0 {
		return true
	}

	for _, c := range r.showErrorCodes {
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
	if !r.inShowErrorCodes(err.ErrorCode()) {
		return msg
	}

	msg = err.String()
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
func (r *response) WithErrorData(err error, data interface{}) {
	bs, err1 := json.Marshal(data)
	if nil != err1 {
		r.ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "error data marshal failed: " + err1.Error(),
		})
		return
	}

	r.ctx.Header(ErrorDataHeaderKey, string(bs))

	r.WithError(err)
}
