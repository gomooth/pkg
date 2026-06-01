package job

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/gomooth/pkg/job"

var jobMeter = telemetry.Meter(meterName)

var (
	jobRunCounter, _  = jobMeter.Int64Counter("job.run.total", metric.WithDescription("Total job executions"))
	jobRetryCounter, _ = jobMeter.Int64Counter("job.run.retry", metric.WithDescription("Job retry attempts"))
	jobRunDuration, _  = jobMeter.Float64Histogram("job.run.duration",
		metric.WithDescription("Job execution duration"),
		metric.WithUnit("s"))
)

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
