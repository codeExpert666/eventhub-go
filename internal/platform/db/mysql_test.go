package db

import (
	"testing"

	drivermysql "github.com/go-sql-driver/mysql"
)

func TestNormalizeMySQLDSNForcesParseTime(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "missing parseTime",
			dsn:  "eventhub:eventhub@tcp(127.0.0.1:3306)/eventhub?multiStatements=true",
		},
		{
			name: "explicit false parseTime",
			dsn:  "eventhub:eventhub@tcp(127.0.0.1:3306)/eventhub?parseTime=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := normalizeMySQLDSN(tt.dsn)
			if err != nil {
				t.Fatalf("normalize dsn: %v", err)
			}
			cfg, err := drivermysql.ParseDSN(normalized)
			if err != nil {
				t.Fatalf("parse normalized dsn: %v", err)
			}
			if !cfg.ParseTime {
				t.Fatalf("expected parseTime=true, got normalized DSN %q", normalized)
			}
		})
	}
}
