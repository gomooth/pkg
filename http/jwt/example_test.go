package jwt_test

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/pkg/http/jwt"
)

func ExampleNewTokenBuilder() {
	secret := []byte("test-secret-key-1234567890")
	user := httpcontext.User{
		ID:      1,
		Account: "admin",
		Name:    "管理员",
		Roles:   []httpcontext.IRole{},
	}
	tok, err := jwt.NewTokenBuilder(secret, user).
		WithExpiration(2 * time.Hour).
		Build()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	str, _ := tok.ToString(context.Background())
	fmt.Println(len(str) > 0)
	// Output: true
}

func ExampleGenerateSecret() {
	secret := jwt.GenerateSecret(32)
	fmt.Println(len(secret) > 0)
	// Output: true
}
