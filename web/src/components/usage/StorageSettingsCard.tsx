import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import type { BackupFileInfo, CreateBackupRequest, RestoreBackupRequest, StorageInfoResponse, UpdateDatabaseCleanupSettingsRequest } from '@/lib/types';
import styles from '@/pages/UsagePage.module.scss';

type StorageOperationScope = 'cleanup' | 'backup' | 'restore';

type StorageActionState = Exclude<StorageOperationScope, 'cleanup'> | null;

export interface StorageOperationNotice {
  scope: StorageOperationScope;
  kind: 'success' | 'error';
  message: string;
}

interface StorageSettingsCardProps {
  info: StorageInfoResponse | null;
  loading?: boolean;
  saving?: boolean;
  actionLoading?: boolean;
  actionState?: StorageActionState;
  notice?: StorageOperationNotice | null;
  onSave: (settings: UpdateDatabaseCleanupSettingsRequest, scope: Exclude<StorageOperationScope, 'restore'>) => void | Promise<void>;
  onCreateBackup: (request: CreateBackupRequest) => void | Promise<void>;
  onRestoreBackup: (request: RestoreBackupRequest) => void | Promise<void>;
}

interface StorageSwitchProps {
  label: string;
  hint: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}

type StorageDomainValues = {
  request_logs: boolean;
  usage_logs: boolean;
  usage_identities: boolean;
  api_keys: boolean;
  redis_inbox: boolean;
  model_prices: boolean;
};

type StorageDomainKey = keyof StorageDomainValues;

interface StorageSettingsDraft {
  recordRequestDetails: boolean;
  cleanupRequestLogs: boolean;
  cleanupUsageLogs: boolean;
  requestLogRetentionDays: string;
  usageLogRetentionDays: string;
  maxDatabaseSizeMB: string;
  backupTime: string;
  maxBackupCount: string;
  backupDomains: StorageDomainValues;
  restoreBackupId: string;
  restoreDomains: StorageDomainValues;
  skipSafetyBackup: boolean;
}

const defaultSettings: UpdateDatabaseCleanupSettingsRequest = {
  record_request_details: true,
  cleanup_request_logs: true,
  cleanup_usage_logs: false,
  request_log_retention_days: 0,
  usage_log_retention_days: 0,
  max_database_size_mb: 0,
  backup_request_logs: false,
  backup_usage_logs: true,
  backup_usage_identities: true,
  backup_api_keys: true,
  backup_redis_inbox: false,
  backup_model_prices: true,
  backup_hour: 4,
  backup_minute: 0,
  max_backup_count: 1,
};

const storageDomainSwitches: Array<{ key: StorageDomainKey; labelKey: string; hintKey: string }> = [
  { key: 'request_logs', labelKey: 'usage_stats.storage_domain_request_logs_label', hintKey: 'usage_stats.storage_domain_request_logs_hint' },
  { key: 'usage_logs', labelKey: 'usage_stats.storage_domain_usage_logs_label', hintKey: 'usage_stats.storage_domain_usage_logs_hint' },
  { key: 'usage_identities', labelKey: 'usage_stats.storage_domain_usage_identities_label', hintKey: 'usage_stats.storage_domain_usage_identities_hint' },
  { key: 'api_keys', labelKey: 'usage_stats.storage_domain_api_keys_label', hintKey: 'usage_stats.storage_domain_api_keys_hint' },
  { key: 'redis_inbox', labelKey: 'usage_stats.storage_domain_redis_inbox_label', hintKey: 'usage_stats.storage_domain_redis_inbox_hint' },
  { key: 'model_prices', labelKey: 'usage_stats.storage_domain_model_prices_label', hintKey: 'usage_stats.storage_domain_model_prices_hint' },
];

const toDraftValue = (value: number | undefined): string => String(Math.max(0, Math.floor(value ?? 0)));
const toBackupTimeDraft = (hour: number | undefined, minute: number | undefined): string => `${String(Math.max(0, Math.min(23, Math.floor(hour ?? 0)))).padStart(2, '0')}:${String(Math.max(0, Math.min(59, Math.floor(minute ?? 0)))).padStart(2, '0')}`;
const parseBackupTime = (value: string): { hour: number; minute: number } | null => {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || hour > 23 || minute > 59) return null;
  return { hour, minute };
};
const parseNonNegativeInteger = (value: string): number | null => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) return null;
  return Math.floor(parsed);
};

function createDraft(settings: UpdateDatabaseCleanupSettingsRequest): StorageSettingsDraft {
  return {
    recordRequestDetails: settings.record_request_details ?? true,
    cleanupRequestLogs: settings.cleanup_request_logs ?? true,
    cleanupUsageLogs: settings.cleanup_usage_logs ?? false,
    requestLogRetentionDays: toDraftValue(settings.request_log_retention_days),
    usageLogRetentionDays: toDraftValue(settings.usage_log_retention_days),
    maxDatabaseSizeMB: toDraftValue(settings.max_database_size_mb),
    backupTime: toBackupTimeDraft(settings.backup_hour, settings.backup_minute),
    maxBackupCount: toDraftValue(settings.max_backup_count),
    backupDomains: {
      request_logs: settings.backup_request_logs ?? false,
      usage_logs: settings.backup_usage_logs ?? true,
      usage_identities: settings.backup_usage_identities ?? true,
      api_keys: settings.backup_api_keys ?? true,
      redis_inbox: settings.backup_redis_inbox ?? false,
      model_prices: settings.backup_model_prices ?? true,
    },
    restoreBackupId: '',
    restoreDomains: {
      request_logs: false,
      usage_logs: true,
      usage_identities: true,
      api_keys: true,
      redis_inbox: false,
      model_prices: true,
    },
    skipSafetyBackup: false,
  };
}

function formatBytes(bytes: number | undefined): string {
  if (!Number.isFinite(bytes) || (bytes ?? 0) < 0) return '-';
  const value = bytes ?? 0;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function normalizeBackup(item: BackupFileInfo) {
  return {
    id: item.id ?? item.ID ?? '',
    fileName: item.file_name ?? item.FileName ?? '',
    sizeBytes: item.size_bytes ?? item.SizeBytes ?? 0,
  };
}

function storageTitle(eyebrow: string, title: string, subtitle: string) {
  return (
    <div className={`${styles.sectionTitleBlock} ${styles.storageSectionTitleBlock}`.trim()}>
      <span className={styles.sectionEyebrow}>{eyebrow}</span>
      <h3 className={styles.sectionTitle}>{title}</h3>
      <p className={styles.sectionSubtitle}>{subtitle}</p>
    </div>
  );
}

function StorageSwitch({ label, hint, checked, disabled = false, onChange }: StorageSwitchProps) {
  return (
    <label className={styles.storageSwitchField} data-checked={checked ? 'true' : 'false'}>
      <span className={styles.storageSwitchCopy}>
        <span className={styles.storageSwitchLabel}>{label}</span>
        <small>{hint}</small>
      </span>
      <input
        type="checkbox"
        role="switch"
        checked={checked}
        disabled={disabled}
        className={styles.storageSwitchInput}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className={styles.storageSwitchTrack} aria-hidden="true">
        <span className={styles.storageSwitchThumb} />
      </span>
    </label>
  );
}

interface StorageDomainSwitchesProps {
  values: StorageDomainValues;
  disabled: boolean;
  onChange: (key: StorageDomainKey, checked: boolean) => void;
}

function StorageDomainSwitches({ values, disabled, onChange }: StorageDomainSwitchesProps) {
  const { t } = useTranslation();

  return (
    <div className={styles.storageDomainSwitchGrid}>
      {storageDomainSwitches.map((item) => (
        <StorageSwitch key={item.key} label={t(item.labelKey)} hint={t(item.hintKey)} checked={values[item.key]} disabled={disabled} onChange={(checked) => onChange(item.key, checked)} />
      ))}
    </div>
  );
}

function domainValuesSelected(values: StorageDomainValues): boolean {
  return Object.values(values).some(Boolean);
}

function tableList(tables: string[], label: string) {
  return (
    <div className={styles.storageTableListBlock}>
      <span className={styles.storageTableListLabel}>{label}</span>
      <ul className={styles.storageTableList}>
        {tables.map((table) => <li key={table}>{table}</li>)}
      </ul>
    </div>
  );
}

function StorageNotice({ notice, scope }: { notice: StorageOperationNotice | null; scope: StorageOperationScope }) {
  if (!notice || notice.scope !== scope) return null;
  const className = notice.kind === 'success' ? styles.successBox : styles.errorBox;
  return <div className={className}>{notice.message}</div>;
}

function StorageProgress({ message }: { message: string }) {
  return (
    <div className={styles.storageProgress} role="status" aria-live="polite">
      <span className={styles.storageProgressBar} aria-hidden="true"><span /></span>
      <span>{message}</span>
    </div>
  );
}

export function StorageSettingsCard({ info, loading = false, saving = false, actionLoading = false, actionState = null, notice = null, onSave, onCreateBackup, onRestoreBackup }: StorageSettingsCardProps) {
  const { t } = useTranslation();
  const settings = info?.settings ?? info?.Settings ?? defaultSettings;
  const [draft, setDraft] = useState(() => createDraft(settings));
  const [formError, setFormError] = useState<StorageOperationNotice | null>(null);

  const domains = info?.domains ?? info?.Domains ?? [];
  const backups = useMemo(() => (info?.backups ?? info?.Backups ?? []).map(normalizeBackup), [info]);
  const currentDatabaseSizeBytes = info?.current_database_size_bytes ?? info?.CurrentDatabaseSizeBytes;
  const backupTotalSizeBytes = info?.backup_total_size_bytes ?? info?.BackupTotalSizeBytes;
  const backupCount = info?.backup_count ?? info?.BackupCount ?? backups.length;
  const databaseBackupsSupported = info?.database_backups_supported ?? info?.DatabaseBackupsSupported ?? false;
  const backupRunning = actionLoading && actionState === 'backup';
  const restoreRunning = actionLoading && actionState === 'restore';
  const busy = saving || actionLoading;
  const moduleNotice = (scope: StorageOperationScope) => (formError?.scope === scope ? formError : notice);

  const setDraftValue = <K extends keyof StorageSettingsDraft>(key: K, value: StorageSettingsDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };
  const setBackupDomain = (key: StorageDomainKey, checked: boolean) => {
    setDraft((current) => ({
      ...current,
      backupDomains: { ...current.backupDomains, [key]: checked },
    }));
  };
  const setRestoreDomain = (key: StorageDomainKey, checked: boolean) => {
    setDraft((current) => ({
      ...current,
      restoreDomains: { ...current.restoreDomains, [key]: checked },
    }));
  };

  const buildSettingsRequest = (scope: Exclude<StorageOperationScope, 'restore'>) => {
    const nextRequestLogRetentionDays = parseNonNegativeInteger(draft.requestLogRetentionDays);
    const nextUsageLogRetentionDays = parseNonNegativeInteger(draft.usageLogRetentionDays);
    const nextMaxDatabaseSizeMB = parseNonNegativeInteger(draft.maxDatabaseSizeMB);
    const nextBackupTime = parseBackupTime(draft.backupTime);
    const nextMaxBackupCount = parseNonNegativeInteger(draft.maxBackupCount);
    if ([nextRequestLogRetentionDays, nextUsageLogRetentionDays, nextMaxDatabaseSizeMB, nextMaxBackupCount].some((value) => value === null)) {
      setFormError({ scope, kind: 'error', message: t('usage_stats.storage_error_non_negative_integer') });
      return null;
    }
    if (!nextBackupTime) {
      setFormError({ scope, kind: 'error', message: t('usage_stats.storage_error_backup_time') });
      return null;
    }
    return {
      record_request_details: draft.recordRequestDetails,
      cleanup_request_logs: draft.cleanupRequestLogs,
      cleanup_usage_logs: draft.cleanupUsageLogs,
      request_log_retention_days: nextRequestLogRetentionDays ?? 0,
      usage_log_retention_days: nextUsageLogRetentionDays ?? 0,
      max_database_size_mb: nextMaxDatabaseSizeMB ?? 0,
      backup_request_logs: draft.backupDomains.request_logs,
      backup_usage_logs: draft.backupDomains.usage_logs,
      backup_usage_identities: draft.backupDomains.usage_identities,
      backup_api_keys: draft.backupDomains.api_keys,
      backup_redis_inbox: draft.backupDomains.redis_inbox,
      backup_model_prices: draft.backupDomains.model_prices,
      backup_hour: nextBackupTime.hour,
      backup_minute: nextBackupTime.minute,
      max_backup_count: nextMaxBackupCount ?? 0,
    };
  };

  const handleSave = (scope: Exclude<StorageOperationScope, 'restore'>) => {
    const request = buildSettingsRequest(scope);
    if (!request) return;
    setFormError(null);
    void onSave(request, scope);
  };

  const handleCreateBackup = () => {
    if (!databaseBackupsSupported) {
      setFormError({ scope: 'backup', kind: 'error', message: t('usage_stats.storage_backup_unavailable') });
      return;
    }
    if (!domainValuesSelected(draft.backupDomains)) {
      setFormError({ scope: 'backup', kind: 'error', message: t('usage_stats.storage_error_backup_domain') });
      return;
    }
    setFormError(null);
    void onCreateBackup(draft.backupDomains);
  };

  const handleRestore = () => {
    if (!databaseBackupsSupported) {
      setFormError({ scope: 'restore', kind: 'error', message: t('usage_stats.storage_restore_unavailable') });
      return;
    }
    if (!draft.restoreBackupId) {
      setFormError({ scope: 'restore', kind: 'error', message: t('usage_stats.storage_error_restore_backup') });
      return;
    }
    if (!domainValuesSelected(draft.restoreDomains)) {
      setFormError({ scope: 'restore', kind: 'error', message: t('usage_stats.storage_error_restore_domain') });
      return;
    }
    if (draft.skipSafetyBackup && typeof window !== 'undefined' && !window.confirm(t('usage_stats.storage_restore_skip_confirm'))) {
      return;
    }
    setFormError(null);
    void onRestoreBackup({ id: draft.restoreBackupId, ...draft.restoreDomains, skip_safety_backup: draft.skipSafetyBackup });
  };

  if (loading && !info) {
    return (
      <div className={styles.storageSettingsCards}>
        <Card title={storageTitle(t('usage_stats.storage_overview_eyebrow'), t('usage_stats.storage_page_title'), t('usage_stats.storage_page_subtitle'))} className={`${styles.detailsFixedCard} ${styles.databaseCleanupSettingsCard}`}>
          <div className={styles.hint}>{t('common.loading')}</div>
        </Card>
      </div>
    );
  }

  return (
    <div className={styles.storageSettingsCards}>
      <Card
        title={storageTitle(t('usage_stats.storage_overview_eyebrow'), t('usage_stats.storage_overview_title'), t('usage_stats.storage_overview_subtitle'))}
        className={`${styles.detailsFixedCard} ${styles.databaseCleanupSettingsCard}`}
      >
        <div className={styles.databaseCleanupSettingsBody}>
          <div className={styles.databaseCleanupSettingsGrid}>
            <div className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_current_database_size_label')}</span><strong className={styles.databaseCleanupSettingsValue}>{formatBytes(currentDatabaseSizeBytes)}</strong><small>{t('usage_stats.storage_current_database_size_hint')}</small></div>
            <div className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_backup_size_label')}</span><strong className={styles.databaseCleanupSettingsValue}>{formatBytes(backupTotalSizeBytes)}</strong><small>{t('usage_stats.storage_backup_count_hint', { count: backupCount })}</small></div>
          </div>
          <div className={styles.databaseCleanupSettingsGrid}>
            {domains.map((domain) => {
              const key = domain.key ?? domain.Key ?? '';
              const label = domain.label ?? domain.Label ?? key;
              const description = domain.description ?? domain.Description ?? '';
              const rows = domain.rows ?? domain.Rows ?? 0;
              const sizeBytes = domain.size_bytes ?? domain.SizeBytes ?? 0;
              const tables = domain.table_names ?? domain.TableNames ?? [];
              return <div key={key} className={styles.databaseCleanupSettingsField}><span>{label}</span><strong className={styles.databaseCleanupSettingsValue}>{t('usage_stats.storage_rows_size_summary', { rows: rows.toLocaleString(), size: formatBytes(sizeBytes) })}</strong><small>{description}</small>{tableList(tables, t('usage_stats.storage_table_list_label'))}</div>;
            })}
          </div>
        </div>
      </Card>

      <Card
        title={storageTitle(t('usage_stats.storage_cleanup_eyebrow'), t('usage_stats.storage_cleanup_title'), t('usage_stats.storage_cleanup_subtitle'))}
        className={`${styles.detailsFixedCard} ${styles.databaseCleanupSettingsCard}`}
      >
        <div className={styles.databaseCleanupSettingsBody}>
          <div className={styles.databaseCleanupSettingsGrid}>
            <StorageSwitch label={t('usage_stats.storage_record_request_details_label')} hint={t('usage_stats.storage_record_request_details_hint')} checked={draft.recordRequestDetails} disabled={saving} onChange={(checked) => setDraftValue('recordRequestDetails', checked)} />
            <StorageSwitch label={t('usage_stats.storage_cleanup_request_logs_label')} hint={t('usage_stats.storage_cleanup_request_logs_hint')} checked={draft.cleanupRequestLogs} disabled={saving} onChange={(checked) => setDraftValue('cleanupRequestLogs', checked)} />
            <StorageSwitch label={t('usage_stats.storage_cleanup_usage_logs_label')} hint={t('usage_stats.storage_cleanup_usage_logs_hint')} checked={draft.cleanupUsageLogs} disabled={saving} onChange={(checked) => setDraftValue('cleanupUsageLogs', checked)} />
          </div>
          <div className={styles.databaseCleanupSettingsGrid}>
            <label className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_request_log_retention_label')}</span><Input type="number" min="0" step="1" value={draft.requestLogRetentionDays} onChange={(event) => setDraftValue('requestLogRetentionDays', event.target.value)} className={styles.usagePillControl} disabled={saving || !draft.cleanupRequestLogs} /><small>{draft.cleanupRequestLogs ? t('usage_stats.storage_request_log_retention_hint') : t('usage_stats.storage_retention_disabled_hint')}</small></label>
            <label className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_usage_log_retention_label')}</span><Input type="number" min="0" step="1" value={draft.usageLogRetentionDays} onChange={(event) => setDraftValue('usageLogRetentionDays', event.target.value)} className={styles.usagePillControl} disabled={saving || !draft.cleanupUsageLogs} /><small>{draft.cleanupUsageLogs ? t('usage_stats.storage_usage_log_retention_hint') : t('usage_stats.storage_retention_disabled_hint')}</small></label>
            <label className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_max_database_size_label')}</span><Input type="number" min="0" step="1" value={draft.maxDatabaseSizeMB} onChange={(event) => setDraftValue('maxDatabaseSizeMB', event.target.value)} className={styles.usagePillControl} disabled={saving} /><small>{t('usage_stats.storage_max_database_size_hint')}</small></label>
          </div>
          <StorageNotice notice={moduleNotice('cleanup')} scope="cleanup" />
          <div className={styles.databaseCleanupSettingsActions}>
            <Button variant="primary" className={styles.usagePillAction} onClick={() => handleSave('cleanup')} disabled={saving}>{saving ? t('usage_stats.database_cleanup_saving') : t('usage_stats.storage_save_cleanup_settings')}</Button>
          </div>
        </div>
      </Card>

      <Card
        title={storageTitle(t('usage_stats.storage_backup_eyebrow'), t('usage_stats.storage_backup_title'), t('usage_stats.storage_backup_subtitle'))}
        className={`${styles.detailsFixedCard} ${styles.databaseCleanupSettingsCard}`}
      >
        <div className={styles.databaseCleanupSettingsBody}>
          <StorageDomainSwitches values={draft.backupDomains} disabled={busy || !databaseBackupsSupported} onChange={setBackupDomain} />
          <div className={styles.databaseCleanupSettingsGrid}>
            <label className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_max_backup_count_label')}</span><Input type="number" min="0" step="1" value={draft.maxBackupCount} onChange={(event) => setDraftValue('maxBackupCount', event.target.value)} className={styles.usagePillControl} disabled={saving} /><small>{t('usage_stats.storage_max_backup_count_hint')}</small></label>
            <label className={styles.databaseCleanupSettingsField}><span>{t('usage_stats.storage_backup_time_label')}</span><Input type="time" step="60" value={draft.backupTime} onChange={(event) => setDraftValue('backupTime', event.target.value)} className={styles.usagePillControl} disabled={saving} /><small>{t('usage_stats.storage_backup_time_hint')}</small></label>
          </div>
          {!databaseBackupsSupported ? <div className={styles.hint}>{t('usage_stats.storage_backup_unavailable')}</div> : null}
          {backupRunning ? <StorageProgress message={t('usage_stats.storage_backup_running')} /> : null}
          <StorageNotice notice={moduleNotice('backup')} scope="backup" />
          <div className={styles.databaseCleanupSettingsActions}>
            <Button variant="primary" className={styles.usagePillAction} onClick={() => handleSave('backup')} disabled={busy}>{saving ? t('usage_stats.database_cleanup_saving') : t('usage_stats.storage_save_backup_settings')}</Button>
            <Button variant="secondary" className={styles.usagePillAction} onClick={handleCreateBackup} disabled={actionLoading || !databaseBackupsSupported}>{backupRunning ? t('usage_stats.storage_backup_running_button') : t('usage_stats.storage_create_backup')}</Button>
          </div>
        </div>
      </Card>

      <Card
        title={storageTitle(t('usage_stats.storage_restore_eyebrow'), t('usage_stats.storage_restore_title'), t('usage_stats.storage_restore_subtitle'))}
        className={`${styles.detailsFixedCard} ${styles.databaseCleanupSettingsCard}`}
      >
        <div className={styles.databaseCleanupSettingsBody}>
          <div className={styles.databaseCleanupSettingsGrid}>
            <label className={styles.databaseCleanupSettingsField}>
              <span>{t('usage_stats.storage_restore_select_label')}</span>
              <span className={styles.storageSelectShell}>
                <select value={draft.restoreBackupId} onChange={(event) => setDraftValue('restoreBackupId', event.target.value)} className={styles.storageSelectControl} disabled={actionLoading || !databaseBackupsSupported}>
                  <option value="">{t('usage_stats.storage_restore_select_placeholder')}</option>
                  {backups.map((backup) => <option key={backup.id} value={backup.id}>{backup.fileName} · {formatBytes(backup.sizeBytes)}</option>)}
                </select>
              </span>
              <small>{draft.skipSafetyBackup ? t('usage_stats.storage_restore_skip_hint') : t('usage_stats.storage_restore_safe_hint')}</small>
            </label>
            <StorageSwitch label={t('usage_stats.storage_skip_safety_backup_label')} hint={t('usage_stats.storage_skip_safety_backup_hint')} checked={draft.skipSafetyBackup} disabled={actionLoading || !databaseBackupsSupported} onChange={(checked) => setDraftValue('skipSafetyBackup', checked)} />
          </div>
          <StorageDomainSwitches values={draft.restoreDomains} disabled={actionLoading || !databaseBackupsSupported} onChange={setRestoreDomain} />
          {!databaseBackupsSupported ? <div className={styles.hint}>{t('usage_stats.storage_restore_unavailable')}</div> : null}
          {restoreRunning ? <StorageProgress message={t('usage_stats.storage_restore_running')} /> : null}
          <StorageNotice notice={moduleNotice('restore')} scope="restore" />
          <div className={styles.databaseCleanupSettingsActions}>
            <Button variant="primary" className={styles.usagePillAction} onClick={handleRestore} disabled={actionLoading || !databaseBackupsSupported}>{restoreRunning ? t('usage_stats.storage_restore_running_button') : t('usage_stats.storage_restore_selected')}</Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
