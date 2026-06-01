package job_test

import (
	"fmt"

	"github.com/gomooth/pkg/job"
)

func ExampleNewCronJobWrapper() {
	wrapper := job.NewCronJobWrapper(
		job.WrapWithMaxRetry(3),
	)
	fmt.Println(wrapper != nil)
	// Output: true
}
