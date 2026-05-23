package dbcache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/eko/gocache/lib/v4/cache"
	libStore "github.com/eko/gocache/lib/v4/store"
	redisStore "github.com/eko/gocache/store/redis/v4"
	"github.com/gomooth/pkg/framework/dbquery"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func newRedisCacheManager(t *testing.T) (*cache.Cache[string], *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	assert.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cacheStore := redisStore.NewRedis(rdb, libStore.WithExpiration(5*time.Minute))
	mgr := cache.New[string](cacheStore)

	return mgr, mr
}

func TestRedis_First_MissThenHit(t *testing.T) {
	mgr, mr := newRedisCacheManager(t)
	defer mr.Close()

	c := New[testEntity, testFilter]("test", mgr)

	callCount := 0
	entity := &testEntity{ID: 1, Name: "hello"}

	// 第一次 miss
	result, err := c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, entity.ID, result.ID)
	assert.Equal(t, 1, callCount)

	// 第二次 hit
	result, err = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, entity.ID, result.ID)
	assert.Equal(t, 1, callCount, "should hit cache on second call")
}

func TestRedis_Remember_MissThenHit(t *testing.T) {
	mgr, mr := newRedisCacheManager(t)
	defer mr.Close()

	c := New[testEntity, testFilter]("test", mgr)

	callCount := 0
	data := []byte(`{"value":42}`)

	_, err := c.Remember(context.Background(), "rkey", func(_ context.Context) ([]byte, error) {
		callCount++
		return data, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	result, err := c.Remember(context.Background(), "rkey", func(_ context.Context) ([]byte, error) {
		callCount++
		return data, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, data, result)
	assert.Equal(t, 1, callCount, "should hit cache on second call")
}

func TestRedis_ClearWithAll_InvalidatesCache(t *testing.T) {
	mgr, mr := newRedisCacheManager(t)
	defer mr.Close()

	c := New[testEntity, testFilter]("test", mgr)

	entity1 := &testEntity{ID: 1, Name: "one"}
	entity2 := &testEntity{ID: 2, Name: "two"}

	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		return entity1, nil
	})
	_, _ = c.First(context.Background(), 2, func(_ context.Context) (*testEntity, error) {
		return entity2, nil
	})

	err := c.Clear(context.Background(), ClearWithAll(true))
	assert.NoError(t, err)

	callCount := 0
	_, _ = c.First(context.Background(), 1, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity1, nil
	})
	_, _ = c.First(context.Background(), 2, func(_ context.Context) (*testEntity, error) {
		callCount++
		return entity2, nil
	})
	assert.Equal(t, 2, callCount, "all cache entries should be invalidated")
}

func TestRedis_List_MissThenHit(t *testing.T) {
	mgr, mr := newRedisCacheManager(t)
	defer mr.Close()

	c := New[testEntity, testFilter]("test", mgr)

	records := []*testEntity{{ID: 1, Name: "foo"}}
	q := dbquery.NewQuery(testFilter{Name: "foo"})
	callCount := 0

	_, err := c.List(context.Background(), q, func(_ context.Context) ([]*testEntity, error) {
		callCount++
		return records, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	result, err := c.List(context.Background(), q, func(_ context.Context) ([]*testEntity, error) {
		callCount++
		return records, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "should hit cache on second call")
	assert.Equal(t, records[0].ID, result[0].ID)
}

func TestRedis_Paginate_MissThenHit(t *testing.T) {
	mgr, mr := newRedisCacheManager(t)
	defer mr.Close()

	c := New[testEntity, testFilter]("test", mgr)

	records := []*testEntity{{ID: 1, Name: "foo"}}
	total := uint(1)
	q := dbquery.NewQuery(testFilter{Name: "foo"}, dbquery.WithOffsetPage[testFilter](0, 10))
	callCount := 0

	_, _, err := c.Paginate(context.Background(), q, func(_ context.Context) ([]*testEntity, uint, error) {
		callCount++
		return records, total, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	result, count, err := c.Paginate(context.Background(), q, func(_ context.Context) ([]*testEntity, uint, error) {
		callCount++
		return records, total, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "should hit cache on second call")
	assert.Equal(t, total, count)
	assert.Equal(t, records[0].ID, result[0].ID)
}
