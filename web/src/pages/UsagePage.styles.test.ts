import { existsSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const readSource = (url: URL) =>
  readFileSync(url, 'utf8').replace(/\r\n/g, '\n');

const usagePageStyles = readSource(
  new URL('./UsagePage.module.scss', import.meta.url),
);
const monitoringCenterStyles = readSource(
  new URL(
    '../components/usage/monitoring/MonitoringCenterTab.module.scss',
    import.meta.url,
  ),
);
const monitoringCenterSource = readSource(
  new URL(
    '../components/usage/monitoring/MonitoringCenterTab.tsx',
    import.meta.url,
  ),
);
const usagePageSource = readSource(new URL('./UsagePage.tsx', import.meta.url));
const requestEventsSource = readSource(
  new URL('../components/usage/RequestEventsDetailsCard.tsx', import.meta.url),
);
const usageNoteBadgeSource = readSource(
  new URL('../components/usage/UsageNoteBadge.tsx', import.meta.url),
);
const usageNoteBadgeStyles = readSource(
  new URL('../components/usage/UsageNoteBadge.module.scss', import.meta.url),
);
const authFileCredentialsSource = readSource(
  new URL(
    '../components/usage/credentials/AuthFileCredentialsSection.tsx',
    import.meta.url,
  ),
);
const aiProviderCredentialsSource = readSource(
  new URL(
    '../components/usage/credentials/AiProviderCredentialsSection.tsx',
    import.meta.url,
  ),
);
const requestDetailJsonViewerSource = readSource(
  new URL('../components/usage/RequestDetailJsonViewer.tsx', import.meta.url),
);
const priceSettingsSource = readSource(
  new URL('../components/usage/PriceSettingsCard.tsx', import.meta.url),
);
const chartLineSelectorSource = readSource(
  new URL('../components/usage/ChartLineSelector.tsx', import.meta.url),
);
const selectSource = readSource(
  new URL('../components/ui/Select.tsx', import.meta.url),
);
const apiIndexSource = readSource(
  new URL('../components/usage/index.ts', import.meta.url),
);
const apiClientSource = readSource(new URL('../lib/api.ts', import.meta.url));
const globalStyles = readSource(new URL('../styles/global.scss', import.meta.url));
const i18nSource = readSource(new URL('../i18n/index.ts', import.meta.url));
const analysisPanelSource = readSource(
  new URL('../components/usage/analysis/AnalysisPanel.tsx', import.meta.url),
);
const analysisPanelStyles = readSource(
  new URL(
    '../components/usage/analysis/AnalysisPanel.module.scss',
    import.meta.url,
  ),
);
const usagePageDevTabsSource = readSource(
  new URL('./usagePageDevTabs.ts', import.meta.url),
);
const usageChartSource = readSource(
  new URL('../components/usage/UsageChart.tsx', import.meta.url),
);
const tokenBreakdownChartSource = readSource(
  new URL('../components/usage/TokenBreakdownChart.tsx', import.meta.url),
);
const costTrendChartSource = readSource(
  new URL('../components/usage/CostTrendChart.tsx', import.meta.url),
);
const statCardsSource = readSource(
  new URL('../components/usage/StatCards.tsx', import.meta.url),
);

const requestEventColumnDefinitionBlock = (columnId: string) => {
  const definitionsStart = requestEventsSource.indexOf('const definitions: RequestEventColumnDefinition[] = [')
  expect(definitionsStart).toBeGreaterThanOrEqual(0)
  const start = requestEventsSource.indexOf(`id: '${columnId}',`, definitionsStart)
  expect(start).toBeGreaterThanOrEqual(0)
  const next = requestEventsSource.indexOf('\n      {', start + 1)
  const end = next === -1 ? requestEventsSource.indexOf('\n    ];', start) : next
  return requestEventsSource.slice(start, end)
}

describe('UsagePage toolbar styles', () => {
  it('keeps visible range controls content-sized in narrow layouts', () => {
    expect(usagePageStyles).toMatch(
      /\.timeRangeGroup\s*\{[\s\S]*?width:\s*fit-content;/,
    );
    expect(usagePageStyles).toMatch(
      /\.timeRangeSelectControl\s*\{[\s\S]*?flex:\s*0 0 164px;/,
    );
  });

  it('keeps overview stat cards in a two-plus-four desktop grid with a distinct cache-rate color', () => {
    expect(usagePageStyles).toMatch(/\.statCard\s*\{[\s\S]*?grid-column:\s*span 3;/)
    expect(usagePageStyles).toMatch(/\.statCard:nth-child\(-n \+ 2\)\s*\{[\s\S]*?grid-column:\s*span 6;/)
    expect(usagePageStyles).toMatch(/\.statLabel\s*\{[\s\S]*?letter-spacing:\s*0;/)
    expect(statCardsSource).toContain("key: 'requests'")
    expect(statCardsSource).toContain("accent: '#3b82f6'")
    expect(statCardsSource).toContain("key: 'cache-rate'")
    expect(statCardsSource).toContain("accent: '#14b8a6'")
    expect(statCardsSource.match(/accent:\s*'#[0-9a-f]{6}'/g)).toHaveLength(new Set(statCardsSource.match(/accent:\s*'#[0-9a-f]{6}'/g)).size)
  })

  it('keeps refresh controls outside the query filter layout', () => {
    expect(usagePageSource).toContain(
      '{showRangeControls && (\n                  <>',
    );
    expect(usagePageSource).toContain(
      '<div className={styles.usageFilterBar}>',
    );
    expect(usagePageSource).toContain('className={styles.usageRefreshSlot}');
    expect(usagePageSource).toContain('showMonitoringQuery && (');
    expect(usagePageSource.indexOf('styles.monitoringQueryGroup')).toBeLessThan(
      usagePageSource.indexOf('styles.apiKeyFilterGroup'),
    );
    expect(usagePageSource).not.toContain('styles.usageFilterBarCollapsed');
    expect(usagePageStyles).toMatch(
      /\.usageRefreshSlot\s*\{[\s\S]*?flex:\s*0 0 auto;/,
    );
  });

  it('does not expose the removed manual sync endpoint from Usage controls', () => {
    expect(usagePageSource).not.toContain('SyncNowButton');
    expect(usagePageSource).not.toContain('manualSyncLoading');
    expect(apiClientSource).not.toContain("apiPath('/sync')");
    expect(apiClientSource).not.toContain('triggerSync');
    expect(
      existsSync(new URL('./usagePageDevActions.tsx', import.meta.url)),
    ).toBe(false);
  });

  it('keeps the API Key filter visible on the Analysis page so Analysis requests can be filtered', () => {
    expect(usagePageSource).not.toContain('shouldShowApiKeyFilter(activeTab)');
    expect(usagePageSource).not.toContain('styles.apiKeyFilterGroupHidden');
    expect(usagePageSource).not.toContain('aria-hidden={!showApiKeyFilter}');
    expect(usagePageStyles).not.toContain('.apiKeyFilterGroupHidden');
  });

  it('uses the new Analysis panel and endpoint instead of the old detail tables', () => {
    expect(usagePageSource).toContain('fetchAnalysis');
    expect(usagePageSource).toContain('<AnalysisPanel');
    expect(usagePageSource).not.toContain('fetchUsageAnalysis');
    expect(usagePageSource).not.toContain('<ApiDetailsCard');
    expect(usagePageSource).not.toContain('<ModelStatsCard');
    expect(apiIndexSource).not.toContain('ApiDetailsCard');
    expect(apiIndexSource).not.toContain('ModelStatsCard');
    expect(apiClientSource).toContain("apiPath('/usage/analysis')");
  });

  it('renames the Analysis tab label and places it before Request Events', () => {
    expect(i18nSource).toContain("tab_analysis: 'Analysis'");
    expect(i18nSource).not.toContain("tab_analysis: 'API & Models'");
    expect(i18nSource).not.toContain("tab_analysis: 'API 与模型'");
    expect(i18nSource).not.toContain("tab_analysis: 'API 與模型'");
    expect(usagePageSource).toContain('const BASE_USAGE_TAB_OPTIONS = [');
    expect(usagePageSource).toContain(
      "'overview',\n  'analysis',\n  'events',\n  'auth-files',\n  'ai-provider',\n  'storage',\n  'settings',",
    );
    expect(usagePageSource).toContain(
      'const USAGE_TAB_OPTIONS = withDevUsageTabs(BASE_USAGE_TAB_OPTIONS)',
    );
    expect(usagePageDevTabsSource).toContain('const DEV_USAGE_TAB_ORDER = [');
    expect(usagePageDevTabsSource).toContain(
      "'overview',\n  DEV_USAGE_TAB_VALUE,\n  'analysis',\n  'events',\n  'auth-files',\n  'ai-provider',\n  'storage',\n  'settings',",
    );
  });

  it('keeps Sign out as the rightmost header action after the update version badge', () => {
    expect(usagePageSource).toContain('logout');
    expect(usagePageSource).toContain('markStatusActive');
    expect(usagePageSource.indexOf('styles.updateCheckSwitcher')).toBeLessThan(
      usagePageSource.indexOf("t('common.logout')"),
    );
    expect(usagePageStyles).toContain('.signOutSwitcher');
    expect(usagePageStyles).toContain('.signOutPill');
  });

  it('keeps mobile tab labels on one line without changing desktop tab sizing', () => {
    const desktopTabPillBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.tabPill {'),
      usagePageStyles.indexOf('.tabPillActive'),
    );

    expect(usagePageStyles).toContain(
      '@include mobile {\n  .tabPill {\n    white-space: nowrap;\n  }\n',
    );
    expect(desktopTabPillBlock).not.toContain('white-space: nowrap;');
  });

  it('lets API Key Settings content scroll inside the card instead of being clipped', () => {
    expect(usagePageStyles).toMatch(
      /\.apiKeySettingsCard:global\(\.card\)\s*\{[\s\S]*?min-height:\s*auto;/,
    );
    expect(usagePageStyles).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?flex:\s*0 0 auto;/,
    );
    expect(usagePageStyles).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?height:\s*var\(--settings-list-scroll-height\);/,
    );
    expect(usagePageStyles).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?min-height:\s*0;/,
    );
    expect(usagePageStyles).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?overflow-y:\s*auto;/,
    );
    expect(usagePageStyles).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?padding-right:\s*4px;/,
    );
    const apiKeySettingsMobileBlock = usagePageStyles.slice(
      usagePageStyles.indexOf(
        '@include mobile {\n  .apiKeySettingsCard:global(.card)',
      ),
      usagePageStyles.indexOf('.pricesList'),
    );

    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeySettingsCard:global\(\.card\)\s*\{[\s\S]*?height:\s*auto;/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?height:\s*var\(--settings-list-scroll-height\);/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeySettingsList\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0, 1fr\);/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeySettingsItem\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\);/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeySettingsItem\s*\{[^}]*align-items:\s*stretch;/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeyAliasField\s*\{[\s\S]*?width:\s*100%;/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeyAliasField\s*\{[\s\S]*?:global\(\.form-group\)\s*\{[\s\S]*?width:\s*100%;/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeyAliasField\s*\{[\s\S]*?:global\(\.form-group\)\s*\{[\s\S]*?min-width:\s*0;/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeyAliasField\s*\{[\s\S]*?:global\(\.form-group\)\s*\{[\s\S]*?margin-bottom:\s*0;/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeyAliasInput\s*\{[\s\S]*?max-width:\s*100%;/,
    );
  });

  it('keeps Model Pricing Settings list viewport aligned with API Key Settings without shrinking it behind the form', () => {
    const settingsSectionsBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.settingsSections {'),
      usagePageStyles.indexOf('// Pricing Section'),
    );
    const pricingBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.pricingFixedCard {'),
      usagePageStyles.indexOf('.priceForm'),
    );
    const apiKeyBodyBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.apiKeySettingsBody {'),
      usagePageStyles.indexOf('.apiKeySettingsList'),
    );
    const apiKeySettingsMobileBlock = usagePageStyles.slice(
      usagePageStyles.indexOf(
        '@include mobile {\n  .apiKeySettingsCard:global(.card)',
      ),
      usagePageStyles.indexOf('.pricesList'),
    );
    const pricingGridBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.pricesGrid {'),
      usagePageStyles.indexOf('.priceItem'),
    );

    expect(settingsSectionsBlock).toMatch(
      /--settings-list-scroll-height:\s*480px;/,
    );
    expect(pricingBlock).toMatch(
      /\.pricingFixedCard\s*\{[\s\S]*?height:\s*auto;/,
    );
    expect(pricingBlock).not.toMatch(
      /\.pricingSection\s*\{[\s\S]*?height:\s*480px;/,
    );
    expect(apiKeyBodyBlock).toMatch(
      /height:\s*var\(--settings-list-scroll-height\);/,
    );
    expect(apiKeySettingsMobileBlock).toMatch(
      /\.apiKeySettingsBody\s*\{[\s\S]*?height:\s*var\(--settings-list-scroll-height\);/,
    );
    expect(pricingGridBlock).toMatch(
      /height:\s*var\(--settings-list-scroll-height\);/,
    );
    expect(pricingGridBlock).toMatch(
      /\.pricesGrid\s*\{[\s\S]*?overflow-y:\s*auto;/,
    );
    expect(pricingGridBlock).toMatch(
      /\.pricesGrid\s*\{[\s\S]*?overflow-x:\s*hidden;/,
    );
    expect(pricingGridBlock).not.toMatch(
      /@include mobile\s*\{[\s\S]*?overflow:\s*visible;/,
    );
  });

it('keeps the Analysis chart presentation aligned with the redesigned Analysis dashboard', () => {
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_token_usage_title')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_token_usage_subtitle')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_cost_breakdown_title')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_model_efficiency_title')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_composition_title')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_composition_token_percent')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_heatmap_title')")
    expect(analysisPanelSource).toContain("t('usage_stats.analysis_heatmap_subtitle')")
    expect(analysisPanelSource).toContain("t('usage_stats.total_cost')")
    expect(analysisPanelSource).toContain("import { Bar, Doughnut, Scatter } from 'react-chartjs-2'")
    expect(usagePageSource).toContain('LineController')
    expect(usagePageSource).toContain('LogarithmicScale')
    expect(usagePageSource).toContain('ChartJS.register(')
    expect(analysisPanelSource).toContain('<Bar data={chartData} options={chartOptions} plugins={[drawRequestsLineOnTopPlugin]} />')
    expect(analysisPanelSource).toContain("const activeContentKey = `${activeTab?.id ?? 'empty'}:${items.map((item) => item.key).join('|')}`")
    expect(analysisPanelSource).toContain('<Doughnut key={`chart-${activeContentKey}`} data={chartData} options={chartOptions} />')
    expect(analysisPanelSource).toContain('<Scatter data={chartData} options={chartOptions} />')
    expect(analysisPanelSource).toContain("cost: '#14b8a6'")
    expect(analysisPanelSource).toContain('ticks: { color: chartTheme.textSecondary')
    expect(analysisPanelSource).toContain('analysis_cost_per_million_tokens')
    expect(analysisPanelSource).toContain('analysis_blended_rate')
    expect(analysisPanelSource).toContain('styles.costStackFloatingTooltip')
    expect(analysisPanelSource).toContain('onMouseEnter={(event) => showCostTooltip(tooltipLines, event)}')
    expect(analysisPanelSource).toContain('createLinearGradient')
    expect(analysisPanelSource).not.toContain('createRadialGradient')
    expect(analysisPanelSource).toContain('className={styles.costRateMetric}')
    expect(analysisPanelSource).not.toContain('removeCompositionTooltip')
    expect(analysisPanelSource).toContain("yAxisID: 'cost'")
    expect(analysisPanelSource).toContain('buildAnalysisTokenChartOptions')
    expect(analysisPanelSource).toContain('buildCompositionChartData')
    expect(analysisPanelSource).toContain('CostBreakdownCard')
    expect(analysisPanelSource).toContain('ModelEfficiencyCard')
    expect(analysisPanelSource).toContain('CompositionPanel')
    expect(analysisPanelSource).toContain('heatmapTooltip')
    expect(analysisPanelSource).toContain('styles.heatmapModelHeaderCell')
    expect(analysisPanelSource).toContain('styles.heatmapModelLabel')
    expect(analysisPanelSource).toContain('onMouseEnter={(event) => showTooltip([model], event)}')
    expect(analysisPanelSource).toContain('onFocus={(event) => showTooltip([model], event)}')
    expect(analysisPanelSource).not.toContain('styles.efficiencyList')
    expect(analysisPanelSource).not.toContain('styles.efficiencyRow')
    expect(analysisPanelSource).toContain('getHeatmapCellColor(intensity, isDark)')
    expect(analysisPanelSource).toContain('formatUsd')
    expect(analysisPanelSource).not.toContain("analysis_api_key_composition_title")
    expect(analysisPanelSource).not.toContain("analysis_model_composition_title")
    expect(analysisPanelSource).not.toContain("analysis_auth_files_composition_title")
    expect(analysisPanelSource).not.toContain("analysis_ai_provider_composition_title")
    expect(analysisPanelSource).not.toContain("analysis_heatmap_tokens_prefix")
    expect(analysisPanelSource).not.toContain("analysis_heatmap_requests_prefix")
    expect(analysisPanelSource).not.toContain("from 'recharts'")
    expect(analysisPanelStyles).toMatch(/\.insightGrid\s*\{[\s\S]*?grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/)
    expect(analysisPanelStyles).toMatch(/\.insightGrid\s*\{[\s\S]*?@include mobile\s*\{[\s\S]*?grid-template-columns:\s*1fr;/)
    expect(analysisPanelStyles).toMatch(/\.costRatePanel\s*\{[\s\S]*?grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\);/)
    expect(analysisPanelStyles).toMatch(/\.costRatePanel\s*\{[\s\S]*?gap:\s*0;/)
    expect(analysisPanelStyles).toMatch(/\.costRateMetric \+ \.costRateMetric,\s*\.costRateSparkline\s*\{[\s\S]*?border-left:\s*1px solid var\(--border-color\);/)
    expect(analysisPanelStyles).toMatch(/\.costRateSparkline\s*\{[\s\S]*?height:\s*100%;/)
    expect(analysisPanelStyles).toMatch(/\.costRateMetric\s*\{[\s\S]*?justify-content:\s*flex-start;/)
    expect(analysisPanelStyles).toMatch(/\.costStackSegment\s*\{[\s\S]*?background:\s*linear-gradient\(90deg, color-mix\(in srgb, var\(--cost-segment-color\) 72%, var\(--bg-secondary\)\), var\(--cost-segment-color\)\);/)
    expect(analysisPanelStyles).toMatch(/\.costStackFloatingTooltip\s*\{[\s\S]*?position:\s*fixed;/)
    expect(analysisPanelStyles).toMatch(/\.insightGrid\s*\{[\s\S]*?align-items:\s*stretch;/)
    expect(analysisPanelStyles).toMatch(/\.efficiencyChartFrame\s*\{[\s\S]*?height:\s*300px;/)
    expect(analysisPanelStyles).not.toContain('.efficiencyList')
    expect(analysisPanelStyles).not.toContain('.efficiencyRow')
    expect(analysisPanelStyles).toMatch(/\.compositionLayout\s*\{[\s\S]*?grid-template-columns:\s*minmax\(220px, 0\.72fr\) minmax\(0, 1\.28fr\);/)
    expect(analysisPanelStyles).toMatch(/\.compositionLayout\s*\{[\s\S]*?@include mobile\s*\{[\s\S]*?grid-template-columns:\s*1fr;/)
    expect(analysisPanelStyles).toMatch(/\.compositionTabActive\s*\{[\s\S]*?background:\s*color-mix\(in srgb, var\(--bg-primary\) 84%, var\(--bg-secondary\)\);/)
    expect(analysisPanelStyles).not.toMatch(/\.compositionTabActive\s*\{[\s\S]*?#2563eb/)
    expect(analysisPanelStyles).toMatch(/\.heatmapCardLight \.analysisChartSurface\s*\{[\s\S]*?background:\s*color-mix/)
    expect(analysisPanelStyles).toMatch(/\.heatmapCardDark \.analysisChartSurface\s*\{[\s\S]*?background:\s*#100e16;/)
    expect(analysisPanelStyles).toMatch(/\.heatmapCell::before\s*\{[\s\S]*?radial-gradient\(circle at 50% 115%/)
    expect(analysisPanelStyles).toMatch(/\.heatmapCorner,\s*\.heatmapHeaderCell\s*\{[\s\S]*?min-height:\s*48px;/)
    const heatmapRowLabelBlock = [...analysisPanelStyles.matchAll(/\.heatmapRowLabel\s*\{([\s\S]*?)\n\}/g)]
      .map((match) => match[1])
      .find((block) => block.includes('display: flex;')) ?? ''
    expect(heatmapRowLabelBlock).toContain('height: 30px;')
    expect(heatmapRowLabelBlock).toContain('align-self: center;')
    expect(analysisPanelStyles).toMatch(/\.heatmapModelLabel\s*\{[\s\S]*?-webkit-line-clamp:\s*2;/)
    expect(analysisPanelStyles).toMatch(/\.heatmapModelLabel\s*\{[\s\S]*?overflow-wrap:\s*anywhere;/)
    expect(analysisPanelStyles).toMatch(/\.heatmapLegendRamp\s*\{[\s\S]*?linear-gradient\(90deg, #fff7ed, #fed7aa, #fb923c, #ef4444, #7c2d12\)/)
    expect(analysisPanelStyles).toMatch(/\.heatmapCardDark \.heatmapLegendRamp\s*\{[\s\S]*?linear-gradient\(90deg, #1a1118, #4a1f23, #9a3412, #f97316, #fde68a\)/)
    expect(analysisPanelStyles).toMatch(/\.heatmapFloatingTooltip\s*\{[\s\S]*?position:\s*fixed;/)
    expect(analysisPanelStyles).toMatch(/\.heatmapFloatingTooltip\s*\{[\s\S]*?border:\s*1px solid var\(--border-color\);/)
    expect(analysisPanelStyles).toMatch(/\.heatmapFloatingTooltip\s*\{[\s\S]*?background:\s*var\(--bg-primary\);/)
    expect(analysisPanelStyles).toMatch(/\.heatmapFloatingTooltip\s*\{[\s\S]*?color:\s*var\(--text-secondary\);/)
    expect(analysisPanelStyles).toMatch(/\.heatmapTooltipTitle\s*\{[\s\S]*?color:\s*var\(--text-primary\);/)
    expect(analysisPanelStyles).not.toContain('.heatmapCellTooltip')
    expect(analysisPanelStyles).not.toContain('.compositionGrid')
    expect(analysisPanelStyles).not.toContain('.heatmapCellRequestValue')
    expect(analysisPanelStyles).not.toContain('rgb(250, 244, 230)')
  })


  it('keeps Monitoring failure model tags readable in light and dark themes', () => {
    expect(monitoringCenterStyles).toMatch(
      /\.failureModelTag\s*\{[\s\S]*?color:\s*#b91c1c;/,
    );
    expect(monitoringCenterStyles).toMatch(
      /\.failureModelTag\s*\{[\s\S]*?background:\s*color-mix\(in srgb, #fee2e2 78%, var\(--bg-primary\)\);/,
    );
    expect(monitoringCenterStyles).toMatch(
      /:global\(\[data-theme='dark'\]\) \.failureModelTag\s*\{[\s\S]*?color:\s*#fca5a5;/,
    );
  });

  it('uses the shared Select control in Monitoring filters', () => {
    expect(monitoringCenterSource).toContain(
      "import { Select, type SelectOption } from '@/components/ui/Select'",
    );
    expect(monitoringCenterSource).toContain(
      'styles.monitoringSelect',
    );
    expect(monitoringCenterSource).toContain(
      "options={withAllOption(t('usage_stats.monitoring_all_sources'), channelSourceOptions)}",
    );
    expect(monitoringCenterSource).toContain(
      "options={withAllOption(t('usage_stats.monitoring_all_sources'), failureSourceOptions)}",
    );
    expect(monitoringCenterSource).not.toContain('requestLogSourceOptions');
    expect(monitoringCenterSource).not.toContain('compactMaskedText');
    expect(monitoringCenterSource).toContain(
      "const source = String(item.source || '').trim()",
    );
    expect(monitoringCenterStyles).toMatch(
      /\.monitoringSelect\s*\{[\s\S]*?border-radius:\s*999px;/,
    );
    expect(monitoringCenterStyles).not.toContain('.filterSelect');
  });

  it('lets Monitoring channel and failure source labels wrap within the source column', () => {
    expect(monitoringCenterSource).toContain('styles.cellTitle');
    expect(monitoringCenterSource).toContain('styles.cellMeta');
    expect(monitoringCenterStyles).toMatch(
      /\.cellTitle\s*\{[\s\S]*?text-overflow:\s*clip;/,
    );
    expect(monitoringCenterStyles).toMatch(
      /\.cellTitle\s*\{[\s\S]*?white-space:\s*normal;/,
    );
    expect(monitoringCenterStyles).toMatch(
      /\.cellTitle\s*\{[\s\S]*?overflow-wrap:\s*anywhere;/,
    );
    expect(monitoringCenterStyles).toMatch(
      /\.cellMeta\s*\{[\s\S]*?text-overflow:\s*clip;/,
    );
    expect(monitoringCenterStyles).toMatch(
      /\.cellMeta\s*\{[\s\S]*?white-space:\s*normal;/,
    );
    expect(monitoringCenterStyles).toMatch(
      /\.cellMeta\s*\{[\s\S]*?overflow-wrap:\s*anywhere;/,
    );
  });

  it('keeps Monitoring data mounted while refresh is loading so scroll position is preserved', () => {
    const monitoringDataHookSource = readFileSync(
      new URL(
        '../components/usage/monitoring/useMonitoringCenterData.ts',
        import.meta.url,
      ),
      'utf8',
    );
    const refreshStartBlock = monitoringDataHookSource.slice(
      monitoringDataHookSource.indexOf('controllerRef.current = controller'),
      monitoringDataHookSource.indexOf('try {'),
    );

    expect(refreshStartBlock).toContain('setLoading(true)');
    expect(refreshStartBlock).not.toContain('setData(null)');
  });

  it('does not request expanded Monitoring request logs for the removed log table', () => {
    const monitoringDataHookSource = readFileSync(
      new URL(
        '../components/usage/monitoring/useMonitoringCenterData.ts',
        import.meta.url,
      ),
      'utf8',
    );

    expect(monitoringCenterSource).not.toContain('REQUEST_LOG_PAGE_SIZE_OPTIONS');
    expect(monitoringDataHookSource).not.toContain('MONITORING_REQUEST_LOG_LIMIT');
    expect(monitoringDataHookSource).toContain(
      'fetchUsageMonitoring(range, start, end, controller.signal, {',
    );
  });

  it('shows Monitoring failure model counts with failure semantics', () => {
    expect(monitoringCenterSource).toContain(
      'const visibleModels = failure.models',
    );
    expect(monitoringCenterSource).not.toContain('failure.models.slice(0, 2)');
    expect(monitoringCenterSource).toContain("t('usage_stats.failure_count')");
    expect(monitoringCenterSource).not.toContain(
      "`${t('usage_stats.requests_count')}: ${formatCompactNumber(model.failure)}`",
    );
  });

  it('removes the Monitoring Recent Request Logs table and detail flow', () => {
    expect(monitoringCenterSource).not.toContain('RequestEventsTable');
    expect(monitoringCenterSource).not.toContain('RequestEventTableRow');
    expect(monitoringCenterSource).not.toContain('monitoring_request_logs');
    expect(monitoringCenterSource).not.toContain('requestLogSourceFilter');
    expect(monitoringCenterSource).not.toContain('requestLogModelFilter');
    expect(monitoringCenterSource).not.toContain('requestLogStatusFilter');
    expect(monitoringCenterSource).not.toContain('selectedRequestLog');
    expect(monitoringCenterSource).not.toContain('renderRequestLogDetail');
    expect(monitoringCenterSource).not.toContain('fetchUsageEventRequestDetail');
    expect(monitoringCenterSource).not.toContain('RequestDetailStructuredView');
    expect(monitoringCenterSource).not.toContain(
      'styles.requestLogTableWrapper',
    );
    expect(monitoringCenterStyles).not.toContain('.requestLogDetailPanel');
    expect(monitoringCenterStyles).not.toContain('.requestLogDetailHeader');
    expect(monitoringCenterStyles).not.toContain('.requestLogTableWrapper');
    expect(monitoringCenterStyles).not.toContain('.requestLogTable');
    expect(monitoringCenterStyles).not.toContain('min-width: 1280px');
    expect(requestEventsSource).toContain('export function RequestEventsTable');
    expect(monitoringCenterSource).not.toContain(
      'channelStats.length + failureAnalysis.length + requestLogs.length',
    );
    expect(monitoringCenterSource).toContain(
      'channelStats.length + failureAnalysis.length',
    );
  });

  it('styles Request Event source notes as distinct inline note badges', () => {
    expect(usageNoteBadgeSource).toContain('UsageNoteBadge');
    expect(requestEventsSource).toContain('<UsageNoteBadge');
    expect(authFileCredentialsSource).toContain('<UsageNoteBadge');
    expect(aiProviderCredentialsSource).toContain('<UsageNoteBadge');
    expect(usageNoteBadgeStyles).toContain('display: inline-flex;');
    expect(usageNoteBadgeStyles).toContain('border-radius: $radius-full;');
    expect(usageNoteBadgeStyles).toContain('var(--quota-medium-color)');
    expect(usageNoteBadgeStyles).toContain('white-space: nowrap;');
    expect(usageNoteBadgeStyles).not.toContain('color: var(--text-tertiary);');
  });

  it('keeps Monitoring recent status as twelve five-minute aligned buckets', () => {
    expect(monitoringCenterSource).toContain('const REQUEST_STATUS_BUCKET_COUNT = 12;');
    expect(monitoringCenterSource).toContain('const REQUEST_STATUS_BUCKET_MS = 5 * 60_000;');
    expect(monitoringCenterSource).toContain('styles.statusMixedLow');
    expect(monitoringCenterSource).toContain('styles.statusMixedMedium');
    expect(monitoringCenterSource).toContain('styles.statusMixedHigh');
    expect(monitoringCenterStyles).toMatch(/\.statusBars\s*\{[\s\S]*?gap:\s*1px;/);
    expect(monitoringCenterStyles).toContain('.statusEmpty');
    expect(monitoringCenterStyles).toContain('.statusMixedLow');
    expect(monitoringCenterStyles).toContain('.statusMixedMedium');
    expect(monitoringCenterStyles).toContain('.statusMixedHigh');
  });

  it('shows Monitoring channel total cost and keeps stats tables compact', () => {
    expect(monitoringCenterSource).toContain("<th>{t('usage_stats.total_cost')}</th>");
    expect(monitoringCenterSource).toContain('channel.cost_available ? formatUsd(channel.total_cost) :');
    expect(monitoringCenterStyles).toMatch(/\.statsTableWrapper\s*\{[\s\S]*?max-height:\s*360px;/);
  });
  it('widens only the API key dropdown menu without changing the trigger width', () => {
    expect(selectSource).toContain('dropdownMinWidth?: number');
    expect(selectSource).toContain('rect.left - (width - rect.width) / 2');
    expect(usagePageSource).toContain('dropdownMinWidth={180}');
  });

  it('moves the top bar continuously with the toolbar push distance', () => {
    const topBarBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.topBar {'),
      usagePageStyles.indexOf('.brandBlock'),
    );

    expect(usagePageSource).toContain('topBarRef');
    expect(usagePageSource).toContain('toolbarRowRef');
    expect(usagePageSource).toContain('STICKY_TOOLBAR_PUSH_GAP_PX');
    expect(usagePageSource).toContain(
      "topBar.style.setProperty('--top-bar-push-offset'",
    );
    expect(usagePageSource).toContain(
      "topBar.style.setProperty('--top-bar-push-progress'",
    );
    expect(usagePageSource).not.toContain('styles.topBarPushed');
    expect(usagePageSource).not.toContain('toolbarSentinelRef');
    expect(topBarBlock).toContain('position: sticky;');
    expect(topBarBlock).toContain('top: 16px;');
    expect(topBarBlock).toContain('z-index: 10;');
    expect(topBarBlock).toContain('--top-bar-push-offset: 0px;');
    expect(topBarBlock).toContain('--top-bar-push-progress: 0;');
    expect(topBarBlock).toContain(
      'transform: translateY(calc(-1 * var(--top-bar-push-offset)));',
    );
    expect(topBarBlock).toContain(
      'opacity: calc(1 - var(--top-bar-push-progress));',
    );
    expect(usagePageStyles).not.toContain('.topBarPushed');
    expect(usagePageStyles).not.toContain('.toolbarStickySentinel');
    expect(usagePageStyles).toMatch(
      /\.toolbarRow\s*\{[\s\S]*?position:\s*sticky;/,
    );
    expect(usagePageStyles).toMatch(/\.toolbarRow\s*\{[\s\S]*?top:\s*16px;/);
    expect(usagePageStyles).toMatch(/\.toolbarRow\s*\{[\s\S]*?z-index:\s*9;/);
    expect(usagePageStyles).not.toMatch(
      /\.toolbarRow\s*\{[\s\S]*?top:\s*94px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.toolbarRow\s*\{[\s\S]*?backdrop-filter:\s*blur\(18px\);/,
    );
    expect(usagePageStyles).toMatch(
      /\.toolbarActionsRight\s*\{[\s\S]*?align-items:\s*center;/,
    );
    expect(usagePageStyles).toMatch(
      /\.usageFilterBar\s*\{[\s\S]*?align-items:\s*center;/,
    );
    expect(usagePageStyles).toMatch(
      /\.apiKeySelectControl\s*\{[\s\S]*?width:\s*172px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.monitoringQueryInput\s*\{[\s\S]*?width:\s*220px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.rangeSelectControl\s*\{[\s\S]*?width:\s*164px;/,
    );
  });

  it('keeps custom range inputs hidden and disabled until the custom range is selected', () => {
    expect(usagePageSource).toContain('styles.customRangeFieldGroupOpen');
    expect(usagePageSource).toContain('aria-hidden={!isCustomRange}');
    expect(usagePageSource).toContain('disabled={!isCustomRange}');
    expect(usagePageSource).not.toContain('{isCustomRange && (');
  });

  it('keeps custom date inputs selectable through the native picker without pointer interception', () => {
    expect(usagePageStyles).toMatch(
      /\.customRangeInput\s*\{[\s\S]*?user-select:\s*none;/,
    );
    expect(usagePageStyles).toMatch(
      /\.customRangeInput\s*\{[\s\S]*?-webkit-user-select:\s*none;/,
    );
    expect(usagePageSource).not.toContain('readOnly');
    expect(usagePageSource).not.toContain(
      'onPointerDown={handleCustomDateInputPointerDown}',
    );
    expect(usagePageSource).toContain(
      'onClick={handleCustomDateInputActivate}',
    );
    expect(usagePageSource).toContain(
      'onFocus={handleCustomDateInputActivate}',
    );
    expect(usagePageSource).toContain(
      'onKeyDown={handleCustomDateInputKeyDown}',
    );
  });


  it('keeps mobile custom date fields inside the toolbar before the refresh action', () => {
    const narrowToolbarStart = usagePageStyles.indexOf('@media (max-width: #{$breakpoint-tablet})')
    const mobileToolbarStart = usagePageStyles.indexOf('@include mobile {\n  .tabPill', narrowToolbarStart)
    const narrowToolbarBlock = usagePageStyles.slice(
      narrowToolbarStart,
      mobileToolbarStart
    )
    const mobileToolbarBlock = usagePageStyles.slice(
      mobileToolbarStart,
      usagePageStyles.indexOf('@media (prefers-reduced-motion: reduce)')
    )

    expect(narrowToolbarBlock).toMatch(/\.usageFilterBar\s*\{[\s\S]*?max-height:\s*none;/)
    expect(narrowToolbarBlock).toMatch(/\.usageFilterBar\s*\{[\s\S]*?overflow:\s*visible;/)
    expect(narrowToolbarBlock).toMatch(/\.timeRangeGroup\s*\{[\s\S]*?width:\s*100%;/)
    expect(narrowToolbarBlock).toMatch(/\.customRangeFieldGroup\s*\{[\s\S]*?width:\s*100%;/)
    expect(narrowToolbarBlock).toMatch(/\.customRangeFieldGroupOpen\s*\{[\s\S]*?max-height:\s*180px;/)
    expect(mobileToolbarBlock).toMatch(/\.usageFilterBar\s*\{[\s\S]*?display:\s*grid;/)
    expect(mobileToolbarBlock).toMatch(/\.usageFilterBar\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0, 1fr\);/)
    expect(mobileToolbarBlock).toMatch(/\.rangeFilterField\s*\{[\s\S]*?grid-template-columns:\s*auto minmax\(0, 1fr\);/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeFieldGroup\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0, 1fr\);/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeField\s*\{[\s\S]*?grid-template-columns:\s*auto minmax\(0, 1fr\);/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeField\s*\{[\s\S]*?min-width:\s*0;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeField\s*\{[\s\S]*?max-width:\s*100%;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInputShell\s*\{[\s\S]*?position:\s*relative;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInputShell\s*\{[\s\S]*?overflow:\s*hidden;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInputDisplay\s*\{[\s\S]*?display:\s*flex;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInput\s*\{[\s\S]*?position:\s*absolute;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInput\s*\{[\s\S]*?min-width:\s*0;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInput\s*\{[\s\S]*?max-width:\s*100%;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInput\s*\{[\s\S]*?display:\s*block;/)
    expect(mobileToolbarBlock).toMatch(/\.customRangeInput\s*\{[\s\S]*?opacity:\s*0;/)
  })

  it('keeps Overview chart period controls hidden because period selection is automatic', () => {
    expect(usageChartSource).not.toContain('className={styles.periodButtons}');
    expect(tokenBreakdownChartSource).not.toContain(
      'className={styles.periodButtons}',
    );
    expect(costTrendChartSource).not.toContain(
      'className={styles.periodButtons}',
    );
  });

  it('places Chart Line Selection and trend cards below Cost Trend on Overview', () => {
    const serviceHealthIndex = usagePageSource.indexOf('<ServiceHealthCard');
    const tokenBreakdownIndex = usagePageSource.indexOf('<TokenBreakdownChart');
    const costTrendIndex = usagePageSource.indexOf('<CostTrendChart');
    const chartLineSelectorIndex =
      usagePageSource.indexOf('<ChartLineSelector');
    const chartsGridIndex = usagePageSource.indexOf(
      '<div className={styles.chartsGrid}>',
    );

    expect(serviceHealthIndex).toBeGreaterThan(-1);
    expect(tokenBreakdownIndex).toBeGreaterThan(serviceHealthIndex);
    expect(costTrendIndex).toBeGreaterThan(tokenBreakdownIndex);
    expect(chartLineSelectorIndex).toBeGreaterThan(costTrendIndex);
    expect(chartsGridIndex).toBeGreaterThan(chartLineSelectorIndex);
  });

  it('keeps Service Health grid height bounded when few columns are returned', () => {
    const healthGridBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.healthGrid {'),
      usagePageStyles.indexOf('.healthBlockWrapper'),
    );

    expect(healthGridBlock).toContain(
      'grid-template-rows: repeat(var(--health-grid-rows), var(--health-grid-block-size));',
    );
    expect(healthGridBlock).not.toContain('aspect-ratio:');
    expect(usagePageStyles).toMatch(
      /\.healthBlockWrapper\s*\{[\s\S]*?height:\s*var\(--health-grid-block-size\);/,
    );
  });

  it('removes the Overview Request Health Timeline label instead of toggling it off', () => {
    expect(usagePageSource).toMatch(/<ServiceHealthCard\s+usage=\{overviewUsage\}\s+loading=\{overviewDisplayLoading\}\s*\/>/)
    expect(usagePageSource).not.toContain('showEyebrow')
  })

  it('keeps chart line controls aligned with reusable pill controls', () => {
    expect(chartLineSelectorSource).toContain(
      'className={styles.usagePillControl}',
    );
    expect(chartLineSelectorSource).toContain(
      'className={styles.usagePillAction}',
    );
  });

  it('aligns Request Event Log pagination with credential pagination height', () => {
    expect(usagePageStyles).toMatch(
      /\.requestEventsCard:global\(\.card\)\s*\{[\s\S]*?padding-bottom:\s*0;/,
    );
    expect(requestEventsSource).toContain(
      'className={styles.requestEventsCard}',
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationFooter\s*\{[\s\S]*?--usage-pagination-bar-height:\s*58px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationFooter\s*\{[\s\S]*?flex:\s*0 0 var\(--usage-pagination-bar-height\);/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationFooter\s*\{[\s\S]*?min-height:\s*var\(--usage-pagination-bar-height\);/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationFooter\s*\{[\s\S]*?box-sizing:\s*border-box;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationFooter\s*\{[\s\S]*?align-items:\s*center;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationFooter\s*\{[\s\S]*?padding:\s*8px #\{\$spacing-lg\};/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPaginationControls\s*\{[\s\S]*?gap:\s*10px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPageSizeControl\s*\{[\s\S]*?margin-left:\s*10px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPageSizeControl\s*\{[\s\S]*?select\s*\{[\s\S]*?appearance:\s*none;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsPageSizeControl\s*\{[\s\S]*?&::after\s*\{[\s\S]*?mask:\s*url\("data:image\/svg\+xml/,
    );
    expect(requestEventsSource).toContain('footer={(');
  });

  it('keeps Request Event Log headers visible while the table scrolls', () => {
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFrame\s*\{[\s\S]*?height:\s*clamp\(560px,\s*72vh,\s*760px\);/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFrame\s*\{[\s\S]*?overflow:\s*hidden;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFrame\s*\{[\s\S]*?border-radius:\s*\$radius-lg;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFrame\s*\{[\s\S]*?box-shadow:/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableScroll\s*\{[\s\S]*?flex:\s*1 1 auto;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableScroll\s*\{[\s\S]*?overflow:\s*auto;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableScroll\s*\{[\s\S]*?thead\s+th\s*\{[\s\S]*?position:\s*sticky;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableScroll\s*\{[\s\S]*?thead\s+th\s*\{[\s\S]*?top:\s*0;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableScroll\s*\{[\s\S]*?thead\s+th\s*\{[\s\S]*?z-index:\s*2;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableScroll\s*\{[\s\S]*?\.table\s*\{[\s\S]*?border-collapse:\s*separate;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFooter\s*\{[\s\S]*?z-index:\s*3;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFooter\s*\{[\s\S]*?padding:\s*14px \$spacing-lg \$spacing-lg;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFooter\s*\{[\s\S]*?background:/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsTableFooter\s*\{[\s\S]*?box-shadow:\s*0 -12px 24px/,
    );
    expect(requestEventsSource).toContain('styles.requestEventsTableFooter');
  });

  it('renders Request Detail JSON with separated tabs and axonhub-style collapsible controls', () => {
    expect(requestEventsSource).toContain('RequestDetailJsonBlock');
    expect(requestDetailJsonViewerSource).toContain('RequestDetailJsonViewer');
    expect(requestDetailJsonViewerSource).toContain('RequestDetailJsonNode');
    expect(requestEventsSource).toContain('RequestDetailContentPage');
    expect(requestEventsSource).toContain('RequestDetailTrafficSide');
    expect(requestEventsSource).toContain('requestDetailHeadersToJsonValue');
    expect(requestEventsSource).toContain('requestDetailBodyToJsonValue');
    expect(requestEventsSource).toContain('styles.requestEventsDetailPageTabs');
    expect(requestEventsSource).toContain(
      'styles.requestEventsDetailTrafficTabs',
    );
    expect(requestDetailJsonViewerSource).toContain(
      'styles.requestEventsDetailJsonBlock',
    );
    expect(requestEventsSource).toContain(
      'request_events_detail_request_headers',
    );
    expect(requestEventsSource).toContain(
      'request_events_detail_response_body',
    );
    expect(requestEventsSource).not.toContain('<details');
    expect(requestDetailJsonViewerSource).not.toContain('<details');
    expect(requestEventsSource).not.toContain('RequestDetailLogSections');
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailPageTabs,\s*\n\.requestEventsDetailTrafficTabs\s*\{[\s\S]*?border-radius:\s*\$radius-full;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailPageTab,\s*\n\.requestEventsDetailTrafficTab\s*\{[\s\S]*?cursor:\s*pointer;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonGrid\s*\{[^}]*flex-direction:\s*column;/,
    );
    expect(usagePageStyles).not.toMatch(
      /\.requestEventsDetailJsonGrid\s*\{[^}]*grid-template-columns:/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonBlock\s*\{[\s\S]*?text-transform:\s*uppercase;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonViewer\s*\{[\s\S]*?contain:\s*inline-size;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonRow\s*\{[\s\S]*?&:hover\s*\{[\s\S]*?background:/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonToggle\s*\{[\s\S]*?cursor:\s*pointer;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonChildren\s*\{[\s\S]*?padding-left:\s*8px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailJsonParseButton,\s*\n\.requestEventsDetailJsonParsedHeader button\s*\{[\s\S]*?text-decoration:\s*underline;/,
    );
    expect(requestEventsSource).toContain('<textarea');
    expect(requestEventsSource).toContain('wrap="off"');
    expect(requestEventsSource).not.toContain(
      '<pre className={styles.requestEventsDetailRawLog}',
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*white-space:\s*pre;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*overflow-wrap:\s*normal;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*word-break:\s*normal;/,
    );
    expect(usagePageStyles).toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*resize:\s*vertical;/,
    );
    expect(usagePageStyles).not.toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*white-space:\s*pre-wrap;/,
    );
    expect(usagePageStyles).not.toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*overflow-wrap:\s*break-word;/,
    );
    expect(usagePageStyles).not.toMatch(
      /\.requestEventsDetailRawLog\s*\{[^}]*word-break:\s*break-word;/,
    );
  });


  it('themes the WebKit scrollbar corner so intersecting scrollbars do not show a white square', () => {
    expect(globalStyles).toMatch(/::-webkit-scrollbar-corner\s*\{[\s\S]*?background:\s*var\(--bg-secondary\);/)
  })

  it('renders Request Event Log with a single outer frame instead of a nested table card', () => {
    const cardBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.requestEventsCard:global(.card) {'),
      usagePageStyles.indexOf('.requestEventsTitleRow')
    )
    const tableFrameBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.requestEventsTableFrame {'),
      usagePageStyles.indexOf('.requestEventsTableScroll')
    )

    expect(cardBlock).toMatch(/padding:\s*0;/)
    expect(cardBlock).toMatch(/overflow:\s*hidden;/)
    expect(cardBlock).toMatch(/:global\(\.card-header\)\s*\{[\s\S]*?margin-bottom:\s*0;/)
    expect(cardBlock).toMatch(/:global\(\.card-header\)\s*\{[\s\S]*?border-bottom:\s*1px solid var\(--border-color\);/)
    expect(usagePageStyles).not.toContain('.requestEventsTableWrapper {')
    expect(tableFrameBlock).toMatch(/border:\s*1px solid/)
    expect(tableFrameBlock).toMatch(/border-radius:\s*\$radius-lg;/)
  })

  it('keeps Request Event Log long text columns controlled', () => {
    expect(usagePageStyles).toMatch(/\.requestEventsAPIKeyCell\s*\{[\s\S]*?min-width:\s*135px;/)
    expect(usagePageStyles).toMatch(/\.requestEventsAPIKeyCell\s*\{[\s\S]*?max-width:\s*240px;/)
    expect(usagePageStyles).toMatch(/\.requestEventsSourceCell\s*\{[\s\S]*?min-width:\s*165px;/)
    expect(usagePageStyles).toMatch(/\.modelCell\s*\{[\s\S]*?min-width:\s*110px;/)
    expect(usagePageStyles).toMatch(/\.modelCell\s*\{[\s\S]*?max-width:\s*240px;/)
    expect(usagePageStyles).not.toContain('.requestEventsAuthIndex')
    expect(usagePageStyles).not.toContain('.requestEventsEndpointCell')
  })

  it('keeps Request Event Log non-text columns adaptive and non-wrapping', () => {
    const adaptiveColumnIds = [
      'timestamp',
      'reasoning_effort',
      'result',
      'request_type',
      'endpoint',
      'ttft',
      'latency',
      'speed',
      'input_tokens',
      'output_tokens',
      'reasoning_tokens',
      'cached_tokens',
      'cache_rate',
      'total_tokens',
      'total_cost',
    ]
    const noWrapCellBlock = usagePageStyles.slice(
      usagePageStyles.indexOf('.requestEventsNoWrapCell {'),
      usagePageStyles.indexOf('.requestEventsSourceCell')
    )

    expect(noWrapCellBlock).toMatch(/white-space:\s*nowrap;/)
    expect(noWrapCellBlock).toMatch(/font-variant-numeric:\s*tabular-nums;/)
    expect(usagePageStyles).not.toContain('.requestEventsSpeedCell')

    adaptiveColumnIds.forEach((columnId) => {
      const block = requestEventColumnDefinitionBlock(columnId)
      expect(block).toMatch(/header:\s*<th[^>]*styles\.requestEventsNoWrapCell/)
      expect(block).toMatch(/renderCell:[\s\S]*<td[^>]*styles\.requestEventsNoWrapCell/)
    })

    ;['api_key', 'source', 'model'].forEach((columnId) => {
      expect(requestEventColumnDefinitionBlock(columnId)).not.toContain('styles.requestEventsNoWrapCell')
    })
  })

  it('provides reusable pill controls for usage subpages', () => {
    expect(usagePageStyles).toMatch(
      /\.usagePillControl\s*\{[\s\S]*?border-radius:\s*999px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.usagePillAction\s*\{[\s\S]*?border-radius:\s*999px;/,
    );
    expect(usagePageStyles).toMatch(
      /\.usagePillActionDanger\s*\{[\s\S]*?color:/,
    );
    expect(usagePageStyles).not.toContain(
      '&:global(.btn-danger):hover:not(:disabled)',
    );
    expect(usagePageStyles).toMatch(
      /:global\(\.input\)\s*\{[^}]*border-radius:\s*999px;/,
    );
    expect(requestEventsSource).toContain('styles.usagePillControl');
    expect(requestEventsSource).toContain('styles.usagePillAction');
    expect(priceSettingsSource).toContain('styles.usagePillControl');
    expect(priceSettingsSource).toContain('styles.usagePillAction');
    expect(priceSettingsSource).toContain('styles.usagePillActionDanger');
  });
});
