package httpsqs

import (
	"context"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/mq/internal/consume"
)

// httpsqsFetcher 实现 consume.Fetcher 接口，从 HTTPSQS 队列拉取消息
type httpsqsFetcher struct {
	client    httpsqs.IClient
	queueName string
}

func newHttpsqsFetcher(client httpsqs.IClient, queueName string) *httpsqsFetcher {
	return &httpsqsFetcher{client: client, queueName: queueName}
}

// Fetch 从 HTTPSQS 队列拉取一条消息
func (f *httpsqsFetcher) Fetch(ctx context.Context) consume.FetchResult {
	data, pos, err := f.client.Get(ctx, f.queueName)
	if err != nil {
		if ctx.Err() != nil {
			return consume.FetchResult{Empty: true}
		}
		return consume.FetchResult{Err: err}
	}
	if pos == -1 {
		return consume.FetchResult{Empty: true}
	}
	return consume.FetchResult{Data: data}
}