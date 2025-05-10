package storage

const storageRoot = "storage"

type IStorage interface {
	// Dir 获得文件存储的目录
	Dir() string
	// Path 获得文件存储的路径
	Path() string
	// Filename 获得文件名
	Filename() string
}

type IPrivateStorage interface {
	IStorage

	// AppendDir 追加存储目录
	AppendDir(dirs ...string) IPrivateStorage
	// SetName 设置文件名
	SetName(name string) IPrivateStorage
}

type IPublicStorage interface {
	IStorage

	// AppendDir 追加存储目录
	AppendDir(dirs ...string) IPublicStorage
	// SetName 设置文件名
	SetName(name string) IPublicStorage

	// URL 获得文件的访问链接（不含 host）
	URL() string
	// URLWithHost 获得文件的访问链接（含 host）
	URLWithHost(host string) string
}
