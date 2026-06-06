package storage

import (
	"os"
	"path"
)

// baseStorage 存储路径的公共基础结构，供 storage 和 public 嵌入
type baseStorage struct {
	root []string

	dirs []string
	name string
	err  error
}

// appendDir 追加存储目录，返回是否成功
func (b *baseStorage) appendDir(dirs ...string) error {
	if b.err != nil {
		return b.err
	}
	for _, d := range dirs {
		if err := sanitizePath(d); err != nil {
			b.err = err
			return err
		}
	}
	b.dirs = append(b.dirs, dirs...)
	return nil
}

// setName 设置文件名，返回是否成功
func (b *baseStorage) setName(name string) error {
	if b.err != nil {
		return b.err
	}
	if len(name) > 0 {
		if err := sanitizePath(name); err != nil {
			b.err = err
			return err
		}
		b.name = name
	}
	return nil
}

// Dir 返回目录路径
func (b *baseStorage) Dir() (string, error) {
	if b.err != nil {
		return "", b.err
	}
	base := path.Join(b.root...)
	return secureJoin(base, b.dirs...)
}

// Filename 返回文件名
func (b *baseStorage) Filename() (string, error) {
	if b.err != nil {
		return "", b.err
	}
	return b.name, nil
}

// Path 返回完整路径
func (b *baseStorage) Path() (string, error) {
	if b.err != nil {
		return "", b.err
	}
	base := path.Join(b.root...)
	segments := append(b.dirs, b.name)
	return secureJoin(base, segments...)
}

type storage struct {
	baseStorage
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
		baseStorage: baseStorage{
			root: []string{root, dir},
			dirs: make([]string, 0),
		},
	}
}

func newTempStorage(opts ...func(*Option)) *storage {
	return &storage{
		baseStorage: baseStorage{
			root: []string{os.TempDir(), "go-pkg"},
			dirs: make([]string, 0),
		},
	}
}

// AppendDir 追加存储目录（链式调用）
func (p *storage) AppendDir(dirs ...string) IPrivateStorage {
	_ = p.appendDir(dirs...)
	return p
}

// SetName 设置文件名（链式调用）
func (p *storage) SetName(name string) IPrivateStorage {
	_ = p.setName(name)
	return p
}
