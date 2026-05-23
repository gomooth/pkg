package dberror

const (
	errorCodeMySQLDuplicateEntry            = 1062
	errorCodeMySQLDuplicateEntryWithKeyName = 1586

	errorCodePostgresUniqueViolation = "23505"

	errorCodeSQLiteConstraintUnique = 2067
	errorCodeSQLiteConstraintPK     = 1555

	// 外键约束违反
	errorCodeMySQLForeignKeyViolation    = 1452
	errorCodePostgresForeignKeyViolation = "23503"

	// 非空约束违反
	errorCodeMySQLNotNullViolation    = 1048
	errorCodePostgresNotNullViolation = "23502"
)
