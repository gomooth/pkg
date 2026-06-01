package storage

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
)

// sanitizePath 检查路径组件不含目录遍历或其他危险字符
func sanitizePath(component string) error {
	// 去除首尾空白
	component = strings.TrimSpace(component)
	if component == "" {
		return xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: empty path component after trimming whitespace")
	}

	// 拒绝 null 字节
	if strings.ContainsRune(component, 0) {
		return xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: null byte detected in path component: %q", component))
	}

	cleaned := filepath.Clean(component)

	// 清理后路径不得向上遍历
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) || strings.HasPrefix(cleaned, "../") {
		return xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: directory traversal detected in path component: %s", component))
	}

	// 检查清理后路径是否与原始路径归一化后一致（允许 ./ 前缀移除）
	normalizedOriginal := filepath.FromSlash(component)
	if cleaned != normalizedOriginal && cleaned != strings.TrimPrefix(normalizedOriginal, "."+string(os.PathSeparator)) {
		return xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: path component was altered during cleaning, possible encoding: %q -> %q", component, cleaned))
	}

	return nil
}

// secureJoin 安全拼接路径，确保结果不超出 base 目录
// 使用 filepath.EvalSymlinks 解析符号链接后的真实路径，防止 symlink 穿越攻击
func secureJoin(base string, segments ...string) (string, error) {
	joined := path.Join(base, path.Join(segments...))

	// 确保基础目录存在
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: failed to create base directory: %s", err))
	}

	// 解析 base 目录的真实路径（消除符号链接）
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: failed to resolve base directory: %s", err))
	}

	// 解析拼接路径的真实路径
	// 若路径不存在，向上找到存在的父目录并拼接剩余部分
	realJoined, err := evalSymlinksPath(joined)
	if err != nil {
		return "", xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: failed to resolve path: %s", err))
	}

	// 验证真实路径仍在 base 目录下
	rel, err := filepath.Rel(realBase, realJoined)
	if err != nil {
		return "", xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: failed to compute relative path")
	}

	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: path traversal detected: result escapes base directory")
	}

	return joined, nil
}

// evalSymlinksPath 解析路径中的符号链接。
// 对于不存在的路径，向上逐级找到存在的父目录后拼接剩余部分。
func evalSymlinksPath(p string) (string, error) {
	// 路径已存在，直接解析
	if _, err := os.Lstat(p); err == nil {
		return filepath.EvalSymlinks(p)
	}

	// 路径不存在，逐级向上查找已存在的父目录
	dir := filepath.Dir(p)
	base := filepath.Base(p)

	for {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		base = filepath.Join(filepath.Base(dir), base)
		dir = filepath.Dir(dir)
		if dir == "." || dir == "/" {
			// 到达根目录仍不存在，直接返回清理后的路径
			return filepath.Clean(p), nil
		}
	}

	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}

	return filepath.Join(realDir, base), nil
}

type public struct {
	root []string

	dirs []string
	name string
	err  error
}

func newPublic(opts ...func(*Option)) *public {
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

	return &public{
		root: []string{root, "public"},
		dirs: make([]string, 0),
	}
}

// Public 创建公开存储实例，文件存储在 {root}/public 目录下
func Public(opts ...func(*Option)) IPublicStorage {
	return newPublic(opts...)
}

// PublicFromFile 从文件名创建公开存储实例，自动解析目录和文件名
func PublicFromFile(filename string, opts ...func(*Option)) IPublicStorage {
	return newPublic(opts...).withFile(filename)
}

// PublicFromUrl 从 URL 创建公开存储实例，自动解析目录和文件名
func PublicFromUrl(fileURL string, opts ...func(*Option)) IPublicStorage {
	return newPublic(opts...).withURL(fileURL)
}

func (p *public) withFile(filename string) *public {
	if len(filename) == 0 {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: empty filename")
		return p
	}

	base := path.Join(p.root...)

	// 用 SplitN 按 base 分割，取 base 之后的部分（与 withURL 风格一致）
	parts := strings.SplitN(filename, base, 2)
	var relPart string
	if len(parts) == 2 {
		relPart = parts[1]
	} else {
		// filename 不包含 base，视为纯相对路径
		relPart = filename
	}

	relPart = strings.TrimLeft(relPart, string(os.PathSeparator))
	relPart = strings.TrimLeft(relPart, "/")
	if len(relPart) == 0 {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: no valid path component extracted from filename")
		return p
	}

	return p.setDirsAndName(relPart)
}

// setDirsAndName 清理相对路径、校验遍历安全后拆分为 dirs 和 name
func (p *public) setDirsAndName(relPart string) *public {
	cleaned := filepath.Clean(relPart)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) || strings.HasPrefix(cleaned, "../") {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, fmt.Sprintf("storage: directory traversal detected in path: %s", relPart))
		return p
	}

	pathParts := splitPathParts(filepath.ToSlash(cleaned))
	if len(pathParts) == 0 {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: no valid path components after cleaning")
		return p
	}

	p.dirs = pathParts[:len(pathParts)-1]
	p.name = pathParts[len(pathParts)-1]
	return p
}

// splitPathParts 将路径拆分为非空、非 "." 的组件
func splitPathParts(p string) []string {
	raw := strings.Split(p, "/")
	parts := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" && s != "." {
			parts = append(parts, s)
		}
	}
	return parts
}

func (p *public) withURL(fileURL string) *public {
	if len(fileURL) == 0 {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: empty file URL")
		return p
	}

	urls := strings.SplitN(fileURL, fmt.Sprintf("%s/", storageRoot), 2)
	if len(urls) != 2 {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: URL does not contain storage root path")
		return p
	}

	relPart := strings.TrimRight(urls[1], "/")
	if len(relPart) == 0 {
		p.err = xerror.NewXCode(xcode.ErrStoragePathInvalid, "storage: no valid path component extracted from URL")
		return p
	}

	return p.setDirsAndName(relPart)
}

// AppendDir 追加存储目录（链式调用）
func (p *public) AppendDir(dirs ...string) IPublicStorage {
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
func (p *public) SetName(name string) IPublicStorage {
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

func (p *public) Dir() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	base := path.Join(p.root...)
	return secureJoin(base, p.dirs...)
}

func (p *public) Filename() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.name, nil
}

func (p *public) Path() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	base := path.Join(p.root...)
	segments := append(p.dirs, p.name)
	return secureJoin(base, segments...)
}

func (p *public) URL() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	// root = [storageRoot, "public"]，URL 格式为 /{storageRoot}/{dirs}/{name}
	// 跳过 root 中的两个元素：storageRoot 由下方 Sprintf 单独添加，"public" 是内部目录不暴露到 URL
	paths := p.root[2:]
	paths = append(paths, p.dirs...)
	paths = append(paths, p.name)

	cleaned := filepath.Clean(path.Join(paths...))
	// 确保清理后的路径不包含任何 .. 遍历
	for strings.HasPrefix(cleaned, "../") {
		cleaned = strings.TrimPrefix(cleaned, "../")
	}
	if strings.Contains(cleaned, "/..") || cleaned == ".." {
		cleaned = ""
	}

	return fmt.Sprintf("/%s/%s", storageRoot, filepath.ToSlash(cleaned)), nil
}

func (p *public) URLWithHost(host string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	if !strings.HasPrefix(host, "http") {
		return p.URL()
	}

	url, err := p.URL()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s", host, url), nil
}
