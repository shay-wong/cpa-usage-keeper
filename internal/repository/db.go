package repository

import (
	"context"
	"cpa-usage-keeper/internal/repository/dto"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/migration"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// OpenDatabase 统一创建 GORM 连接，并按配置选择 SQLite 或 PostgreSQL 主库。
func OpenDatabase(cfg config.Config) (*gorm.DB, error) {
	driver := strings.TrimSpace(cfg.DatabaseDriver)
	if driver == "" {
		driver = config.DatabaseDriverSQLite
	}
	switch driver {
	case config.DatabaseDriverSQLite:
		return openSQLiteDatabase(cfg)
	case config.DatabaseDriverPostgres:
		return openPostgresDatabase(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func openSQLiteDatabase(cfg config.Config) (*gorm.DB, error) {
	// 先判断物理文件是否存在，后续用它区分全新数据库和需要跑迁移的旧库。
	databaseExists, err := sqliteDatabaseFileExists(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	db, err := openGormDatabase(sqlite.Open(sqliteDSN(cfg.SQLitePath)), cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	if err := configureSQLiteDatabase(db); err != nil {
		return nil, err
	}
	return initializeDatabaseSchema(db, databaseExists)
}

func openPostgresDatabase(cfg config.Config) (*gorm.DB, error) {
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("postgres database URL is required")
	}
	db, err := openGormDatabase(postgres.Open(cfg.DatabaseURL), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := configurePostgresDatabase(db); err != nil {
		return nil, err
	}
	hasTables, err := databaseHasTables(db)
	if err != nil {
		return nil, err
	}
	return initializeDatabaseSchema(db, hasTables)
}

func openGormDatabase(dialector gorm.Dialector, source string) (*gorm.DB, error) {
	db, err := gorm.Open(dialector, &gorm.Config{NowFunc: func() time.Time { return timeutil.NormalizeStorageTime(time.Now()) }})
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", sanitizeDatabaseSource(source), err)
	}
	return db, nil
}

func sanitizeDatabaseSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" {
		return parsed.Redacted()
	}
	return filepath.Clean(trimmed)
}

func configureSQLiteDatabase(db *gorm.DB) error {
	// SQLite 写入仍是单 writer，连接池限制成单连接，避免同进程多个连接互相抢写锁。
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("configure sqlite database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	// WAL 让读写并发更友好，但 writer 之间仍串行；这里配合 busy_timeout 等待短暂锁竞争。
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return fmt.Errorf("enable sqlite WAL: %w", err)
	}
	if err := db.Exec("PRAGMA busy_timeout=5000").Error; err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if err := db.Exec("PRAGMA foreign_keys=ON").Error; err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	return nil
}

func configurePostgresDatabase(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("configure postgres database: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	return nil
}

func initializeDatabaseSchema(db *gorm.DB, databaseExists bool) (*gorm.DB, error) {
	// 空库直接 AutoMigrate 到当前 schema 后标记历史迁移已完成。
	hasTables := databaseExists
	if databaseExists {
		var err error
		hasTables, err = databaseHasTables(db)
		if err != nil {
			return nil, err
		}
	}
	if !hasTables {
		if err := db.AutoMigrate(entities.All()...); err != nil {
			return nil, fmt.Errorf("auto migrate fresh database: %w", err)
		}
		if err := migration.MarkAllAsApplied(db); err != nil {
			return nil, fmt.Errorf("mark schema migrations applied: %w", err)
		}
		return db, nil
	}

	// 已有业务表的数据库必须走显式迁移，确保旧库按版本顺序补齐结构和索引。
	if err := migration.Run(db); err != nil {
		return nil, fmt.Errorf("run schema migrations: %w", err)
	}
	return db, nil
}

// sqliteDSN 在调用方没有自定义 query 参数时追加 SQLite 连接级默认参数。
func sqliteDSN(path string) string {
	// 保留调用方显式传入的 DSN 参数，避免覆盖测试或特殊部署配置。
	trimmed := strings.TrimSpace(path)
	if strings.Contains(trimmed, "?") {
		return trimmed
	}
	return trimmed + "?_busy_timeout=5000&_foreign_keys=on"
}

// sqliteDatabaseFileExists 判断磁盘数据库文件是否存在；内存库和空路径都按新库处理。
func sqliteDatabaseFileExists(path string) (bool, error) {
	trimmed := strings.TrimSpace(path)
	if before, _, ok := strings.Cut(trimmed, "?"); ok {
		trimmed = before
	}
	if trimmed == "" || trimmed == ":memory:" {
		return false, nil
	}
	_, err := os.Stat(trimmed)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("check sqlite database %s: %w", filepath.Clean(trimmed), err)
}

// databaseHasTables 用业务表数量判断数据库是否已经初始化过。
func databaseHasTables(db *gorm.DB) (bool, error) {
	var count int64
	switch db.Dialector.Name() {
	case config.DatabaseDriverSQLite:
		// sqlite_% 是 SQLite 内部表，不能用来判断项目 schema 是否存在。
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
			return false, fmt.Errorf("check sqlite database tables: %w", err)
		}
	case config.DatabaseDriverPostgres:
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'").Scan(&count).Error; err != nil {
			return false, fmt.Errorf("check postgres database tables: %w", err)
		}
	default:
		return false, fmt.Errorf("unsupported database driver %q", db.Dialector.Name())
	}
	return count > 0, nil
}

// InsertUsageEvents 按 Redis inbox 消费结果逐条落库；request_id/event_key 重复也保留为独立事件。
func InsertUsageEvents(db *gorm.DB, events []entities.UsageEvent) (int, int, error) {
	if db == nil {
		return 0, 0, fmt.Errorf("database is nil")
	}
	if len(events) == 0 {
		return 0, 0, nil
	}

	// 仍保留 deduped 返回位是为了兼容上层结果结构；当前语义固定为不去重。
	inserted := 0

	err := db.Transaction(func(tx *gorm.DB) error {
		// 按仓储默认批次拆分写入，避免单条 INSERT 的 SQLite 变量数量过多。
		for start := 0; start < len(events); start += insertBatchSize(entities.UsageEvent{}) {
			end := min(start+insertBatchSize(entities.UsageEvent{}), len(events))
			batch := events[start:end]
			// 入库前统一规范时间，确保 storageTime 字符串比较和后续增量聚合使用同一时区语义。
			for index := range batch {
				batch[index].Timestamp = timeutil.NormalizeStorageTime(batch[index].Timestamp)
			}

			// Redis 队列是消费型数据源，同 request_id/event_key 的消息也代表独立消费记录。
			result := tx.Create(&batch)
			if result.Error != nil {
				return fmt.Errorf("insert usage events: %w", result.Error)
			}
			inserted += int(result.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return inserted, 0, nil
}

// CleanupStorage 是每日维护任务的统一仓储清理入口：清 Redis inbox、清理请求/用量日志，并删除过期 Overview health 细粒度统计。
func CleanupStorage(db *gorm.DB, now time.Time) (dto.StorageCleanupResult, error) {
	return CleanupStorageWithSettings(context.Background(), db, now, dto.DatabaseCleanupSettingsInput{}, "")
}

func CleanupStorageWithSettings(ctx context.Context, db *gorm.DB, now time.Time, settings dto.DatabaseCleanupSettingsInput, sqlitePath string) (dto.StorageCleanupResult, error) {
	redisResult, err := CleanupRedisUsageInbox(db, now)
	if err != nil {
		return dto.StorageCleanupResult{RedisInbox: redisResult}, err
	}
	requestDetailResult, err := CleanupUsageRequestDetails(db, settings, now, sqlitePath)
	if err != nil {
		return dto.StorageCleanupResult{RedisInbox: redisResult, UsageRequestDetail: requestDetailResult}, err
	}
	usageEventResult, err := CleanupUsageEvents(ctx, db, settings, now, sqlitePath)
	if err != nil {
		return dto.StorageCleanupResult{RedisInbox: redisResult, UsageRequestDetail: requestDetailResult, UsageEvent: usageEventResult}, err
	}
	// Health stats 只服务最近窗口展示，过期数据在每日维护中清掉，避免表无限增长。
	if err := CleanupUsageOverviewHealthStats(db, now); err != nil {
		return dto.StorageCleanupResult{RedisInbox: redisResult, UsageRequestDetail: requestDetailResult, UsageEvent: usageEventResult}, err
	}
	return dto.StorageCleanupResult{RedisInbox: redisResult, UsageRequestDetail: requestDetailResult, UsageEvent: usageEventResult}, nil
}

// Vacuum 提供单独的 SQLite 收缩入口，供需要只做文件整理的调用方使用。
func Vacuum(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if db.Dialector.Name() != config.DatabaseDriverSQLite {
		return nil
	}
	return db.Exec("VACUUM").Error
}
