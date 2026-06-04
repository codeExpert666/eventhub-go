package db

import (
	"context"
	"database/sql"
	"errors"
)

type txContextKey struct{}

// TxRunner 表示 service 层需要的事务运行能力。
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

// Transactor 由 service 层用于显式控制数据库事务边界。
type Transactor struct {
	db *sql.DB
	// options 是创建新事务时传给 database/sql 的选项，用于指定隔离级别和只读模式；nil 表示使用数据库默认配置。
	options *sql.TxOptions
}

// NewTransactor 创建事务控制器。
func NewTransactor(database *sql.DB, options *sql.TxOptions) *Transactor {
	return &Transactor{db: database, options: options}
}

// WithinTx 在事务内执行 fn。若 ctx 已经携带事务，则复用当前事务。
func (t *Transactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if t == nil || t.db == nil {
		return errors.New("transactor database is nil")
	}
	// 已有事务表示当前调用嵌套在外层事务内，提交、回滚和事务选项均由外层事务创建者决定。
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := t.db.BeginTx(ctx, t.options)
	if err != nil {
		return err
	}
	// 延迟回滚用于兜底错误路径；事务提交成功后 Rollback 会变成已结束错误，可忽略。
	defer func() {
		_ = tx.Rollback()
	}()

	txCtx := ContextWithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// ContextWithTx 把事务写入 context，供 repository/mysql 选择 sqlc 执行器。
func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext 读取当前事务。
func TxFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*sql.Tx)
	return tx, ok && tx != nil
}
