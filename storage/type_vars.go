package storage

var storageRoot = "storage"

// Option storage 配置选项
type Option struct {
	root string
}

// WithRoot 设置存储根目录
func WithRoot(root string) func(*Option) {
	return func(o *Option) { o.root = root }
}

// IStorage 文件存储基础接口，提供目录、路径和文件名查询
type IStorage interface {
	// Dir 获得文件存储的目录
	Dir() (string, error)
	// Path 获得文件存储的路径
	Path() (string, error)
	// Filename 获得文件名
	Filename() (string, error)
}

// IPrivateStorage 私有文件存储接口，支持追加目录和设置文件名
type IPrivateStorage interface {
	IStorage

	// AppendDir 追加存储目录（链式调用）
	AppendDir(dirs ...string) IPrivateStorage
	// SetName 设置文件名（链式调用）
	SetName(name string) IPrivateStorage
}

// IPublicStorage 公开文件存储接口，支持 URL 生成
type IPublicStorage interface {
	IStorage

	// AppendDir 追加存储目录（链式调用）
	AppendDir(dirs ...string) IPublicStorage
	// SetName 设置文件名（链式调用）
	SetName(name string) IPublicStorage

	// URL 获得文件的访问链接（不含 host）
	URL() (string, error)
	// URLWithHost 获得文件的访问链接（含 host）
	URLWithHost(host string) (string, error)
}
