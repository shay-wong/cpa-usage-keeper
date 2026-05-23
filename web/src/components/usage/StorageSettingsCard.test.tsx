import '@/i18n';
import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { StorageSettingsCard } from './StorageSettingsCard';

const storageInfo = {
  settings: {
    record_request_details: true,
    cleanup_request_logs: true,
    cleanup_usage_logs: false,
    request_log_retention_days: 7,
    usage_log_retention_days: 30,
    max_database_size_mb: 512,
    backup_request_logs: false,
    backup_usage_logs: true,
    backup_usage_identities: true,
    backup_api_keys: true,
    backup_redis_inbox: false,
    backup_model_prices: true,
    backup_hour: 4,
    backup_minute: 0,
    max_backup_count: 1,
  },
  current_database_size_bytes: 3145728,
  backup_total_size_bytes: 1048576,
  backup_count: 1,
  domains: [{ key: 'usage_logs', label: '用量日志', description: '用量事件和统计缓存。', rows: 3, size_bytes: 4096, table_names: ['usage_events'] }],
  backups: [{ id: '2026-05-24/database_20260524_040000', file_name: 'database_20260524_040000.db', size_bytes: 1048576 }],
};

describe('StorageSettingsCard', () => {
  it('renders storage sections as separate cards and boolean controls as switches', () => {
    const html = renderToStaticMarkup(
      <StorageSettingsCard
        info={storageInfo}
        onSave={() => undefined}
        onCreateBackup={() => undefined}
        onRestoreBackup={() => undefined}
      />,
    );

    expect(html.match(/databaseCleanupSettingsCard/g)?.length).toBe(4);
    expect(html).toContain('Storage Overview');
    expect(html).toContain('Recording &amp; Cleanup');
    expect(html).toContain('Backup Settings');
    expect(html).toContain('Restore Backup');
    expect(html.match(/role="switch"/g)?.length).toBe(16);
    expect(html).toContain('type="time"');
    expect(html).toContain('API Key');
    expect(html).toContain('Redis inbox cache');
    expect(html).toContain('Model prices');
    expect(html).toContain('Skip safety backup');
    expect(html).toContain('storageTableList');
    expect(html).toContain('storageSelectControl');
  });
});
