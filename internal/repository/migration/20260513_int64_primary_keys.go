package migration

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type int64PrimaryKeyColumn struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
	PK   int    `gorm:"column:pk"`
}

func useInt64PrimaryKeysMigration(tx *gorm.DB) error {
	for _, table := range []string{"usage_events", "usage_identities", "redis_usage_inboxes", "model_price_settings"} {
		ok, err := tableHasInt64PrimaryKey(tx, table)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("table %s id column is not an integer primary key", table)
		}
	}
	return nil
}

func tableHasInt64PrimaryKey(tx *gorm.DB, table string) (bool, error) {
	switch tx.Dialector.Name() {
	case "sqlite":
		return sqliteTableHasInt64PrimaryKey(tx, table)
	case "postgres":
		return postgresTableHasInt64PrimaryKey(tx, table)
	default:
		return false, fmt.Errorf("unsupported database driver %q", tx.Dialector.Name())
	}
}

func sqliteTableHasInt64PrimaryKey(tx *gorm.DB, table string) (bool, error) {
	var columns []int64PrimaryKeyColumn
	if err := tx.Raw(fmt.Sprintf("PRAGMA table_info(%s)", table)).Scan(&columns).Error; err != nil {
		return false, fmt.Errorf("inspect %s schema: %w", table, err)
	}
	if len(columns) == 0 {
		return true, nil
	}
	for _, column := range columns {
		if strings.EqualFold(column.Name, "id") {
			return column.PK > 0 && strings.Contains(strings.ToUpper(column.Type), "INT"), nil
		}
	}
	return false, nil
}

func postgresTableHasInt64PrimaryKey(tx *gorm.DB, table string) (bool, error) {
	var tableCount int64
	if err := tx.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?", table).Scan(&tableCount).Error; err != nil {
		return false, fmt.Errorf("inspect %s table: %w", table, err)
	}
	if tableCount == 0 {
		return true, nil
	}

	var dataTypes []struct {
		DataType string `gorm:"column:data_type"`
	}
	if err := tx.Raw(`
		SELECT columns.data_type
		FROM information_schema.columns AS columns
		JOIN information_schema.key_column_usage AS key_columns
			ON columns.table_schema = key_columns.table_schema
			AND columns.table_name = key_columns.table_name
			AND columns.column_name = key_columns.column_name
		JOIN information_schema.table_constraints AS constraints
			ON key_columns.constraint_schema = constraints.constraint_schema
			AND key_columns.constraint_name = constraints.constraint_name
			AND key_columns.table_schema = constraints.table_schema
			AND key_columns.table_name = constraints.table_name
		WHERE columns.table_schema = current_schema()
			AND columns.table_name = ?
			AND columns.column_name = 'id'
			AND constraints.constraint_type = 'PRIMARY KEY'
	`, table).Scan(&dataTypes).Error; err != nil {
		return false, fmt.Errorf("inspect %s primary key: %w", table, err)
	}
	for _, column := range dataTypes {
		if strings.EqualFold(column.DataType, "bigint") {
			return true, nil
		}
	}
	return false, nil
}
