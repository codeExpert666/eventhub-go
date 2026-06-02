package db_test

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"

	"eventhub-go/internal/platform/db"
)

func TestIsUniqueConstraintError(t *testing.T) {
	err := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}
	if !db.IsUniqueConstraintError(err) {
		t.Fatal("expected duplicate entry error to be recognized")
	}

	wrapped := fmt.Errorf("insert user: %w", err)
	if !db.IsUniqueConstraintError(wrapped) {
		t.Fatal("expected wrapped duplicate entry error to be recognized")
	}

	if db.IsUniqueConstraintError(&mysql.MySQLError{Number: 1452, Message: "Cannot add child row"}) {
		t.Fatal("foreign key error must not be recognized as unique constraint error")
	}
}
