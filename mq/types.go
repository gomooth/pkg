package mq

import "github.com/gomooth/pkg/mq/internal/types"

// 统一接口 re-export
type IHandler = types.IHandler
type IConsumeServer = types.IConsumeServer
type IProducer = types.IProducer
type FailedHandlerFunc = types.FailedHandlerFunc
type DeadLetterHandler = types.DeadLetterHandler
type FuncHandler = types.FuncHandler
