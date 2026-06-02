package mysql

import (
	"context"
	"database/sql"

	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository/mysql/sqlc"
)

// queryFactory 为 MySQL repository 集中创建绑定到正确执行器的 sqlc 查询对象，
// 避免各 repository 重复实现事务选择逻辑。
type queryFactory struct {
	// db 是无事务上下文时使用的默认数据库连接池。
	db *sql.DB
}

// queries 优先复用 ctx 中的事务，否则使用默认连接池。
func (f queryFactory) queries(ctx context.Context) *sqlc.Queries {
	if tx, ok := platformdb.TxFromContext(ctx); ok {
		return sqlc.New(tx)
	}
	return sqlc.New(f.db)
}
