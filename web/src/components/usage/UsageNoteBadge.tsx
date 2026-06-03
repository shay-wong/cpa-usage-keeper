import type { ReactNode } from 'react';
import styles from './UsageNoteBadge.module.scss';

export function UsageNoteBadge({ children, className, title }: { children: ReactNode; className?: string; title?: string }) {
  return (
    <span className={`${styles.usageNoteBadge} ${className ?? ''}`.trim()} title={title}>
      {children}
    </span>
  );
}
