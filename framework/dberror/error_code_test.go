package dberror

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestIsDuplicateEntry(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
		comment string
	}{
		{
			name:    "MySQL_DuplicateEntry_1062",
			err:     &mysql.MySQLError{Number: errorCodeMySQLDuplicateEntry},
			want:    true,
			comment: "MySQL duplicate entry error (1062)",
		},
		{
			name:    "MySQL_DuplicateEntryWithKeyName_1586",
			err:     &mysql.MySQLError{Number: errorCodeMySQLDuplicateEntryWithKeyName},
			want:    true,
			comment: "MySQL duplicate entry with key name error (1586)",
		},
		{
			name:    "MySQL_WrongCode_1045",
			err:     &mysql.MySQLError{Number: 1045},
			want:    false,
			comment: "MySQL wrong error code (1045)",
		},
		{
			name:    "PostgreSQL_UniqueViolation_23505",
			err:     &pgconn.PgError{Code: errorCodePostgresUniqueViolation},
			want:    true,
			comment: "PostgreSQL unique violation error (23505)",
		},
		{
			name:    "PostgreSQL_WrongCode_23502",
			err:     &pgconn.PgError{Code: "23502"},
			want:    false,
			comment: "PostgreSQL wrong error code (23502)",
		},
		{
			name:    "GenericError",
			err:     errors.New("some error"),
			want:    false,
			comment: "Non-database error",
		},
		{
			name:    "NilError",
			err:     nil,
			want:    false,
			comment: "Nil error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDuplicateEntry(tt.err)
			assert.Equal(t, tt.want, got, tt.comment)
		})
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
		comment string
	}{
		{
			name:    "MySQL_ForeignKeyViolation_1452",
			err:     &mysql.MySQLError{Number: errorCodeMySQLForeignKeyViolation},
			want:    true,
			comment: "MySQL foreign key violation error (1452)",
		},
		{
			name:    "MySQL_WrongCode_1062",
			err:     &mysql.MySQLError{Number: 1062},
			want:    false,
			comment: "MySQL wrong error code (1062)",
		},
		{
			name:    "PostgreSQL_ForeignKeyViolation_23503",
			err:     &pgconn.PgError{Code: errorCodePostgresForeignKeyViolation},
			want:    true,
			comment: "PostgreSQL foreign key violation error (23503)",
		},
		{
			name:    "PostgreSQL_WrongCode_23505",
			err:     &pgconn.PgError{Code: "23505"},
			want:    false,
			comment: "PostgreSQL wrong error code (23505)",
		},
		{
			name:    "SQLite_NotSupported",
			err:     &sqlite3.Error{Code: sqlite3.ErrNo(19), ExtendedCode: 2067},
			want:    false,
			comment: "SQLite does not support foreign key violation detection",
		},
		{
			name:    "GenericError",
			err:     errors.New("some error"),
			want:    false,
			comment: "Non-database error",
		},
		{
			name:    "NilError",
			err:     nil,
			want:    false,
			comment: "Nil error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsForeignKeyViolation(tt.err)
			assert.Equal(t, tt.want, got, tt.comment)
		})
	}
}

func TestIsNotNullViolation(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
		comment string
	}{
		{
			name:    "MySQL_NotNullViolation_1048",
			err:     &mysql.MySQLError{Number: errorCodeMySQLNotNullViolation},
			want:    true,
			comment: "MySQL not null violation error (1048)",
		},
		{
			name:    "MySQL_WrongCode_1062",
			err:     &mysql.MySQLError{Number: 1062},
			want:    false,
			comment: "MySQL wrong error code (1062)",
		},
		{
			name:    "PostgreSQL_NotNullViolation_23502",
			err:     &pgconn.PgError{Code: errorCodePostgresNotNullViolation},
			want:    true,
			comment: "PostgreSQL not null violation error (23502)",
		},
		{
			name:    "PostgreSQL_WrongCode_23505",
			err:     &pgconn.PgError{Code: "23505"},
			want:    false,
			comment: "PostgreSQL wrong error code (23505)",
		},
		{
			name:    "SQLite_NotSupported",
			err:     &sqlite3.Error{Code: sqlite3.ErrNo(19), ExtendedCode: 2067},
			want:    false,
			comment: "SQLite does not support not null violation detection",
		},
		{
			name:    "GenericError",
			err:     errors.New("some error"),
			want:    false,
			comment: "Non-database error",
		},
		{
			name:    "NilError",
			err:     nil,
			want:    false,
			comment: "Nil error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotNullViolation(tt.err)
			assert.Equal(t, tt.want, got, tt.comment)
		})
	}
}
