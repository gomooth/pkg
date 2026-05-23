package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gomooth/xerror"

	"github.com/gin-gonic/gin"
)

type handler struct {
	ctx        *gin.Context
	respWriter *responseWriter

	retractive      string
	redactEnabled   bool
	sensitiveFields []string
	sensitiveKeys   map[string]bool // 预计算的查找集合
}

// defaultSensitiveFields 默认敏感字段名
var defaultSensitiveFields = []string{
	"password", "newPassword", "oldPassword", "confirmedPassword",
	"pwd", "newPwd", "oldPwd", "confirmedPwd",
	"secret", "token", "accessToken", "refreshToken",
	"apiKey", "privateKey", "credential",
}

func New(ctx *gin.Context, redactEnabled bool, sensitiveFields []string) ILogger {
	if redactEnabled && len(sensitiveFields) == 0 {
		sensitiveFields = defaultSensitiveFields
	}
	sensitiveKeys := make(map[string]bool, len(sensitiveFields))
	for _, f := range sensitiveFields {
		sensitiveKeys[strings.ToLower(f)] = true
	}

	hl := &handler{
		ctx:             ctx,
		respWriter:      newResponseWriter(ctx.Writer),
		retractive:      "   ",
		redactEnabled:   redactEnabled,
		sensitiveFields: sensitiveFields,
		sensitiveKeys:   sensitiveKeys,
	}

	ctx.Writer = hl.respWriter

	return hl
}

func (f handler) String() string {
	return fmt.Sprintf(
		"api: %s%s%s%s",
		f.general(),
		f.request(),
		f.response(),
		f.error(),
	)
}

func (f handler) general() string {
	var bf bytes.Buffer
	bf.WriteString("\n[")
	bf.WriteString(f.ctx.Request.Method)
	bf.WriteString("] ")

	uri := f.ctx.Request.RequestURI
	if uri == "" {
		uri = f.ctx.Request.URL.Path
	}
	bf.WriteString(uri)

	return bf.String()
}

func (f handler) request() string {
	var bs bytes.Buffer
	bs.WriteString("\n\n[Request] ")
	bs.WriteString(f.printHeader(f.ctx.Request.Header))
	bs.WriteString(f.printRequestPayload())

	return bs.String()
}

// sensitiveHeaders 需要脱敏的 header 字段名（小写）
var sensitiveHeaders = map[string]bool{
	"authorization":    true,
	"cookie":           true,
	"set-cookie":       true,
	"www-authenticate": true,
}

func (f handler) printHeader(headers http.Header) string {
	var bf bytes.Buffer
	bf.WriteString("\n [HEADER] ")

	for key, val := range headers {
		bf.WriteByte('\n')
		bf.WriteString(f.retractive)
		bf.WriteString(key)
		bf.WriteString(": ")
		valStr := strings.Join(val, ", ")
		if f.redactEnabled && sensitiveHeaders[strings.ToLower(key)] {
			valStr = f.redactValue(valStr)
		}
		bf.WriteString(valStr)
	}

	return bf.String()
}

// redactValue 对敏感值脱敏：保留前4个字符，其余替换为 ***
func (f handler) redactValue(val string) string {
	if len(val) <= 4 {
		return "****"
	}
	return val[:4] + "****"
}

func (f handler) printRequestPayload() string {
	var bf bytes.Buffer

	// 读取 request body 失败，则在日志中显示
	bs, err := io.ReadAll(f.ctx.Request.Body)
	if err != nil {
		bf.WriteString("\n [PAYLOAD] ")
		bf.WriteByte('\n')
		bf.WriteString(f.retractive)
		bf.WriteString("<read body failed: ")
		bf.WriteString(err.Error())
		bf.WriteString(">")

		return bf.String()
	}

	// 如果是 GET 请求，没有 payload 则不显示
	if f.ctx.Request.Method == http.MethodGet && len(bs) == 0 {
		return ""
	}

	bf.WriteString("\n [PAYLOAD] ")

	if len(bs) == 0 {
		bf.WriteByte('\n')
		bf.WriteString(f.retractive)
		bf.WriteString("<nil>")
		return bf.String()
	}

	bf.WriteByte('\n')
	// 通过 header 判断是否为文件上传，
	// 如果是文件，不打印文件内容，仅使用占位符表示
	ct := f.ctx.Request.Header.Get("Content-Type")
	if strings.Contains(ct, "boundary=") {
		boundary := strings.Split(ct, "boundary=")[1]
		reg := regexp.MustCompile(fmt.Sprintf("(%s\r\n.*?filename=[\\s\\S]*?\r\n\r\n)([\\s\\S]*?)(\r\n--%s)", boundary, boundary))
		nstr := reg.ReplaceAllString(string(bs), "$1>>>> FILE DATA <<<<$3")

		bss := strings.Split(nstr, "\r\n")
		for _, s := range bss {
			bf.WriteString(f.retractive)
			bf.WriteString(s)
			bf.WriteString("\r\n")
		}
	} else {
		bf.WriteString(f.retractive)
		bf.Write(f.redactBody(bs, ct))
	}

	return bf.String()
}

// redactBody 对请求体中的敏感字段脱敏，支持 JSON 和 form-data
func (f handler) redactBody(body []byte, contentType string) []byte {
	if !f.redactEnabled || len(f.sensitiveKeys) == 0 {
		return body
	}

	if strings.Contains(contentType, "application/json") {
		return f.redactJSON(body)
	}
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		return f.redactFormData(body)
	}
	return body
}

// redactJSON 对 JSON body 中的敏感字段值替换为 "***"
func (f handler) redactJSON(body []byte) []byte {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return body // 非 JSON 格式，原样返回
	}
	f.redactJSONValue(data)
	redacted, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return redacted
}

// redactJSONValue 递归遍历 JSON 数据，对敏感字段的值替换为 "***"
func (f handler) redactJSONValue(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	for key, val := range m {
		if f.sensitiveKeys[strings.ToLower(key)] {
			m[key] = "***"
			continue
		}
		switch child := val.(type) {
		case map[string]any:
			f.redactJSONValue(child)
		case []any:
			for _, item := range child {
				if subMap, ok := item.(map[string]any); ok {
					f.redactJSONValue(subMap)
				}
			}
		}
	}
}

// redactFormData 对 form-data 中的敏感字段值替换为 "***"
func (f handler) redactFormData(body []byte) []byte {
	parsed, err := url.ParseQuery(string(body))
	if err != nil {
		return body
	}
	for key := range parsed {
		if f.sensitiveKeys[strings.ToLower(key)] {
			parsed.Set(key, "***")
		}
	}
	return []byte(parsed.Encode())
}

func (f handler) response() string {
	var bs bytes.Buffer
	bs.WriteString("\n\n[Response] ")
	bs.WriteString("\n [STATUS] ")
	bs.WriteString(strconv.Itoa(f.respWriter.Status()))
	bs.WriteString(f.printHeader(f.respWriter.Header()))

	bs.WriteString("\n [BODY] ")
	bs.WriteByte('\n')
	bs.WriteString(f.retractive)

	body := f.respWriter.body
	if len(body.String()) == 0 {
		bs.WriteString("<nil>")
	} else {
		ct := f.respWriter.Header().Get("Content-Type")
		bs.Write(f.redactBody(body.Bytes(), ct))
	}

	return bs.String()
}

func (f handler) error() string {
	errs := f.ctx.Errors.ByType(gin.ErrorTypeAny)
	if len(errs) == 0 {
		return ""
	}

	//err := errors[0].Err
	//if err.IsType(gin.ErrorTypePrivate) {
	//	err = err.Err
	//}

	return f.printError(errs[0].Err)
}

func (f handler) printError(err error) string {
	if err == nil {
		return ""
	}

	var bs bytes.Buffer
	bs.WriteString("\n\n[Error] \n")
	bs.WriteString(f.retractive)
	bs.WriteString(err.Error())

	// 如果是 xerror，展示 xfield 内容
	if xf, ok := err.(xerror.FieldCarrier); ok {
		fields := xf.GetFields()
		if fields != nil && len(fields) > 0 {
			bs.WriteByte('\n')
			bs.WriteString(" [FIELDS] \n")
			bs.WriteString(f.retractive)

			//jsonIndentStr := f.retractive + f.retractive + f.retractive
			//xfbs, _ := json.MarshalIndent(fields, "", jsonIndentStr)
			xfbs, _ := json.Marshal(fields)
			bs.WriteString(string(xfbs))
		}
	}

	bs.WriteByte('\n')
	bs.WriteString(" [STACK] \n")

	//stack := fmt.Sprintf("%s%+v", f.retractive, err)
	var xe xerror.XError
	if errors.As(err, &xe) {
		err = xe.Unwrap()
	}
	stack := strings.ReplaceAll(fmt.Sprintf("%s%+v", f.retractive, err), "\n", "\n"+f.retractive)
	bs.WriteString(stack)

	return bs.String()
}
