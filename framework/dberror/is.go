package dberror

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

func IsDuplicateEntry(err error) bool {
	//if sqliteErr, ok := err.(sqlite3.Error); ok {
	//	return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
	//		sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	//} else
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == errorCodeMySQLDuplicateEntry ||
			mysqlErr.Number == errorCodeMySQLDuplicateEntryWithKeyName
	}

	return false
}
