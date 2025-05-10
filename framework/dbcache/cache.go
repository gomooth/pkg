package dbcache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gomooth/pkg/framework/dbquery"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"

	"github.com/redis/go-redis/v9"

	"github.com/save95/xerror"

	"golang.org/x/sync/singleflight"
)

var single singleflight.Group

type dbCache[E, F any] struct {
	cacheManager *cache.Cache[string]
	name         string
	autoRenew    bool // 自动延长缓存有效期
	expiration   time.Duration
}

func New[E, F any](name string, cacheManager *cache.Cache[string], opts ...func(*option)) IDBCache[E, F] {
	dc := &dbCache[E, F]{
		name:         name,
		cacheManager: cacheManager,
		autoRenew:    true,
		expiration:   5 * time.Minute,
	}

	if len(opts) > 0 {
		cnf := &option{
			autoRenew:  true,
			expiration: 5 * time.Minute,
		}
		for _, opt := range opts {
			opt(cnf)
		}
		dc.autoRenew = cnf.autoRenew
		dc.expiration = cnf.expiration
	}

	return dc
}

type queryResult[E any] struct {
	Paginate struct {
		Data  []*E `json:"data"`
		Total uint `json:"total"`
	} `json:"paginate,omitempty"`

	First struct {
		Data *E `json:"data"`
	} `json:"first,omitempty"`

	Remember struct {
		Data any `json:"data"`
	} `json:"remember,omitempty"`
}

func (s *dbCache[E, F]) Paginate(ctx context.Context, start, limit int, opt dbquery.IFilter[F],
	query func() ([]*E, uint, error),
) ([]*E, uint, error) {
	k := strings.ToLower(fmt.Sprintf("%x", md5.Sum([]byte(opt.String()))))
	key := fmt.Sprintf("%s:paginate:%d,%d:%s", s.name, start, limit, k)
	tags := []string{s.tag("paginate")}

	cacheDataText, err := s.remember(ctx, key, tags, func() (*queryResult[E], error) {
		records, total, err := query()
		if nil != err {
			return nil, err
		}

		res := new(queryResult[E])
		res.Paginate.Data = records
		res.Paginate.Total = total
		return res, nil
	})
	if nil != err {
		return nil, 0, err
	}

	var result *queryResult[E]
	if err := json.Unmarshal([]byte(cacheDataText), &result); nil != err {
		return nil, 0, xerror.Wrap(err, "result unmarshal failed")
	}

	return result.Paginate.Data, result.Paginate.Total, nil
}

func (s *dbCache[E, F]) List(ctx context.Context, opt dbquery.IFilter[F],
	query func() ([]*E, error),
) ([]*E, error) {
	k := strings.ToLower(fmt.Sprintf("%x", md5.Sum([]byte(opt.String()))))
	key := fmt.Sprintf("%s:list:%s", s.name, k)
	tags := []string{s.tag("list")}

	cacheDataText, err := s.remember(ctx, key, tags, func() (*queryResult[E], error) {
		records, err := query()
		if nil != err {
			return nil, err
		}

		res := new(queryResult[E])
		res.Paginate.Data = records
		return res, nil
	})
	if nil != err {
		return nil, err
	}

	var result *queryResult[E]
	if err := json.Unmarshal([]byte(cacheDataText), &result); nil != err {
		return nil, xerror.Wrap(err, "result unmarshal failed")
	}

	return result.Paginate.Data, nil
}

func (s *dbCache[E, F]) First(ctx context.Context, id uint, query func() (*E, error)) (*E, error) {
	if id == 0 {
		return nil, xerror.New("id error")
	}

	tags := []string{s.tag(fmt.Sprintf("%d", id))}
	key := fmt.Sprintf("%s:first:%d", s.name, id)
	cacheDataText, err := s.remember(ctx, key, tags, func() (*queryResult[E], error) {
		record, err := query()
		if nil != err {
			return nil, err
		}
		result := new(queryResult[E])
		result.First.Data = record
		return result, nil
	})
	if nil != err {
		return nil, err
	}

	var result *queryResult[E]
	if err := json.Unmarshal([]byte(cacheDataText), &result); nil != err {
		return nil, xerror.Wrap(err, "result unmarshal failed")
	}

	return result.First.Data, nil
}

func (s *dbCache[E, F]) Clear(ctx context.Context, opts ...func(*clearOption)) error {
	cnf := new(clearOption)
	for _, opt := range opts {
		opt(cnf)
	}

	// 未配置，或显式指定所有，则清理所有缓存
	if !cnf.single || cnf.all {
		return s.cacheManager.Invalidate(ctx, store.WithInvalidateTags([]string{
			s.ownTag(),
		}))
	}

	tags := make([]string, 0)
	if len(cnf.ids) > 0 {
		for _, id := range cnf.ids {
			tags = append(tags, s.tag(fmt.Sprintf("%d", id)))
		}
	}
	if len(cnf.keys) > 0 {
		for _, key := range cnf.keys {
			tags = append(tags, s.tag(key))
		}
	}
	if len(cnf.tags) > 0 {
		tags = append(tags, cnf.tags...)
	}

	if cnf.paginate {
		tags = append(tags, s.tag("paginate"))
	}
	if cnf.list {
		tags = append(tags, s.tag("list"))
	}
	if cnf.remember {
		tags = append(tags, s.tag("remember"))
	}

	if len(tags) > 0 {
		return s.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags))
	}
	return nil
}

func (s *dbCache[E, F]) Remember(ctx context.Context, key string, query func() (any, error)) (any, error) {
	tags := []string{
		s.tag(key),
		s.tag("remember"),
		s.tag(fmt.Sprintf("remember:%s", key)),
	}
	key = fmt.Sprintf("%s:remember:%s", s.name, key)

	cacheDataText, err := s.remember(ctx, key, tags, func() (*queryResult[E], error) {
		record, err := query()
		if nil != err {
			return nil, err
		}
		result := new(queryResult[E])
		result.Remember.Data = record
		return result, nil
	})
	if nil != err {
		return nil, err
	}

	var result *queryResult[E]
	if err := json.Unmarshal([]byte(cacheDataText), &result); nil != err {
		return nil, xerror.Wrap(err, "data unmarshal failed")
	}

	return result.Remember.Data, nil
}

func (s *dbCache[E, F]) remember(ctx context.Context, key string, tags []string,
	fun func() (*queryResult[E], error),
) (string, error) {
	if s.cacheManager == nil {
		return "", xerror.New("cache manager no init")
	}

	cachedTags := append([]string{"dbcache", s.ownTag()}, tags...)
	cacheData, d, err := s.cacheManager.GetWithTTL(ctx, key)
	if nil == err {
		// 延长时效
		if s.autoRenew && d <= time.Minute {
			err = s.cacheManager.Set(
				ctx, key, cacheData,
				store.WithExpiration(s.expiration),
				store.WithTags(cachedTags),
			)
		}

		return cacheData, nil
	}

	if !errors.Is(err, redis.Nil) {
		return "", err
	}

	v, err, _ := single.Do(key, func() (interface{}, error) {
		record, err := fun()
		if nil != err {
			return nil, err
		}

		bs, err := json.Marshal(record)
		if nil != err {
			return nil, err
		}

		err = s.cacheManager.Set(
			ctx, key, string(bs),
			store.WithExpiration(s.expiration),
			store.WithTags(cachedTags),
		)
		if nil != err {
			return nil, err
		}

		return string(bs), nil
	})
	if nil != err {
		return "", xerror.Wrap(err, "cache store failed")
	}

	return v.(string), nil
}

func (s *dbCache[E, F]) ownTag() string {
	return fmt.Sprintf("dbcache:%s", s.name)
}

func (s *dbCache[E, F]) tag(tag string) string {
	return fmt.Sprintf("dbcache:%s:%s", s.name, tag)
}

func (s *dbCache[E, F]) Forget(ctx context.Context, key string) error {
	if s.cacheManager == nil {
		return xerror.New("cache manager no init")
	}

	return s.cacheManager.Invalidate(ctx, store.WithInvalidateTags([]string{key}))
}
