package restful

import (
	"errors"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"

	"github.com/gomooth/pkg/http/httpcontext"

	"github.com/gin-gonic/gin"
)

type handler struct {
	ctx     *gin.Context
	version string
}

func New(ctx *gin.Context, version string) *handler {
	return &handler{
		ctx:     ctx,
		version: version,
	}
}

func (h handler) Handle() error {
	if err := h.parseAccept(); nil != err {
		return err
	}

	return nil
}

func (h handler) parseAccept() error {
	stx, err := httpcontext.MustParse(h.ctx)
	if nil != err {
		return err
	}

	// see: https://developer.github.com/v3/media/#request-specific-version
	// application/vnd.server[.version].param[+json]
	// eg: application/vnd.server.v1.raw+json
	accept := h.ctx.GetHeader("Accept")

	// 默认值
	if len(accept) == 0 || accept == "*/*" || strings.Contains(accept, "application/json") {
		stx.Set("version", h.version).
			Set("bodyProperty", "raw").
			StorageTo(h.ctx)
		return nil
	}

	// 解析自定义媒体类型
	re := regexp.MustCompile(`application/vnd\.server(\.(v\S+?))(\.(raw|text|html|full))?\+json`)
	params := re.FindStringSubmatch(accept)
	//fmt.Printf("accept: %+v\n  %+v\n", accept, params)
	if len(params) == 5 {
		v := params[2]
		if _, err := version.NewVersion(v); err != nil {
			return errors.New("not support api version")
		}

		bp := params[4]
		if bp != "raw" && bp != "text" && bp != "html" && bp != "full" {
			return errors.New("not support custom media type")
		}

		stx.Set("version", v).
			Set("bodyProperty", bp).
			StorageTo(h.ctx)
		return nil
	}

	return errors.New("not support custom media type")
}
