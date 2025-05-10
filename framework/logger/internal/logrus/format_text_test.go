package logrus

import (
	"fmt"
	"testing"
	"time"

	ologrus "github.com/sirupsen/logrus"
)

func TestFormat_Format(t *testing.T) {
	type a struct {
		s string
	}

	entry := &ologrus.Entry{
		Logger: nil,
		Data: ologrus.Fields{
			"a":  "123",
			"b":  123,
			"c":  false,
			"p":  &a{s: "point"},
			"p1": a{s: "point"},
		},
		Time:    time.Now(),
		Level:   ologrus.InfoLevel,
		Caller:  nil,
		Message: "test entry",
		Buffer:  nil,
		Context: nil,
	}

	f := &formatText{}
	b, e := f.Format(entry)
	if nil != e {
		t.Error(e)
	}

	fmt.Print(string(b))
}
