// DEV_USAGE_TAB_VALUE 标记 dev 分支独有的监控中心 tab，避免直接改 main 的基础 tab 列表。
export const DEV_USAGE_TAB_VALUE = 'monitoring' as const;
export type DevUsageTab = typeof DEV_USAGE_TAB_VALUE;

// DEV_USAGE_TAB_ORDER 保持 dev 现有展示顺序，同时让 main 的基础 tab 列表可独立合并。
const DEV_USAGE_TAB_ORDER = ['overview', DEV_USAGE_TAB_VALUE, 'analysis', 'events', 'credentials', 'storage', 'settings'] as const;

export const withDevUsageTabs = <TTab extends string>(options: ReadonlyArray<TTab>): Array<TTab | DevUsageTab> => {
  const remaining = new Set<TTab | DevUsageTab>([...options, DEV_USAGE_TAB_VALUE]);
  const ordered: Array<TTab | DevUsageTab> = [];

  for (const value of DEV_USAGE_TAB_ORDER) {
    if (!remaining.has(value as TTab | DevUsageTab)) {
      continue;
    }
    ordered.push(value as TTab | DevUsageTab);
    remaining.delete(value as TTab | DevUsageTab);
  }

  return [...ordered, ...remaining];
};

export const devUsageTabLabelKey = <TTab extends string>(value: TTab | DevUsageTab, baseLabelKeys: Readonly<Record<TTab, string>>): string => (
  value === DEV_USAGE_TAB_VALUE ? 'usage_stats.tab_monitoring' : baseLabelKeys[value as TTab]
);
