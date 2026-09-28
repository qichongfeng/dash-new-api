package model

import (
	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
)

// GetDBTimestamp returns a UNIX timestamp from database time.
// Falls back to application time on error.
func GetDBTimestamp() int64 {
	return getDBTimestampOn(DB)
}

// GetDBTimestampTx is the transaction-scoped variant: callers already holding a
// transaction must not touch the global DB — on single-connection databases
// (in-memory SQLite test setups enforce MaxOpenConns(1)) that self-deadlocks.
func GetDBTimestampTx(tx *gorm.DB) int64 {
	if tx == nil {
		return common.GetTimestamp()
	}
	return getDBTimestampOn(tx)
}

func getDBTimestampOn(db *gorm.DB) int64 {
	var ts int64
	var err error
	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		err = db.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		err = db.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	default:
		err = db.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}
