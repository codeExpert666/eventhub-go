package mysql

import "database/sql"

// lastInsertID 从数据库执行结果中读取自增主键 ID。
//
// 如果执行 SQL 时已经返回错误，则直接透传该错误，不再访问 sql.Result。
func lastInsertID(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// rowsAffected 从数据库执行结果中读取受影响的行数。
//
// 如果执行 SQL 时已经返回错误，则直接透传该错误，不再访问 sql.Result。
func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
