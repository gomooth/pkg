package dbrepo

import (
	"context"
	"database/sql"

	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"gorm.io/gorm"
)

// RunInTx 在数据库事务中执行 fn，自动处理 Commit/Rollback。
//
// 行为：
//   - fn 返回 nil → Commit
//   - fn 返回 error → Rollback + 返回 error
//   - fn panic → Rollback + re-panic
//   - 嵌套调用（db 已在事务中）→ 自动创建 Savepoint
func RunInTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return xerror.NewXCode(xcode.DBRequestParamError, "dbrepo: RunInTx called with nil *gorm.DB")
	}
	if fn == nil {
		return xerror.NewXCode(xcode.DBRequestParamError, "dbrepo: RunInTx called with nil fn")
	}

	// 检测是否已在事务中（嵌套调用 → Savepoint）
	if isInTransaction(db) {
		return runInSavepoint(ctx, db, fn)
	}

	return db.WithContext(ctx).Transaction(fn)
}

// isInTransaction 检测 db 是否已在事务中
func isInTransaction(db *gorm.DB) bool {
	if db.Statement == nil || db.Statement.ConnPool == nil {
		return false
	}
	_, ok := db.Statement.ConnPool.(*sql.Tx)
	return ok
}

// runInSavepoint 在已有事务中创建 Savepoint 执行 fn
func runInSavepoint(ctx context.Context, tx *gorm.DB, fn func(tx *gorm.DB) error) error {
	savepointName := "sp_dbrepo"
	if err := tx.WithContext(ctx).SavePoint(savepointName).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	if err := fn(tx); err != nil {
		// Rollback to savepoint
		_ = tx.WithContext(ctx).RollbackTo(savepointName).Error
		return err
	}

	return nil
}
