package jwt_test

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/pkg/http/jwt"
)

func ExampleNewToken() {
	secret := []byte("test-secret-key-1234567890")
	user := httpcontext.User{
		ID:      1,
		Account: "admin",
		Name:    "管理员",
		Roles:   []httpcontext.IRole{},
	}
	tok, err := jwt.NewToken(secret, user)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	tok.SetDuration(2 * time.Hour)
	str, _ := tok.ToString(context.Background())
	fmt.Println(len(str) > 0)
	// Output: true
}

func ExampleGenerateSecret() {
	secret := jwt.GenerateSecret(32)
	fmt.Println(len(secret) > 0)
	// Output: true
}
