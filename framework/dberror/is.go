package dberror

import (
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
)

// IsDuplicateEntry 判断是否为唯一约束冲突（重复条目）错误，支持 MySQL、PostgreSQL 和 SQLite
func IsDuplicateEntry(err error) bool {
	// MySQL
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == errorCodeMySQLDuplicateEntry ||
			mysqlErr.Number == errorCodeMySQLDuplicateEntryWithKeyName
	}

	// PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == errorCodePostgresUniqueViolation
	}

	// SQLite
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == errorCodeSQLiteConstraintUnique ||
			sqliteErr.ExtendedCode == errorCodeSQLiteConstraintPK
	}

	return false
}

// IsForeignKeyViolation 判断是否为外键约束违反错误
func IsForeignKeyViolation(err error) bool {
	// MySQL
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == errorCodeMySQLForeignKeyViolation
	}

	// PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == errorCodePostgresForeignKeyViolation
	}

	return false
}

// IsNotNullViolation 判断是否为非空约束违反错误
func IsNotNullViolation(err error) bool {
	// MySQL
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == errorCodeMySQLNotNullViolation
	}

	// PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == errorCodePostgresNotNullViolation
	}

	return false
}
