package dbrepo

import (
	"context"
	"errors"
	"testing"

	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestRunInTx_Commit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := RunInTx(ctx, db, func(tx *gorm.DB) error {
		return tx.Create(&testModel{Name: "Alice", Email: "alice@example.com"}).Error
	})
	assert.NoError(t, err)

	// Record should exist after commit
	var record testModel
	err = db.Where("name = ?", "Alice").First(&record).Error
	assert.NoError(t, err)
	assert.Equal(t, "Alice", record.Name)
	assert.Equal(t, "alice@example.com", record.Email)
}

func TestRunInTx_Rollback(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := RunInTx(ctx, db, func(tx *gorm.DB) error {
		if err := tx.Create(&testModel{Name: "TxRollback", Email: "txrb@example.com"}).Error; err != nil {
			return err
		}
		return errors.New("something went wrong")
	})
	assert.Error(t, err)
	assert.Equal(t, "something went wrong", err.Error())

	// Record should NOT exist after rollback
	var count int64
	db.Model(&testModel{}).Where("name = ?", "TxRollback").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestRunInTx_NilDB(t *testing.T) {
	ctx := context.Background()
	err := RunInTx(ctx, nil, func(tx *gorm.DB) error {
		return nil
	})
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
}

func TestRunInTx_NilFn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	err := RunInTx(ctx, db, nil)
	assert.Error(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.DBRequestParamError))
}

func TestRunInTx_NestedSavepoint(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := RunInTx(ctx, db, func(tx *gorm.DB) error {
		// Create outer record
		if err := tx.Create(&testModel{Name: "Outer", Email: "outer@example.com"}).Error; err != nil {
			return err
		}

		// Nested RunInTx — should use Savepoint since tx is already in a transaction
		return RunInTx(ctx, tx, func(tx2 *gorm.DB) error {
			return tx2.Create(&testModel{Name: "Inner", Email: "inner@example.com"}).Error
		})
	})
	assert.NoError(t, err)

	// Both records should exist
	var outer testModel
	err = db.Where("name = ?", "Outer").First(&outer).Error
	assert.NoError(t, err)
	assert.Equal(t, "Outer", outer.Name)

	var inner testModel
	err = db.Where("name = ?", "Inner").First(&inner).Error
	assert.NoError(t, err)
	assert.Equal(t, "Inner", inner.Name)
}

func TestRunInTx_NestedSavepointRollback(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := RunInTx(ctx, db, func(tx *gorm.DB) error {
		// Create outer record
		if err := tx.Create(&testModel{Name: "Outer2", Email: "outer2@example.com"}).Error; err != nil {
			return err
		}

		// Nested RunInTx that fails — should rollback to savepoint but not affect outer transaction
		nestedErr := RunInTx(ctx, tx, func(tx2 *gorm.DB) error {
			if err := tx2.Create(&testModel{Name: "InnerFail", Email: "innerfail@example.com"}).Error; err != nil {
				return err
			}
			return errors.New("nested failure")
		})
		assert.Error(t, nestedErr, "nested RunInTx should return error")

		// Outer transaction continues — create another record
		return tx.Create(&testModel{Name: "Outer3", Email: "outer3@example.com"}).Error
	})
	assert.NoError(t, err)

	// Outer records should exist
	var outer2 testModel
	err = db.Where("name = ?", "Outer2").First(&outer2).Error
	assert.NoError(t, err)

	var outer3 testModel
	err = db.Where("name = ?", "Outer3").First(&outer3).Error
	assert.NoError(t, err)

	// InnerFail record should NOT exist (rolled back to savepoint)
	var count int64
	db.Model(&testModel{}).Where("name = ?", "InnerFail").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestRunInTx_Panic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "should re-panic")
		assert.Equal(t, "boom", r)

		// Record should NOT exist (rolled back)
		var count int64
		db.Model(&testModel{}).Where("name = ?", "Panic").Count(&count)
		assert.Equal(t, int64(0), count)
	}()

	_ = RunInTx(ctx, db, func(tx *gorm.DB) error {
		if err := tx.Create(&testModel{Name: "Panic", Email: "panic@example.com"}).Error; err != nil {
			return err
		}
		panic("boom")
	})
}
