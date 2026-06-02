package mysql

import (
	"context"
	"database/sql"

	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository/mysql/sqlc"
)

type queryFactory struct {
	db *sql.DB
}

func (f queryFactory) queries(ctx context.Context) *sqlc.Queries {
	if tx, ok := platformdb.TxFromContext(ctx); ok {
		return sqlc.New(tx)
	}
	return sqlc.New(f.db)
}
