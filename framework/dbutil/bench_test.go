package dbutil

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
)

// ============================================================
// 辅助：构建基于 SQLite 内存的连接选项
// ============================================================

func newBenchOption(name string) *Option {
	return &Option{
		Name: name,
		Config: &ConnectConfig{
			Driver: "sqlite",
			Dsn:    ":memory:",
		},
	}
}

// ============================================================
// BenchmarkConnectWithContext_HotPath — 测量获取已缓存的数据库连接的延迟
// ============================================================

func BenchmarkConnectWithContext_HotPath(b *testing.B) {
	b.ReportAllocs()

	// 使用唯一名称避免与其他测试冲突
	opt := newBenchOption("bench-hot-path")

	// 先建立连接，让 dbHolder 进入 ready 状态
	_, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConnectWithContext(ctx, opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	}

	b.StopTimer()
	_ = Close("bench-hot-path")
}

// ============================================================
// BenchmarkConnectWithContext_ColdPath — 测量首次建立连接的开销
// ============================================================

func BenchmarkConnectWithContext_ColdPath(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := strings.Builder{}
		name.WriteString("bench-cold-")
		name.WriteString(strings.Repeat("x", 0))

		opt := &Option{
			Name: name.String(),
			Config: &ConnectConfig{
				Driver: "sqlite",
				Dsn:    ":memory:",
			},
		}
		_, _ = ConnectWithContext(ctx, opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
		_ = Close(name.String())
	}
}

// ============================================================
// BenchmarkConnectWithContext_ParallelHotPath — 并发获取已缓存连接
// ============================================================

func BenchmarkConnectWithContext_ParallelHotPath(b *testing.B) {
	b.ReportAllocs()

	opt := newBenchOption("bench-parallel-hot")

	// 先建立连接
	_, err := ConnectWithContext(context.Background(), opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ConnectWithContext(ctx, opt, ConnectWithGORMDialector(sqlite.Open(":memory:")))
		}
	})

	b.StopTimer()
	_ = Close("bench-parallel-hot")
}

// ============================================================
// BenchmarkConnectWithoutCache — 测量不使用缓存的连接建立开销
// ============================================================

func BenchmarkConnectWithoutCache(b *testing.B) {
	b.ReportAllocs()

	opt := newBenchOption("bench-without-cache")
	dialect := sqlite.Open(":memory:")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := connectWithoutCache(dialect, opt)
		if err != nil {
			b.Fatal(err)
		}
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	}
}
