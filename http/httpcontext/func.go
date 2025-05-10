package httpcontext

import (
	"crypto/md5"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"
)

// NOTE 也可以使用三方UUID
func makeTraceID() string {
	h := md5.New()
	h.Write([]byte("trace-id"))
	h.Write([]byte(strconv.FormatInt(rand.Int64(), 10)))
	h.Write([]byte("-"))
	h.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	h.Write([]byte("-"))
	h.Write([]byte(strconv.FormatInt(int64(rand.Int32()), 10)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func makeSpanID() string {
	h := md5.New()
	h.Write([]byte("span-id"))
	h.Write([]byte(strconv.FormatInt(rand.Int64(), 10)))
	h.Write([]byte("-"))
	h.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
