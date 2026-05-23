package storage

func Disk(dir string, opts ...func(*Option)) IPrivateStorage {
	if dir == "tmp" || dir == "temp" {
		return Temp(opts...)
	}

	return newStorage(dir, opts...)
}

func Temp(opts ...func(*Option)) IPrivateStorage {
	return newTempStorage(opts...)
}
