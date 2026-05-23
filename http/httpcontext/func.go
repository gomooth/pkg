package httpcontext

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gomooth/xerror"
)

func makeTraceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", xerror.Wrap(err, "httpcontext: failed to generate trace ID")
	}
	return hex.EncodeToString(b), nil
}

func makeSpanID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", xerror.Wrap(err, "httpcontext: failed to generate span ID")
	}
	return hex.EncodeToString(b), nil
}
