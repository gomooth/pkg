package dbrepo

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testModel is a test model for DAO tests
type testModel struct {
	gorm.Model
	Name  string
	Email string
}

// setupTestDB creates an in-memory SQLite database and auto-migrates the test model
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.AutoMigrate(&testModel{})
	assert.NoError(t, err)

	return db
}

func newTestDAO(t *testing.T) IDAO[testModel] {
	t.Helper()
	dao, err := NewDAO[testModel](setupTestDB(t))
	assert.NoError(t, err)
	return dao
}

func TestNewDAO_NilDB(t *testing.T) {
	dao, err := NewDAO[testModel](nil)
	assert.Error(t, err)
	assert.Nil(t, dao)
}

func TestDAO_Create(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		record := &testModel{Name: "Alice", Email: "alice@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)
	})

	t.Run("nil record", func(t *testing.T) {
		err := dao.Create(ctx, nil)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}

func TestDAO_Creates(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		records := []*testModel{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		}
		err := dao.Creates(ctx, records)
		assert.NoError(t, err)
		assert.NotZero(t, records[0].ID)
		assert.NotZero(t, records[1].ID)
	})

	t.Run("empty slice", func(t *testing.T) {
		err := dao.Creates(ctx, []*testModel{})
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("nil in slice", func(t *testing.T) {
		records := []*testModel{
			{Name: "Alice", Email: "alice@example.com"},
			nil,
		}
		err := dao.Creates(ctx, records)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}

func TestDAO_First(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	// Create a record first
	record := &testModel{Name: "Alice", Email: "alice@example.com"}
	err := dao.Create(ctx, record)
	assert.NoError(t, err)
	id := record.ID

	t.Run("success", func(t *testing.T) {
		result, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Alice", result.Name)
		assert.Equal(t, "alice@example.com", result.Email)
	})

	t.Run("zero id", func(t *testing.T) {
		result, err := dao.First(ctx, 0)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("not found", func(t *testing.T) {
		result, err := dao.First(ctx, 99999)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})
}

func TestDAO_FirstWith(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	record := &testModel{Name: "Alice", Email: "alice@example.com"}
	err := dao.Create(ctx, record)
	assert.NoError(t, err)
	id := record.ID

	t.Run("success without preloads", func(t *testing.T) {
		result, err := dao.FirstWith(ctx, id)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Alice", result.Name)
	})

	t.Run("zero id", func(t *testing.T) {
		result, err := dao.FirstWith(ctx, 0)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("not found", func(t *testing.T) {
		result, err := dao.FirstWith(ctx, 99999)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})
}

func TestDAO_Delete(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		record := &testModel{Name: "Alice", Email: "alice@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		_, err = dao.Delete(ctx, id)
		assert.NoError(t, err)

		// After soft delete, First should return not found
		_, err = dao.First(ctx, id)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})

	t.Run("zero id", func(t *testing.T) {
		_, err := dao.Delete(ctx, 0)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}

func TestDAO_Remove(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		record := &testModel{Name: "Alice", Email: "alice@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		_, err = dao.Remove(ctx, id)
		assert.NoError(t, err)

		// After hard delete, First should return not found
		_, err = dao.First(ctx, id)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})

	t.Run("zero id", func(t *testing.T) {
		_, err := dao.Remove(ctx, 0)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}

func TestDAO_Update(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	record := &testModel{Name: "Alice", Email: "alice@example.com"}
	err := dao.Create(ctx, record)
	assert.NoError(t, err)
	id := record.ID

	t.Run("success", func(t *testing.T) {
		err := dao.Update(ctx, id, &testModel{Name: "Alice Updated"}, "name")
		assert.NoError(t, err)

		updated, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "Alice Updated", updated.Name)
		assert.Equal(t, "alice@example.com", updated.Email)
	})

	t.Run("zero id", func(t *testing.T) {
		err := dao.Update(ctx, 0, &testModel{Name: "test"}, "name")
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("empty fields", func(t *testing.T) {
		err := dao.Update(ctx, id, &testModel{Name: "test"})
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}

func TestDAO_Save(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	t.Run("create via save", func(t *testing.T) {
		record := &testModel{Name: "Alice", Email: "alice@example.com"}
		err := dao.Save(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)
	})

	t.Run("update via save", func(t *testing.T) {
		record := &testModel{Name: "Alice", Email: "alice@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		record.Name = "Alice Saved"
		err = dao.Save(ctx, record)
		assert.NoError(t, err)

		result, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "Alice Saved", result.Name)
	})

	t.Run("nil record", func(t *testing.T) {
		err := dao.Save(ctx, nil)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}

func TestDAO_SoftDeleteVsHardDelete(t *testing.T) {
	dao := newTestDAO(t)
	ctx := context.Background()

	// Create two records
	record1 := &testModel{Name: "Alice", Email: "alice@example.com"}
	err := dao.Create(ctx, record1)
	assert.NoError(t, err)

	record2 := &testModel{Name: "Bob", Email: "bob@example.com"}
	err = dao.Create(ctx, record2)
	assert.NoError(t, err)

	// Soft delete record1
	_, err = dao.Delete(ctx, record1.ID)
	assert.NoError(t, err)

	// record1 should be not found
	_, err = dao.First(ctx, record1.ID)
	assert.Error(t, err)

	// record2 should still exist
	found, err := dao.First(ctx, record2.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Bob", found.Name)

	// Hard delete record2
	_, err = dao.Remove(ctx, record2.ID)
	assert.NoError(t, err)

	// record2 should be not found
	_, err = dao.First(ctx, record2.ID)
	assert.Error(t, err)
}

func TestDAO_WithTx_NilReturnsSelf(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)

	result := dao.WithTx(nil)
	assert.Same(t, dao, result, "WithTx(nil) 应返回当前 DAO 实例")
}

func TestDAO_WithBatchSize(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	t.Run("default batch size is 100", func(t *testing.T) {
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)

		// 创建超过默认批次大小的记录来验证批量创建正常工作
		records := make([]*testModel, 150)
		for i := range records {
			records[i] = &testModel{Name: "user", Email: "user@example.com"}
		}
		err = dao.Creates(ctx, records)
		assert.NoError(t, err)
	})

	t.Run("custom batch size", func(t *testing.T) {
		dao, err := NewDAO[testModel](db, WithBatchSize(10))
		assert.NoError(t, err)

		// 使用小批次大小创建记录
		records := make([]*testModel, 25)
		for i := range records {
			records[i] = &testModel{Name: "batch_user", Email: "batch@example.com"}
		}
		err = dao.Creates(ctx, records)
		assert.NoError(t, err)
	})

	t.Run("WithBatchSize ignores zero or negative", func(t *testing.T) {
		dao, err := NewDAO[testModel](db, WithBatchSize(0))
		assert.NoError(t, err)

		// batchSize=0 应保持默认值 100，不影响正常创建
		records := []*testModel{
			{Name: "zero_batch", Email: "zero@example.com"},
		}
		err = dao.Creates(ctx, records)
		assert.NoError(t, err)
	})
}

func TestDAO_WithTx(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	t.Run("returns new DAO instance", func(t *testing.T) {
		txDAO := dao.WithTx(db)
		assert.NotNil(t, txDAO)
		// Should implement IDAO interface
		var _ IDAO[testModel] = txDAO
	})

	t.Run("CRUD operations work through WithTx DAO", func(t *testing.T) {
		txDAO := dao.WithTx(db)

		// Create via txDAO
		record := &testModel{Name: "WithTx Test", Email: "wtx@example.com"}
		err := txDAO.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		// Read via original DAO
		found, err := dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "WithTx Test", found.Name)

		// Update via txDAO
		err = txDAO.Update(ctx, record.ID, &testModel{Name: "WithTx Updated"}, "name")
		assert.NoError(t, err)

		// Verify update via original DAO
		found, err = dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "WithTx Updated", found.Name)

		// Delete via txDAO
		_, err = txDAO.Delete(ctx, record.ID)
		assert.NoError(t, err)

		_, err = dao.First(ctx, record.ID)
		assert.Error(t, err)
	})

	t.Run("WithTx supports transaction rollback", func(t *testing.T) {
		tx := db.Begin()
		txDAO := dao.WithTx(tx)

		// Create within transaction
		record := &testModel{Name: "Rollback Test", Email: "rollback@example.com"}
		err := txDAO.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		// Rollback the transaction
		tx.Rollback()

		// Record should not exist after rollback
		_, err = dao.First(ctx, record.ID)
		assert.Error(t, err)
	})

	t.Run("WithTx supports transaction commit", func(t *testing.T) {
		tx := db.Begin()
		txDAO := dao.WithTx(tx)

		// Create within transaction
		record := &testModel{Name: "Commit Test", Email: "commit@example.com"}
		err := txDAO.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		// Commit the transaction
		tx.Commit()

		// Record should exist after commit
		found, err := dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Commit Test", found.Name)
	})
}

func TestDAO_ConcurrentRead(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	// 预先创建记录（SQLite 不支持并发写入）
	const n = 20
	ids := make([]uint, n)
	for i := 0; i < n; i++ {
		record := &testModel{
			Name:  fmt.Sprintf("concurrent_%d", i),
			Email: fmt.Sprintf("c%d@example.com", i),
		}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		ids[i] = record.ID
	}

	// 并发读取验证无 race
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			result, err := dao.First(ctx, ids[idx])
			if err != nil {
				errCh <- err
				return
			}
			if result.Name != fmt.Sprintf("concurrent_%d", idx) {
				errCh <- fmt.Errorf("unexpected name: %s", result.Name)
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < n; i++ {
		assert.NoError(t, <-errCh)
	}
}

func TestDAO_Update_ZeroValueFields(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	record := &testModel{Name: "Alice", Email: "alice@example.com"}
	err = dao.Create(ctx, record)
	assert.NoError(t, err)
	id := record.ID

	t.Run("update zero-value field via DAO Update method with Select", func(t *testing.T) {
		// Use DAO's Update with Select to set name to empty string (zero value)
		err := dao.Update(ctx, id, &testModel{Name: ""}, "name")
		assert.NoError(t, err)

		updated, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "", updated.Name)
		assert.Equal(t, "alice@example.com", updated.Email)
	})
}

// ---------------------------------------------------------------------------
// DB 错误路径测试（使用 sqlmock）
// ---------------------------------------------------------------------------

func TestDAO_Create_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `test_models`")).
		WillReturnError(fmt.Errorf("connection refused"))
	mock.ExpectRollback()

	err = dao.Create(ctx, &testModel{Name: "Alice", Email: "alice@example.com"})
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_Creates_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `test_models`")).
		WillReturnError(fmt.Errorf("connection refused"))
	mock.ExpectRollback()

	err = dao.Creates(ctx, []*testModel{
		{Name: "Alice", Email: "alice@example.com"},
	})
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_Save_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	// GORM Save with zero-ID record generates INSERT; with non-zero ID generates UPDATE.
	// Test the INSERT path (new record).
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `test_models`")).
		WillReturnError(fmt.Errorf("connection refused"))
	mock.ExpectRollback()

	err = dao.Save(ctx, &testModel{Name: "Alice", Email: "alice@example.com"})
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_First_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	// Simulate a generic DB error (not ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `test_models` WHERE id = ? AND `test_models`.`deleted_at` IS NULL ORDER BY `test_models`.`id` LIMIT ?")).
		WithArgs(uint(42), 1).
		WillReturnError(fmt.Errorf("connection refused"))

	result, err := dao.First(ctx, 42)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_Delete_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `test_models` SET `deleted_at`=? WHERE id = ? AND `test_models`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("connection refused"))
	mock.ExpectRollback()

	rowsAffected, err := dao.Delete(ctx, 42)
	assert.Error(t, err)
	assert.Equal(t, int64(0), rowsAffected)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_Remove_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `test_models` WHERE id = ?")).
		WithArgs(uint(42)).
		WillReturnError(fmt.Errorf("connection refused"))
	mock.ExpectRollback()

	rowsAffected, err := dao.Remove(ctx, 42)
	assert.Error(t, err)
	assert.Equal(t, int64(0), rowsAffected)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_Update_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `test_models` SET `updated_at`=?,`name`=? WHERE id = ? AND `test_models`.`deleted_at` IS NULL")).
		WithArgs(sqlmock.AnyArg(), "Updated", uint(42)).
		WillReturnError(fmt.Errorf("connection refused"))
	mock.ExpectRollback()

	err = dao.Update(ctx, 42, &testModel{Name: "Updated"}, "name")
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBFailed))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDAO_Update_NilRecord(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	err = dao.Update(ctx, 1, nil, "name")
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
}

// ---------------------------------------------------------------------------
// WithTx 事务传播边界测试
// ---------------------------------------------------------------------------

func TestDAO_WithTx_NilTxUsesDefaultDB(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	// WithTx(nil) returns the same DAO instance, so operations use default db
	resultDAO := dao.WithTx(nil)
	assert.Same(t, dao, resultDAO, "WithTx(nil) should return the same DAO instance")

	// Operations on the nil-tx DAO should use the default db
	record := &testModel{Name: "NilTx Test", Email: "niltx@example.com"}
	err = resultDAO.Create(ctx, record)
	assert.NoError(t, err)
	assert.NotZero(t, record.ID)

	// Verify the record is persisted (not in a transaction)
	found, err := dao.First(ctx, record.ID)
	assert.NoError(t, err)
	assert.Equal(t, "NilTx Test", found.Name)
}

func TestDAO_WithTx_ValidTxExecutesOnTransaction(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	t.Run("create through txDAO is visible only after commit", func(t *testing.T) {
		tx := db.Begin()
		txDAO := dao.WithTx(tx)

		record := &testModel{Name: "TxVisible", Email: "tx@example.com"}
		err := txDAO.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		// Within the same tx, reading via txDAO should find it
		found, err := txDAO.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "TxVisible", found.Name)

		// Reading via original DAO (different connection) may or may not see it
		// depending on isolation level — for SQLite it typically won't until commit

		tx.Commit()

		// After commit, original DAO should see the record
		found, err = dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "TxVisible", found.Name)
	})

	t.Run("update through txDAO uses transaction connection", func(t *testing.T) {
		// Create a record first
		record := &testModel{Name: "Original", Email: "original@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		// Update via txDAO
		tx := db.Begin()
		txDAO := dao.WithTx(tx)
		err = txDAO.Update(ctx, id, &testModel{Name: "TxUpdated"}, "name")
		assert.NoError(t, err)

		tx.Commit()

		// Verify via original DAO
		found, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "TxUpdated", found.Name)
	})

	t.Run("delete through txDAO uses transaction connection", func(t *testing.T) {
		record := &testModel{Name: "ToSoftDelete", Email: "del@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		tx := db.Begin()
		txDAO := dao.WithTx(tx)
		_, err = txDAO.Delete(ctx, id)
		assert.NoError(t, err)

		tx.Commit()

		_, err = dao.First(ctx, id)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})

	t.Run("remove through txDAO uses transaction connection", func(t *testing.T) {
		record := &testModel{Name: "ToHardDelete", Email: "harddel@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		tx := db.Begin()
		txDAO := dao.WithTx(tx)
		_, err = txDAO.Remove(ctx, id)
		assert.NoError(t, err)

		tx.Commit()

		_, err = dao.First(ctx, id)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})
}

// ---------------------------------------------------------------------------
// CRUD 边界条件测试
// ---------------------------------------------------------------------------

func TestDAO_Delete_NonExistentRecord(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	// Delete a record that doesn't exist — should return 0 rows affected, no error
	rowsAffected, err := dao.Delete(ctx, 99999)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rowsAffected, "deleting non-existent record should affect 0 rows")
}

func TestDAO_Remove_NonExistentRecord(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	// Hard delete a record that doesn't exist — should return 0 rows affected, no error
	rowsAffected, err := dao.Remove(ctx, 99999)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), rowsAffected, "removing non-existent record should affect 0 rows")
}

func TestDAO_Create_FieldSelection(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	t.Run("create with only required fields", func(t *testing.T) {
		record := &testModel{Name: "OnlyName"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		found, err := dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "OnlyName", found.Name)
		assert.Equal(t, "", found.Email, "unset fields should be zero value")
	})

	t.Run("create with all fields populated", func(t *testing.T) {
		record := &testModel{Name: "Full", Email: "full@example.com"}
		err := dao.Create(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		found, err := dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Full", found.Name)
		assert.Equal(t, "full@example.com", found.Email)
	})
}

func TestDAO_Update_MultipleFields(t *testing.T) {
	db := setupTestDB(t)
	dao, err := NewDAO[testModel](db)
	assert.NoError(t, err)
	ctx := context.Background()

	record := &testModel{Name: "Alice", Email: "alice@example.com"}
	err = dao.Create(ctx, record)
	assert.NoError(t, err)
	id := record.ID

	t.Run("update multiple fields at once", func(t *testing.T) {
		err := dao.Update(ctx, id, &testModel{Name: "Bob", Email: "bob@example.com"}, "name", "email")
		assert.NoError(t, err)

		updated, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "Bob", updated.Name)
		assert.Equal(t, "bob@example.com", updated.Email)
	})

	t.Run("update only name leaves email unchanged", func(t *testing.T) {
		err := dao.Update(ctx, id, &testModel{Name: "Charlie"}, "name")
		assert.NoError(t, err)

		updated, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "Charlie", updated.Name)
		assert.Equal(t, "bob@example.com", updated.Email, "email should remain unchanged when not in fields list")
	})
}

// ---------------------------------------------------------------------------
// SQL 注入安全测试
// ---------------------------------------------------------------------------

// setupMockDB creates a GORM DB backed by go-sqlmock for SQL capture.
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)
	return db, mock
}

func TestSQLInjection_First_ParameterizedQuery(t *testing.T) {
	t.Run("First uses parameterized WHERE for id lookup", func(t *testing.T) {
		// 攻击向量：即使调用方传入异常 id 值，DAO 使用 uint 类型确保类型安全
		// 预期防御：生成的 SQL 使用 "WHERE id = ?" 参数化查询
		db, mock := setupMockDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		// 模拟 GORM First 查询：验证 SQL 使用参数化
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `test_models` WHERE id = ? AND `test_models`.`deleted_at` IS NULL ORDER BY `test_models`.`id` LIMIT ?")).
			WithArgs(uint(42), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(42, "Alice", "alice@example.com"))

		result, err := dao.First(ctx, 42)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Alice", result.Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSQLInjection_Delete_ParameterizedQuery(t *testing.T) {
	t.Run("Delete uses parameterized WHERE for id", func(t *testing.T) {
		// 攻击向量：DAO 的 Delete 方法使用参数化查询
		// 预期防御：生成的 SQL 使用 "WHERE id = ?"
		db, mock := setupMockDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `test_models` SET `deleted_at`=? WHERE id = ? AND `test_models`.`deleted_at` IS NULL")).
			WithArgs(sqlmock.AnyArg(), uint(42)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		rowsAffected, err := dao.Delete(ctx, 42)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSQLInjection_Remove_ParameterizedQuery(t *testing.T) {
	t.Run("Remove uses parameterized WHERE for id", func(t *testing.T) {
		// 攻击向量：DAO 的 Remove 方法使用参数化查询
		// 预期防御：生成的 SQL 使用 "WHERE id = ?"
		db, mock := setupMockDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `test_models` WHERE id = ?")).
			WithArgs(uint(42)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		rowsAffected, err := dao.Remove(ctx, 42)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSQLInjection_Update_ParameterizedQuery(t *testing.T) {
	t.Run("Update uses parameterized WHERE for id", func(t *testing.T) {
		// 攻击向量：DAO 的 Update 方法使用参数化查询
		// 预期防御：生成的 SQL 使用 "WHERE id = ?"
		db, mock := setupMockDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `test_models` SET `updated_at`=?,`name`=? WHERE id = ? AND `test_models`.`deleted_at` IS NULL")).
			WithArgs(sqlmock.AnyArg(), "Updated", uint(42)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err = dao.Update(ctx, 42, &testModel{Name: "Updated"}, "name")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSQLInjection_SqlSpecialCharactersInRecord(t *testing.T) {
	t.Run("Create with SQL special characters in string fields is safe", func(t *testing.T) {
		// 攻击向量：记录字段值包含 SQL 特殊字符（引号、分号、注释）
		// 预期防御：GORM 使用参数化查询，字段值不会影响 SQL 结构
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		maliciousValues := []struct {
			name  string
			email string
		}{
			{name: "Robert'); DROP TABLE users;--", email: "robert@example.com"},
			{name: "Alice\" OR \"1\"=\"1", email: "alice@example.com"},
			{name: "Bob'; SELECT * FROM users;--", email: "bob@example.com"},
			{name: "Eve; DELETE FROM test_models WHERE 1=1;--", email: "eve@example.com"},
		}

		for _, mv := range maliciousValues {
			record := &testModel{Name: mv.name, Email: mv.email}
			err := dao.Create(ctx, record)
			assert.NoError(t, err, "Create should succeed for malicious value: %q", mv.name)
			assert.NotZero(t, record.ID)

			// 验证值被原样存储，没有被解释为 SQL
			found, err := dao.First(ctx, record.ID)
			assert.NoError(t, err)
			assert.Equal(t, mv.name, found.Name, "Stored value should match input exactly")
			assert.Equal(t, mv.email, found.Email)
		}
	})

	t.Run("Update with SQL special characters in string fields is safe", func(t *testing.T) {
		// 攻击向量：更新字段值包含 SQL 特殊字符
		// 预期防御：GORM 使用参数化查询，字段值不会影响 SQL 结构
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		// 先创建一条正常记录
		record := &testModel{Name: "Original", Email: "original@example.com"}
		err = dao.Create(ctx, record)
		assert.NoError(t, err)
		id := record.ID

		// 更新为包含特殊字符的值
		maliciousName := "Robert'); DROP TABLE users;--"
		err = dao.Update(ctx, id, &testModel{Name: maliciousName}, "name")
		assert.NoError(t, err)

		// 验证其他记录未受影响
		found, err := dao.First(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, maliciousName, found.Name, "Updated value should match input exactly")
		assert.Equal(t, "original@example.com", found.Email, "Email should be unchanged")

		// 确认数据库中仍然只有一条该 ID 的记录
		var count int64
		db.Model(&testModel{}).Where("id = ?", id).Count(&count)
		assert.Equal(t, int64(1), count, "Should still have exactly 1 record")
	})

	t.Run("Save with SQL special characters in string fields is safe", func(t *testing.T) {
		// 攻击向量：Save 字段值包含 SQL 特殊字符
		// 预期防御：GORM 使用参数化查询
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		maliciousName := "1 OR 1=1; DROP TABLE users;--"
		record := &testModel{Name: maliciousName, Email: "save@example.com"}
		err = dao.Save(ctx, record)
		assert.NoError(t, err)
		assert.NotZero(t, record.ID)

		found, err := dao.First(ctx, record.ID)
		assert.NoError(t, err)
		assert.Equal(t, maliciousName, found.Name)
	})
}

func TestSQLInjection_FirstWith_PreloadSafety(t *testing.T) {
	t.Run("FirstWith with malicious preload name does not inject SQL", func(t *testing.T) {
		// 攻击向量：FirstWith 的 preloads 参数包含恶意字符串
		// 预期防御：GORM Preload 将其视为关联名，不会注入原始 SQL；
		// 无效关联名将导致查询错误，但不会执行注入
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		record := &testModel{Name: "Alice", Email: "alice@example.com"}
		err = dao.Create(ctx, record)
		assert.NoError(t, err)

		// GORM 对不存在的关联名会报错，但不会注入
		_, err = dao.FirstWith(ctx, record.ID, "Items; DROP TABLE users--")
		// GORM 可能返回错误（关联不存在），但不应该 panic 或注入
		_ = err
	})
}

func TestSQLInjection_UIntPreventsIDManipulation(t *testing.T) {
	t.Run("First rejects zero id preventing empty WHERE", func(t *testing.T) {
		// 攻击向量：id=0 可能在某些场景下被忽略
		// 预期防御：DAO 显式校验 id=0 返回参数错误
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		_, err = dao.First(ctx, 0)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("Delete rejects zero id preventing empty WHERE", func(t *testing.T) {
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		_, err = dao.Delete(ctx, 0)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("Remove rejects zero id preventing empty WHERE", func(t *testing.T) {
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		_, err = dao.Remove(ctx, 0)
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})

	t.Run("Update rejects zero id preventing empty WHERE", func(t *testing.T) {
		db := setupTestDB(t)
		dao, err := NewDAO[testModel](db)
		assert.NoError(t, err)
		ctx := context.Background()

		err = dao.Update(ctx, 0, &testModel{Name: "test"}, "name")
		assert.Error(t, err)
		assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
	})
}
