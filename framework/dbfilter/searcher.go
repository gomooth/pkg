package dbfilter

import (
	"encoding/json"

	"github.com/gomooth/pkg/framework/pager"
)

type IFilter[F any] interface {
	pager.IPager[F]

	Preloads() []string

	//Build(db *gorm.DB, opts ...func(*buildOption)) *gorm.DB
}

type dbFilter[F any] struct {
	filter   *F
	sorters  []pager.Sorter
	preloads []string
}

func New[F any](filter F, opts ...func(*option)) IFilter[F] {
	pOpt := new(option)
	for _, opt := range opts {
		opt(pOpt)
	}

	return &dbFilter[F]{
		filter:   &filter,
		sorters:  pOpt.sorters,
		preloads: pOpt.preloads,
	}
}

func (s *dbFilter[F]) Filter() *F {
	if s.filter == nil {
		s.filter = new(F)
	}
	return s.filter
}

func (s *dbFilter[F]) Sorters() []pager.Sorter {
	if s.sorters == nil {
		return []pager.Sorter{}
	}

	return s.sorters
}

func (s *dbFilter[F]) Preloads() []string {
	if s.preloads == nil {
		return []string{}
	}

	return s.preloads
}

func (s *dbFilter[F]) String() string {
	data := map[string]interface{}{
		"filter":   s.filter,
		"sorters":  s.sorters,
		"preloads": s.preloads,
	}
	bs, _ := json.Marshal(data)
	return string(bs)
}
