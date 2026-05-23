package types

import (
	"os"
	"path"
	"runtime"
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
