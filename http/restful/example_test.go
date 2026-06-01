package restful_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/restful"
)

func ExampleNewResponse() {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/users/1", nil)

	resp := restful.NewResponse(c)
	resp.Retrieve(map[string]string{"name": "test"})
	fmt.Println(w.Code)
	// Output: 200
}
