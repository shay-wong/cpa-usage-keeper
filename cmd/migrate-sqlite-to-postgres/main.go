package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const copyBatchSize = 10

type tableCopy struct {
	name string
	copy func(context.Context, *gorm.DB, *gorm.DB) (int64, error)
}

func main() {
	sourcePath := flag.String("sqlite", "", "source SQLite app.db path")
	databaseURL := flag.String("database-url", "", "target PostgreSQL DATABASE_URL")
	truncate := flag.Bool("truncate", false, "truncate target tables before importing")
	flag.Parse()

	if err := run(context.Background(), strings.TrimSpace(*sourcePath), strings.TrimSpace(*databaseURL), *truncate); err != nil {
		fmt.Fprintf(os.Stderr, "migrate sqlite to postgres: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, sourcePath string, databaseURL string, truncate bool) error {
	if sourcePath == "" {
		return fmt.Errorf("-sqlite is required")
	}
	if databaseURL == "" {
		return fmt.Errorf("-database-url is required")
	}
	source, err := openSQLiteSource(sourcePath)
	if err != nil {
		return err
	}
	defer closeDatabase(source)

	target, err := repository.OpenDatabase(config.Config{DatabaseDriver: config.DatabaseDriverPostgres, DatabaseURL: databaseURL})
	if err != nil {
		return err
	}
	defer closeDatabase(target)

	copies := orderedTableCopies()
	if truncate {
		if err := truncateTargetTables(target, copies); err != nil {
			return err
		}
	} else if err := ensureTargetTablesEmpty(target, copies); err != nil {
		return err
	}

	for _, item := range copies {
		count, err := item.copy(ctx, source, target)
		if err != nil {
			return fmt.Errorf("copy %s: %w", item.name, err)
		}
		fmt.Printf("copied %s: %d rows\n", item.name, count)
	}
	return resetPostgresSequences(target, copies)
}

func openSQLiteSource(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro&_query_only=true"), &gorm.Config{NowFunc: func() time.Time { return timeutil.NormalizeStorageTime(time.Now()) }})
	if err != nil {
		return nil, fmt.Errorf("open source sqlite database: %w", err)
	}
	return db, nil
}

func orderedTableCopies() []tableCopy {
	return []tableCopy{
		{name: "database_cleanup_settings", copy: copyTable[entities.DatabaseCleanupSettings]},
		{name: "model_price_settings", copy: copyTable[entities.ModelPriceSetting]},
		{name: "usage_identities", copy: copyTable[entities.UsageIdentity]},
		{name: "cpa_api_keys", copy: copyTable[entities.CPAAPIKey]},
		{name: "usage_events", copy: copyTable[entities.UsageEvent]},
		{name: "usage_request_details", copy: copyTable[entities.UsageRequestDetail]},
		{name: "usage_overview_aggregation_checkpoints", copy: copyTable[entities.UsageOverviewAggregationCheckpoint]},
		{name: "usage_overview_hourly_stats", copy: copyTable[entities.UsageOverviewHourlyStat]},
		{name: "usage_overview_daily_stats", copy: copyTable[entities.UsageOverviewDailyStat]},
		{name: "usage_overview_health_stats", copy: copyTable[entities.UsageOverviewHealthStat]},
		{name: "redis_usage_inboxes", copy: copyTable[entities.RedisUsageInbox]},
	}
}

func copyTable[T any](ctx context.Context, source *gorm.DB, target *gorm.DB) (int64, error) {
	var copied int64
	var rows []T
	err := source.WithContext(ctx).FindInBatches(&rows, copyBatchSize, func(tx *gorm.DB, batch int) error {
		if len(rows) == 0 {
			return nil
		}
		if err := target.WithContext(ctx).CreateInBatches(rows, copyBatchSize).Error; err != nil {
			return fmt.Errorf("insert batch %d: %w", batch, err)
		}
		copied += int64(len(rows))
		return nil
	}).Error
	if err != nil {
		return copied, err
	}
	return copied, nil
}

func ensureTargetTablesEmpty(db *gorm.DB, copies []tableCopy) error {
	for _, item := range copies {
		var count int64
		if err := db.Table(item.name).Count(&count).Error; err != nil {
			return fmt.Errorf("count target table %s: %w", item.name, err)
		}
		if count > 0 {
			return fmt.Errorf("target table %s is not empty; rerun with -truncate to replace it", item.name)
		}
	}
	return nil
}

func truncateTargetTables(db *gorm.DB, copies []tableCopy) error {
	for index := len(copies) - 1; index >= 0; index-- {
		if err := db.Exec("TRUNCATE TABLE " + copies[index].name + " RESTART IDENTITY CASCADE").Error; err != nil {
			return fmt.Errorf("truncate target table %s: %w", copies[index].name, err)
		}
	}
	return nil
}

func resetPostgresSequences(db *gorm.DB, copies []tableCopy) error {
	for _, item := range copies {
		if err := db.Exec(`
			SELECT setval(
				pg_get_serial_sequence(?, 'id'),
				COALESCE((SELECT MAX(id) FROM `+item.name+`), 1),
				(SELECT COUNT(*) FROM `+item.name+`) > 0
			)
		`, item.name).Error; err != nil {
			return fmt.Errorf("reset sequence for %s: %w", item.name, err)
		}
	}
	return nil
}

func closeDatabase(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
