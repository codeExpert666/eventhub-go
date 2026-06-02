package db

import (
	"context"
	"database/sql"
	"errors"
)

type txContextKey struct{}

// Transactor 由 service 层用于显式控制数据库事务边界。
type Transactor struct {
	db      *sql.DB
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
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := t.db.BeginTx(ctx, t.options)
	if err != nil {
		return err
	}
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
