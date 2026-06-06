package job

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/gomooth/pkg/job"

var (
	jobRunCounter   metric.Int64Counter
	jobRetryCounter metric.Int64Counter
	jobRunDuration  metric.Float64Histogram
)

func init() {
	telemetry.OnProviderSet(func() {
		m := telemetry.Meter(meterName)
		jobRunCounter, _ = m.Int64Counter("job.run.total", metric.WithDescription("Total job executions"))
		jobRetryCounter, _ = m.Int64Counter("job.run.retry", metric.WithDescription("Job retry attempts"))
		jobRunDuration, _ = m.Float64Histogram("job.run.duration",
			metric.WithDescription("Job execution duration"),
			metric.WithUnit("s"))
	})
}

func recordJobRun(ctx context.Context, jobName, result string, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("job_name", jobName),
		attribute.String("result", result),
	)
	jobRunCounter.Add(ctx, 1, attrs)
	jobRunDuration.Record(ctx, duration.Seconds(), attrs)
}

func recordJobRetry(ctx context.Context, jobName string) {
	jobRetryCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("job_name", jobName),
	))
}
