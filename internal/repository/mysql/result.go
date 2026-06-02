package mysql

import "database/sql"

func lastInsertID(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
