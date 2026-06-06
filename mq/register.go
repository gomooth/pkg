package mq

import "github.com/gomooth/pkg/mq/internal/types"

// RegisterOption re-export
type RegisterOption = types.RegisterOption
type RegisterConfig = types.RegisterConfig
type QueueOption = types.QueueOption
type QueueConfig = types.QueueConfig

var WithGroup = types.WithGroup
var WithExtraTopics = types.WithExtraTopics
var WithQueueOptions = types.WithQueueOptions
var WithQueueClient = types.WithQueueClient
var WithQueueMaxRetry = types.WithQueueMaxRetry
var WithQueueBackoff = types.WithQueueBackoff
var WithQueueRetryMode = types.WithQueueRetryMode
var WithQueueFailedHandler = types.WithQueueFailedHandler
var ApplyRegisterOptions = types.ApplyRegisterOptions
