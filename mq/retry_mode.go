package mq

import "github.com/gomooth/pkg/mq/internal/types"

// RetryMode re-export
type RetryMode = types.RetryMode

const (
	RetryModeSync    = types.RetryModeSync
	RetryModeRequeue = types.RetryModeRequeue
)
