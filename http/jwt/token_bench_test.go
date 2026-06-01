package jwt

import (
	"context"
	"testing"

	"github.com/gomooth/pkg/http/httpcontext"
)

// ============================================================
// 辅助
// ============================================================

var benchSecret = []byte("benchmark-secret-key-32bytes-long!")

type benchRole int

func (benchRole) String() string { return "admin" }

func benchUser() httpcontext.User {
	return httpcontext.User{
		ID:      1,
		Account: "benchuser",
		Name:    "Benchmark User",
		Roles:   []httpcontext.IRole{benchRole(1)},
		Extend:  map[string]string{"dept": "engineering"},
	}
}

// ============================================================
// P1: Token 生成（ToString）— 每次鉴权请求生成 token
// ============================================================

func BenchmarkToken_ToString(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tk, _ := NewToken(benchSecret, user)
		_, _ = tk.ToString(context.Background())
	}
}

// ============================================================
// P1: Token 生成 + 配置链 — 含 SetIssuer / SetDuration
// ============================================================

func BenchmarkToken_ToString_WithConfig(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tk, _ := NewToken(benchSecret, user)
		tk.SetIssuer("bench-app").SetDuration(3600e9) // 1h
		_, _ = tk.ToString(context.Background())
	}
}

// ============================================================
// P1: Token 解析 — 每次鉴权请求解析 token
// ============================================================

func BenchmarkParseToken(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()
	tk, _ := NewToken(benchSecret, user)
	tokenStr, _ := tk.ToString(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseToken(tokenStr, benchSecret, nil, 0, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================
// P1: Token 解析 + GetUser — 完整鉴权路径
// ============================================================

func BenchmarkParseToken_GetUser(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()
	tk, _ := NewToken(benchSecret, user)
	tokenStr, _ := tk.ToString(context.Background())

	toRole := func(s string) (httpcontext.IRole, error) { return benchRole(1), nil }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := parseToken(tokenStr, benchSecret, nil, 0, nil)
		if err != nil {
			b.Fatal(err)
		}
		parsedTk := newTokenWith(c)
		parsedTk.secret = benchSecret
		_, _ = parsedTk.GetUser(toRole)
	}
}

// ============================================================
// P1: NewToken 构造开销（不调用 ToString）
// ============================================================

func BenchmarkNewToken(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewToken(benchSecret, user)
	}
}

// ============================================================
// P1: IsExpired — 过期检查
// ============================================================

func BenchmarkToken_IsExpired(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()
	tk, _ := NewToken(benchSecret, user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tk.IsExpired()
	}
}

// ============================================================
// P1: Refresh — Token 刷新
// ============================================================

func BenchmarkToken_Refresh(b *testing.B) {
	b.ReportAllocs()

	user := benchUser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tk, _ := NewToken(benchSecret, user)
		tk.Refresh()
	}
}

// ============================================================
// P1: DefaultHashFunc — SHA256 哈希
// ============================================================

func BenchmarkDefaultHashFunc(b *testing.B) {
	b.ReportAllocs()

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultHashFunc(token)
	}
}

// ============================================================
// P1: GenerateSecret — 密钥生成
// ============================================================

func BenchmarkGenerateSecret(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSecret(32)
	}
}

// ============================================================
// P1: ParseSorts — 多字段排序解析
// ============================================================

// This goes into the pager benchmark file, not here
