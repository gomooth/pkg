package dbcache

type clearOption struct {
	paginate, list, remember bool
	single, all              bool

	ids  []uint
	keys []string
	tags []string
}

func ClearWithID(id uint, others ...uint) func(*clearOption) {
	return func(o *clearOption) {
		vals := append([]uint{id}, others...)
		if len(o.ids) == 0 {
			o.ids = make([]uint, 0)
		}
		o.ids = append(o.ids, vals...)
		if !o.all {
			o.single = true
			o.paginate = true
			o.list = true
			o.remember = true
		}
	}
}

func ClearWithKey(key string, others ...string) func(*clearOption) {
	return func(o *clearOption) {
		vals := append([]string{key}, others...)
		if len(o.keys) == 0 {
			o.keys = make([]string, 0)
		}
		o.keys = append(o.keys, vals...)
		if !o.all {
			o.single = true
		}
	}
}

func ClearWithTags(tag string, others ...string) func(*clearOption) {
	return func(o *clearOption) {
		vals := append([]string{tag}, others...)
		if len(o.tags) == 0 {
			o.tags = make([]string, 0)
		}
		o.tags = append(o.tags, vals...)
		if !o.all {
			o.single = true
		}
	}
}

func ClearWithAll(all bool) func(*clearOption) {
	return func(o *clearOption) {
		if all {
			o.all = true
			o.single = false
		} else {
			o.all = false
			o.single = len(o.ids) > 0 || len(o.keys) > 0 || len(o.tags) > 0
		}
	}
}
