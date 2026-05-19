import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import type { DatabaseCleanupSettingsResponse, UpdateDatabaseCleanupSettingsRequest } from '@/lib/types';
import styles from '@/pages/UsagePage.module.scss';

interface DatabaseCleanupSettingsTitleProps {
  title: string;
  subtitle: string;
  eyebrow: string;
}

function DatabaseCleanupSettingsTitle({ title, subtitle, eyebrow }: DatabaseCleanupSettingsTitleProps) {
  return (
    <div className={styles.sectionTitleBlock}>
      <span className={styles.sectionEyebrow}>{eyebrow}</span>
      <h3 className={styles.sectionTitle}>{title}</h3>
      <p className={styles.sectionSubtitle}>{subtitle}</p>
    </div>
  );
}

export interface DatabaseCleanupSettingsCardProps {
  settings: DatabaseCleanupSettingsResponse | null;
  loading?: boolean;
  saving?: boolean;
  onSave: (settings: UpdateDatabaseCleanupSettingsRequest) => void | Promise<void>;
}

const toDraftValue = (value: number | undefined): string => String(Math.max(0, Math.floor(value ?? 0)));
const parseNonNegativeInteger = (value: string): number | null => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) return null;
  return Math.floor(parsed);
};

function formatDatabaseSize(bytes: number | undefined): string {
  if (!Number.isFinite(bytes) || (bytes ?? 0) < 0) return '-';
  const value = bytes ?? 0;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

export function DatabaseCleanupSettingsCard({ settings, loading = false, saving = false, onSave }: DatabaseCleanupSettingsCardProps) {
  const { t } = useTranslation();
  const formKey = `${settings?.request_log_retention_days ?? 0}:${settings?.max_database_size_mb ?? 0}:${settings?.current_database_size_bytes ?? 0}`;

  return (
    <Card
      title={
        <DatabaseCleanupSettingsTitle
          eyebrow={t('usage_stats.database_cleanup_eyebrow')}
          title={t('usage_stats.database_cleanup_title')}
          subtitle={t('usage_stats.database_cleanup_subtitle')}
        />
      }
      className={`${styles.detailsFixedCard} ${styles.databaseCleanupSettingsCard}`}
    >
      <div className={styles.databaseCleanupSettingsBody}>
        {loading && !settings ? (
          <div className={styles.hint}>{t('common.loading')}</div>
        ) : (
          <DatabaseCleanupSettingsForm key={formKey} settings={settings} saving={saving} onSave={onSave} />
        )}
      </div>
    </Card>
  );
}

interface DatabaseCleanupSettingsFormProps {
  settings: DatabaseCleanupSettingsResponse | null;
  saving: boolean;
  onSave: (settings: UpdateDatabaseCleanupSettingsRequest) => void | Promise<void>;
}

function DatabaseCleanupSettingsForm({ settings, saving, onSave }: DatabaseCleanupSettingsFormProps) {
  const { t } = useTranslation();
  const [retentionDays, setRetentionDays] = useState(toDraftValue(settings?.request_log_retention_days));
  const [maxDatabaseSizeMB, setMaxDatabaseSizeMB] = useState(toDraftValue(settings?.max_database_size_mb));
  const [error, setError] = useState('');

  const handleSave = () => {
    const nextRetentionDays = parseNonNegativeInteger(retentionDays);
    const nextMaxDatabaseSizeMB = parseNonNegativeInteger(maxDatabaseSizeMB);
    if (nextRetentionDays === null || nextMaxDatabaseSizeMB === null) {
      setError(t('usage_stats.database_cleanup_invalid'));
      return;
    }
    setError('');
    void onSave({
      request_log_retention_days: nextRetentionDays,
      max_database_size_mb: nextMaxDatabaseSizeMB,
    });
  };

  return (
    <>
      <div className={styles.databaseCleanupSettingsField}>
        <span>{t('usage_stats.database_cleanup_current_size_label')}</span>
        <strong className={styles.databaseCleanupSettingsValue}>{formatDatabaseSize(settings?.current_database_size_bytes)}</strong>
        <small>{t('usage_stats.database_cleanup_current_size_hint')}</small>
      </div>
      <div className={styles.databaseCleanupSettingsGrid}>
        <label className={styles.databaseCleanupSettingsField}>
          <span>{t('usage_stats.database_cleanup_retention_days')}</span>
          <Input
            type="number"
            min="0"
            step="1"
            value={retentionDays}
            onChange={(event) => setRetentionDays(event.target.value)}
            className={styles.usagePillControl}
            disabled={saving}
          />
          <small>{t('usage_stats.database_cleanup_retention_hint')}</small>
        </label>
        <label className={styles.databaseCleanupSettingsField}>
          <span>{t('usage_stats.database_cleanup_max_size_mb')}</span>
          <Input
            type="number"
            min="0"
            step="1"
            value={maxDatabaseSizeMB}
            onChange={(event) => setMaxDatabaseSizeMB(event.target.value)}
            className={styles.usagePillControl}
            disabled={saving}
          />
          <small>{t('usage_stats.database_cleanup_size_hint')}</small>
        </label>
      </div>
      {error && <div className={styles.errorBox}>{error}</div>}
      <div className={styles.databaseCleanupSettingsActions}>
        <Button variant="primary" className={styles.usagePillAction} onClick={handleSave} disabled={saving}>
          {saving ? t('usage_stats.database_cleanup_saving') : t('common.save')}
        </Button>
      </div>
    </>
  );
}
