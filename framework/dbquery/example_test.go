package dbquery_test

import (
	"fmt"

	"github.com/gomooth/pkg/framework/dbquery"
)

func ExampleNewQuery() {
	type Filter struct{ Name string }
	q := dbquery.NewQuery(Filter{Name: "test"},
		dbquery.WithSorts[Filter]("-id"),
		dbquery.WithOffsetPage[Filter](0, 20),
	)
	fmt.Println(q.Filter().Name)
	// Output: test
}
