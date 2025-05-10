package types

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/gomooth/pkg/framework/logger/internal/trace"
	"github.com/save95/xlog"
)

const (
	defaultDir            = "/var/log/gomooth" // 默认日志存储目录
	defaultFilenameFormat = "runtime.log"      // 默认日志文件规则
)

func GetDefaultDir() string {
	if runtime.GOOS == "linux" {
		return defaultDir
	}

	return path.Join(os.TempDir(), "logs", "go-pkg")
}

func GetDefaultFilenameFormat() string {
	return defaultFilenameFormat
}

func ParseTrace(ctx context.Context) string {
	traceID, spanID := trace.GetSpanInfo(ctx)
	traceContent := ""
	if traceID != "" {
		traceContent = traceID
	}
	if spanID != "" {
		traceContent += "-" + spanID
	}
	if len(traceContent) > 0 {
		traceContent = fmt.Sprintf("[%s]", traceContent)
	}

	return traceContent
}

func ParseField(fields xlog.Fields) string {
	content := ""
	if fields != nil {
		bs, _ := json.Marshal(fields)
		content = fmt.Sprintf("[FIELD: %s]", string(bs))
	}

	return content
}
