package backup

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PostgresWriter struct {
	dir         string
	databaseURL string
}

func NewPostgresWriter(dir string, databaseURL string) *PostgresWriter {
	return &PostgresWriter{dir: strings.TrimSpace(dir), databaseURL: strings.TrimSpace(databaseURL)}
}

// PostgresToolsAvailable 确认运行环境同时具备页面备份和恢复所需的 PostgreSQL 客户端工具。
func PostgresToolsAvailable() bool {
	for _, tool := range []string{"pg_dump", "pg_restore", "psql"} {
		if _, err := exec.LookPath(tool); err != nil {
			return false
		}
	}
	return true
}

func (w *PostgresWriter) WriteDatabase(ctx context.Context, backupAt time.Time) (string, error) {
	return w.writeDatabaseWithPrefix(ctx, backupAt, Options{RequestLogs: true, UsageLogs: true, UsageIdentities: true, APIKeys: true, RedisInbox: true, ModelPrices: true}, databaseBackupFilePrefix)
}

func (w *PostgresWriter) WriteDatabaseWithOptions(ctx context.Context, backupAt time.Time, options Options) (string, error) {
	return w.writeDatabaseWithPrefix(ctx, backupAt, options, databaseBackupFilePrefix)
}

func (w *PostgresWriter) WriteRestoreSafetyBackup(ctx context.Context, backupAt time.Time) (string, error) {
	return w.writeDatabaseWithPrefix(ctx, backupAt, Options{RequestLogs: true, UsageLogs: true, UsageIdentities: true, APIKeys: true, RedisInbox: true, ModelPrices: true}, restoreSafetyBackupFilePrefix)
}

func (w *PostgresWriter) writeDatabaseWithPrefix(ctx context.Context, backupAt time.Time, options Options, filePrefix string) (string, error) {
	if w == nil {
		return "", fmt.Errorf("postgres backup writer is nil")
	}
	if w.dir == "" {
		return "", fmt.Errorf("backup directory is required")
	}
	if strings.TrimSpace(w.databaseURL) == "" {
		return "", fmt.Errorf("postgres database URL is required")
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return "", fmt.Errorf("pg_dump is required for postgres backups: %w", err)
	}
	options = options.withDefaults()

	stamp := backupAt.In(time.Local)
	if stamp.IsZero() {
		stamp = time.Now().In(time.Local)
	}
	dayDir := filepath.Join(w.dir, stamp.Format("2006-01-02"))
	if err := os.MkdirAll(dayDir, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Chmod(dayDir, 0o700); err != nil {
		return "", fmt.Errorf("restrict backup directory permissions: %w", err)
	}

	fileName := fmt.Sprintf("%s%s.dump", filePrefix, stamp.Format("20060102T150405.000000000"))
	fullPath := filepath.Join(dayDir, fileName)
	tempPath := fullPath + ".tmp"
	_ = os.Remove(tempPath)
	if err := w.dumpDatabase(ctx, tempPath, options); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("restrict backup file permissions: %w", err)
	}
	if err := os.Rename(tempPath, fullPath); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("finalize backup file: %w", err)
	}
	return fullPath, nil
}

func (w *PostgresWriter) dumpDatabase(ctx context.Context, outputPath string, options Options) error {
	connectionArgs, env, err := postgresConnectionArgs(w.databaseURL)
	if err != nil {
		return err
	}
	args := []string{"--format=custom", "--no-owner", "--no-privileges", "--file", outputPath}
	for _, table := range postgresExcludedTableData(options) {
		args = append(args, "--exclude-table-data", table)
	}
	args = append(args, connectionArgs...)
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run pg_dump: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func RestorePostgresDatabase(ctx context.Context, databaseURL string, dumpPath string, options Options) error {
	if strings.TrimSpace(databaseURL) == "" {
		return fmt.Errorf("postgres database URL is required")
	}
	if strings.TrimSpace(dumpPath) == "" {
		return fmt.Errorf("postgres backup path is required")
	}
	if _, err := os.Stat(dumpPath); err != nil {
		return fmt.Errorf("check postgres backup file: %w", err)
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("pg_restore is required for postgres restores: %w", err)
	}
	if _, err := exec.LookPath("psql"); err != nil {
		return fmt.Errorf("psql is required for postgres restores: %w", err)
	}
	restoreSQL, err := postgresRestoreDataSQL(ctx, dumpPath, options)
	if err != nil {
		return err
	}
	if err := runPostgresRestoreSQL(ctx, databaseURL, options, restoreSQL); err != nil {
		return err
	}
	return nil
}

func postgresRestoreDataSQL(ctx context.Context, dumpPath string, options Options) ([]byte, error) {
	args := postgresRestoreSQLCommandArgs(dumpPath, options)
	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = os.Environ()
	restoreSQL, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("run pg_restore: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("run pg_restore: %w", err)
	}
	return restoreSQL, nil
}

func runPostgresRestoreSQL(ctx context.Context, databaseURL string, options Options, restoreSQL []byte) error {
	connectionArgs, env, err := postgresConnectionArgs(databaseURL)
	if err != nil {
		return err
	}
	var input bytes.Buffer
	writePostgresRestoreTransactionInput(&input, options, restoreSQL)
	args := []string{"--single-transaction", "--set", "ON_ERROR_STOP=1"}
	args = append(args, connectionArgs...)
	cmd := exec.CommandContext(ctx, "psql", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = &input
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run psql restore transaction: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func postgresRestoreSQLCommandArgs(dumpPath string, options Options) []string {
	options = options.withDefaults()
	args := []string{"--data-only", "--no-owner", "--no-privileges", "--file", "-"}
	for _, table := range PostgresTablesForOptions(options) {
		args = append(args, "--table", table)
	}
	args = append(args, dumpPath)
	return args
}

func writePostgresRestoreTransactionInput(input *bytes.Buffer, options Options, restoreSQL []byte) {
	for _, table := range PostgresTablesForOptions(options) {
		input.WriteString("TRUNCATE TABLE ")
		input.WriteString(quotePostgresIdentifier(table))
		input.WriteString(" RESTART IDENTITY CASCADE;\n")
	}
	input.Write(restoreSQL)
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func postgresConnectionArgs(databaseURL string) ([]string, []string, error) {
	parsed, err := url.Parse(strings.TrimSpace(databaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, nil, fmt.Errorf("postgres database URL must be a URL-style DSN")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return nil, nil, fmt.Errorf("postgres database URL scheme must be postgres or postgresql")
	}
	env := []string{}
	if parsed.User != nil {
		username := parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			env = append(env, "PGPASSWORD="+password)
		}
		if username != "" {
			parsed.User = url.User(username)
		}
	}
	return []string{"--dbname", parsed.String()}, env, nil
}

func PostgresTablesForOptions(options Options) []string {
	options = options.withDefaults()
	tables := []string{}
	if options.RequestLogs {
		tables = append(tables, "usage_request_details")
	}
	if options.UsageIdentities {
		tables = append(tables, "usage_identities")
	}
	if options.UsageLogs {
		tables = append(tables,
			"usage_events",
			"usage_overview_hourly_stats",
			"usage_overview_daily_stats",
			"usage_overview_health_stats",
			"usage_overview_aggregation_checkpoints",
		)
	}
	if options.APIKeys {
		tables = append(tables, "cpa_api_keys")
	}
	if options.RedisInbox {
		tables = append(tables, "redis_usage_inboxes")
	}
	if options.ModelPrices {
		tables = append(tables, "model_price_settings")
	}
	return tables
}

func postgresExcludedTableData(options Options) []string {
	options = options.withDefaults()
	excluded := []string{}
	if !options.RequestLogs {
		excluded = append(excluded, "usage_request_details")
	}
	if !options.UsageIdentities {
		excluded = append(excluded, "usage_identities")
	}
	if !options.UsageLogs {
		excluded = append(excluded,
			"usage_events",
			"usage_overview_hourly_stats",
			"usage_overview_daily_stats",
			"usage_overview_health_stats",
			"usage_overview_aggregation_checkpoints",
		)
	}
	if !options.APIKeys {
		excluded = append(excluded, "cpa_api_keys")
	}
	if !options.RedisInbox {
		excluded = append(excluded, "redis_usage_inboxes")
	}
	if !options.ModelPrices {
		excluded = append(excluded, "model_price_settings")
	}
	return excluded
}
