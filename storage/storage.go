package storage

import (
	"os"
	"path"
)

type storage struct {
	root []string

	dirs []string
	name string
	err  error
}

func newStorage(dir string, opts ...func(*Option)) *storage {
	root := storageRoot
	if len(opts) > 0 {
		o := &Option{}
		for _, opt := range opts {
			opt(o)
		}
		if o.root != "" {
			root = o.root
		}
	}

	return &storage{
		root: []string{root, dir},
		dirs: make([]string, 0),
	}
}

func newTempStorage(opts ...func(*Option)) *storage {
	return &storage{
		root: []string{os.TempDir(), "go-pkg"},
		dirs: make([]string, 0),
	}
}

// AppendDir 追加存储目录（链式调用）
func (p *storage) AppendDir(dirs ...string) IPrivateStorage {
	if p.err != nil {
		return p
	}
	for _, d := range dirs {
		if err := sanitizePath(d); err != nil {
			p.err = err
			return p
		}
	}
	p.dirs = append(p.dirs, dirs...)

	return p
}

// SetName 设置文件名（链式调用）
func (p *storage) SetName(name string) IPrivateStorage {
	if p.err != nil {
		return p
	}
	if len(name) > 0 {
		if err := sanitizePath(name); err != nil {
			p.err = err
			return p
		}
		p.name = name
	}

	return p
}

func (p *storage) Dir() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	base := path.Join(p.root...)
	return secureJoin(base, p.dirs...)
}

func (p *storage) Filename() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.name, nil
}

func (p *storage) Path() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	base := path.Join(p.root...)
	segments := append(p.dirs, p.name)
	return secureJoin(base, segments...)
}
