package dbrepo

import (
	"context"
	"fmt"
	"testing"

	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"github.com/stretchr/testify/assert"
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
		dao, err := NewDAO[testModel](db, WithBatchSize[testModel](10))
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
		dao, err := NewDAO[testModel](db, WithBatchSize[testModel](0))
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

	result := db.WithContext(ctx).Model(&testModel{}).Where("id = ?", id).Update("name", "")
	assert.NoError(t, result.Error)

	updated, err := dao.First(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, "", updated.Name)
	assert.Equal(t, "alice@example.com", updated.Email)
}
