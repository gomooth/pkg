package logrus

import (
	"fmt"

	ologrus "github.com/sirupsen/logrus"
)

type stdoutHook struct {
	formatter ologrus.Formatter
}

func newStdoutHook(f ologrus.Formatter) ologrus.Hook {
	return &stdoutHook{formatter: f}
}

func (sh *stdoutHook) Levels() []ologrus.Level {
	return ologrus.AllLevels
}

func (sh *stdoutHook) Fire(entry *ologrus.Entry) error {
	bs, _ := sh.formatter.Format(entry)
	fmt.Println(string(bs))

	return nil
}
