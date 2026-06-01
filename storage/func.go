package storage

// Disk 创建指定目录的私有存储实例，dir 为 "tmp" 或 "temp" 时自动使用临时目录
func Disk(dir string, opts ...func(*Option)) IPrivateStorage {
	if dir == "tmp" || dir == "temp" {
		return Temp(opts...)
	}

	return newStorage(dir, opts...)
}

// Temp 创建临时目录的私有存储实例，文件存储在系统临时目录的 go-pkg 子目录下
func Temp(opts ...func(*Option)) IPrivateStorage {
	return newTempStorage(opts...)
}
