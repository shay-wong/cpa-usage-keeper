import type { ReactElement } from 'react';

import { ApiError, triggerSync } from '@/lib/api';
import type { StatusResponse } from '@/lib/types';

import styles from './UsagePage.module.scss';

type SyncCpaDataOptions = {
  refreshActiveTab: () => Promise<void>;
  refreshStatus: () => Promise<StatusResponse>;
  onStatus: (status: StatusResponse) => void;
};

export const syncCpaData = async ({ refreshActiveTab, refreshStatus, onStatus }: SyncCpaDataOptions): Promise<void> => {
  try {
    await triggerSync();
    await refreshActiveTab();
    const nextStatus = await refreshStatus();
    onStatus(nextStatus);
  } catch (error) {
    if (!(error instanceof ApiError && error.status === 401)) {
      try {
        const nextStatus = await refreshStatus();
        onStatus(nextStatus);
      } catch {
        // 忽略状态刷新失败，继续抛出原始同步错误。
      }
    }
    throw error;
  }
};

type SyncNowButtonProps = {
  loading: boolean;
  disabled: boolean;
  label: string;
  loadingLabel: string;
  ariaLabel: string;
  onClick: () => void;
};

export function SyncNowButton({ loading, disabled, label, loadingLabel, ariaLabel, onClick }: SyncNowButtonProps): ReactElement {
  return (
    <div className={styles.syncSwitcher} role="group" aria-label={ariaLabel}>
      <button
        type="button"
        className={styles.syncPill}
        onClick={onClick}
        disabled={disabled}
        title={ariaLabel}
      >
        {loading ? loadingLabel : label}
      </button>
    </div>
  );
}
