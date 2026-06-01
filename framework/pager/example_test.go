package pager_test

import (
	"fmt"

	"github.com/gomooth/pkg/framework/pager"
)

func ExampleParseSorts() {
	sorters := pager.ParseSorts("+name,-created_at")
	fmt.Println(len(sorters))
	// Output: 2
}
