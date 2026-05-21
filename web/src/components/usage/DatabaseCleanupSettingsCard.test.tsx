import '@/i18n';
import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { DatabaseCleanupSettingsCard } from './DatabaseCleanupSettingsCard';

describe('DatabaseCleanupSettingsCard', () => {
  it('renders database cleanup settings and disabled hints', () => {
    const html = renderToStaticMarkup(
      <DatabaseCleanupSettingsCard
        settings={{ request_log_retention_days: 0, max_database_size_mb: 0, current_database_size_bytes: 3145728 }}
        loading={false}
        saving={false}
        onSave={() => undefined}
      />,
    );

    expect(html).toContain('Database Cleanup');
    expect(html).toContain('Request log detail retention');
    expect(html).toContain('Maximum database size (MB)');
    expect(html).toContain('Current database size');
    expect(html).toContain('3.0 MB');
    expect(html).toContain('0 disables cleanup');
  });
});
