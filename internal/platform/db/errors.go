package db

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

const mysqlDuplicateEntryErrorNumber uint16 = 1062

// IsUniqueConstraintError 判断错误是否来自 MySQL 唯一约束冲突。
//
// 后续 auth 注册流程会把该错误映射为 AUTH-409。当前函数只识别底层数据库错误，
// 不在 platform 层引入业务错误码。
func IsUniqueConstraintError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntryErrorNumber
}
