package migration

import (
	"fmt"
	"strings"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/gorm"
)

// backfillClaudeUsageTokensMigration 修复旧版 Claude usage token 口径。
//
// 这次回填只修正已经进入各自聚合 cursor 的历史事件：
// - usage_events 先统一修正成新口径；
// - overview 只补已经聚合过的事件，未聚合事件留给后续 catch-up；
// - usage_identities 也只补已经聚合过的事件，避免和后续增量聚合重复累加。
func backfillClaudeUsageTokensMigration(tx *gorm.DB) error {
	for _, table := range []string{"usage_events", "usage_identities", "usage_overview_hourly_stats", "usage_overview_daily_stats", "usage_overview_aggregation_checkpoints"} {
		if !tx.Migrator().HasTable(table) {
			return nil
		}
	}

	overviewLastAggregatedUsageEventID, err := loadClaudeBackfillOverviewCursor(tx)
	if err != nil {
		return err
	}
	identities, err := loadClaudeBackfillIdentities(tx)
	if err != nil {
		return err
	}
	events, err := loadClaudeBackfillCandidateEvents(tx)
	if err != nil {
		return err
	}

	hourlyDeltas := map[claudeUsageTokenAggregateKey]claudeUsageTokenDelta{}
	dailyDeltas := map[claudeUsageTokenAggregateKey]claudeUsageTokenDelta{}
	identityDeltas := map[claudeUsageTokenIdentityKey]claudeUsageTokenDelta{}

	for _, event := range events {
		candidate, ok := buildClaudeUsageTokenBackfillCandidate(event, identities, overviewLastAggregatedUsageEventID)
		if !ok {
			continue
		}
		delta := candidate.delta()
		if delta.isZero() {
			continue
		}
		if err := updateClaudeBackfillUsageEvent(tx, candidate); err != nil {
			return err
		}
		if candidate.applyOverviewDelta {
			addClaudeUsageTokenDelta(hourlyDeltas, candidate.aggregateKey(candidate.timestamp.Truncate(time.Hour)), delta)
			dayBucket := time.Date(candidate.timestamp.Year(), candidate.timestamp.Month(), candidate.timestamp.Day(), 0, 0, 0, 0, candidate.timestamp.Location())
			addClaudeUsageTokenDelta(dailyDeltas, candidate.aggregateKey(dayBucket), delta)
		}
		if candidate.applyIdentityDelta && candidate.authIndex != "" {
			addClaudeUsageTokenDelta(identityDeltas, claudeUsageTokenIdentityKey{authType: candidate.identityAuthType, identity: candidate.authIndex}, delta)
		}
	}

	if err := applyClaudeBackfillOverviewDeltas(tx, "usage_overview_hourly_stats", hourlyDeltas); err != nil {
		return err
	}
	if err := applyClaudeBackfillOverviewDeltas(tx, "usage_overview_daily_stats", dailyDeltas); err != nil {
		return err
	}
	if err := applyClaudeBackfillIdentityDeltas(tx, identityDeltas); err != nil {
		return err
	}
	return nil
}

type claudeUsageTokenIdentityKey struct {
	authType entities.UsageIdentityAuthType
	identity string
}

type claudeUsageTokenAggregateKey struct {
	bucketStart time.Time
	apiGroupKey string
	model       string
	authIndex   string
	modelAlias  string
}

type claudeUsageTokenDelta struct {
	input  int64
	cached int64
	total  int64
}

type claudeUsageTokenBackfillCandidate struct {
	id                  int64
	identityAuthType    entities.UsageIdentityAuthType
	authIndex           string
	apiGroupKey         string
	model               string
	modelAlias          string
	timestamp           time.Time
	inputTokens         int64
	outputTokens        int64
	reasoningTokens     int64
	cachedTokens        int64
	cacheReadTokens     int64
	cacheCreationTokens int64
	totalTokens         int64
	newInputTokens      int64
	newCachedTokens     int64
	newTotalTokens      int64
	applyOverviewDelta  bool
	applyIdentityDelta  bool
}

func loadClaudeBackfillOverviewCursor(tx *gorm.DB) (int64, error) {
	var lastAggregatedUsageEventID int64
	if err := tx.Table("usage_overview_aggregation_checkpoints").
		Select("COALESCE(last_aggregated_usage_event_id, 0)").
		Where("name = ?", "overview").
		Limit(1).
		Scan(&lastAggregatedUsageEventID).Error; err != nil {
		return 0, fmt.Errorf("load Claude usage token overview cursor: %w", err)
	}
	return lastAggregatedUsageEventID, nil
}

func loadClaudeBackfillIdentities(tx *gorm.DB) (map[claudeUsageTokenIdentityKey]entities.UsageIdentity, error) {
	var rows []entities.UsageIdentity
	if err := tx.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load Claude usage token identities: %w", err)
	}
	identities := make(map[claudeUsageTokenIdentityKey]entities.UsageIdentity, len(rows))
	for _, row := range rows {
		key := claudeUsageTokenIdentityKey{authType: row.AuthType, identity: strings.TrimSpace(row.Identity)}
		identities[key] = row
	}
	return identities, nil
}

func loadClaudeBackfillCandidateEvents(tx *gorm.DB) ([]entities.UsageEvent, error) {
	var events []entities.UsageEvent
	if err := tx.Where("(COALESCE(cache_read_tokens, 0) + COALESCE(cache_creation_tokens, 0)) > 0").
		Where("auth_type IN ?", []string{"oauth", "apikey", "api_key"}).
		Order("id ASC").
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("load Claude usage token backfill events: %w", err)
	}
	return events, nil
}

func buildClaudeUsageTokenBackfillCandidate(event entities.UsageEvent, identities map[claudeUsageTokenIdentityKey]entities.UsageIdentity, overviewLastAggregatedUsageEventID int64) (claudeUsageTokenBackfillCandidate, bool) {
	identityAuthType, ok := claudeUsageTokenIdentityAuthType(event.AuthType)
	if !ok {
		return claudeUsageTokenBackfillCandidate{}, false
	}
	authIndex := strings.TrimSpace(event.AuthIndex)
	identity := identities[claudeUsageTokenIdentityKey{authType: identityAuthType, identity: authIndex}]
	if !isClaudeBackfillEvent(event, identity) {
		return claudeUsageTokenBackfillCandidate{}, false
	}
	candidate := claudeUsageTokenBackfillCandidate{
		id:                  event.ID,
		identityAuthType:    identityAuthType,
		authIndex:           authIndex,
		apiGroupKey:         normalizeClaudeBackfillDimension(event.APIGroupKey, "unknown"),
		model:               normalizeClaudeBackfillDimension(event.Model, "unknown"),
		modelAlias:          normalizeClaudeBackfillModelAlias(event.ModelAlias),
		timestamp:           timeutil.NormalizeStorageTime(event.Timestamp),
		inputTokens:         event.InputTokens,
		outputTokens:        event.OutputTokens,
		reasoningTokens:     event.ReasoningTokens,
		cachedTokens:        event.CachedTokens,
		cacheReadTokens:     event.CacheReadTokens,
		cacheCreationTokens: event.CacheCreationTokens,
		totalTokens:         event.TotalTokens,
		applyOverviewDelta:  event.ID <= overviewLastAggregatedUsageEventID,
		applyIdentityDelta:  event.ID <= identity.LastAggregatedUsageEventID,
	}
	candidate.newInputTokens = max(candidate.proposedInputTokens(), candidate.inputTokens)
	candidate.newCachedTokens = candidate.cacheReadTokens
	candidate.newTotalTokens = max(candidate.proposedTotalTokens(), candidate.totalTokens)
	if candidate.timestamp.IsZero() {
		candidate.applyOverviewDelta = false
	}
	return candidate, true
}

func claudeUsageTokenIdentityAuthType(authType string) (entities.UsageIdentityAuthType, bool) {
	switch strings.TrimSpace(authType) {
	case "oauth":
		return entities.UsageIdentityAuthTypeAuthFile, true
	case "apikey", "api_key":
		return entities.UsageIdentityAuthTypeAIProvider, true
	default:
		return 0, false
	}
}

func isClaudeBackfillEvent(event entities.UsageEvent, identity entities.UsageIdentity) bool {
	switch strings.TrimSpace(event.AuthType) {
	case "oauth":
		return isClaudeBackfillType(event.Provider)
	case "apikey", "api_key":
		return isClaudeBackfillType(identity.Type)
	default:
		return false
	}
}

func isClaudeBackfillType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "claude", "anthropic":
		return true
	default:
		return false
	}
}

func normalizeClaudeBackfillDimension(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func normalizeClaudeBackfillModelAlias(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (candidate claudeUsageTokenBackfillCandidate) shouldRepairCachedInputTokens() bool {
	return candidate.totalTokens == 0 || candidate.cachedTokens != candidate.cacheReadTokens
}

func (candidate claudeUsageTokenBackfillCandidate) cacheInputTokens() int64 {
	return candidate.inputTokens + candidate.cacheReadTokens + candidate.cacheCreationTokens
}

func (candidate claudeUsageTokenBackfillCandidate) fullTotalTokens() int64 {
	return candidate.cacheInputTokens() + candidate.outputTokens + candidate.reasoningTokens
}

func (candidate claudeUsageTokenBackfillCandidate) proposedInputTokens() int64 {
	if !candidate.shouldRepairCachedInputTokens() {
		if candidate.totalTokens > candidate.outputTokens+candidate.reasoningTokens {
			return candidate.totalTokens - candidate.outputTokens - candidate.reasoningTokens
		}
		return candidate.inputTokens
	}
	return max(candidate.cacheInputTokens(), candidate.inputTokens)
}

func (candidate claudeUsageTokenBackfillCandidate) proposedTotalTokens() int64 {
	if !candidate.shouldRepairCachedInputTokens() {
		return candidate.totalTokens
	}
	return max(candidate.fullTotalTokens(), candidate.totalTokens)
}

func (candidate claudeUsageTokenBackfillCandidate) delta() claudeUsageTokenDelta {
	return claudeUsageTokenDelta{
		input:  candidate.newInputTokens - candidate.inputTokens,
		cached: candidate.newCachedTokens - candidate.cachedTokens,
		total:  candidate.newTotalTokens - candidate.totalTokens,
	}
}

func (delta claudeUsageTokenDelta) isZero() bool {
	return delta.input == 0 && delta.cached == 0 && delta.total == 0
}

func (candidate claudeUsageTokenBackfillCandidate) aggregateKey(bucketStart time.Time) claudeUsageTokenAggregateKey {
	return claudeUsageTokenAggregateKey{
		bucketStart: bucketStart,
		apiGroupKey: candidate.apiGroupKey,
		model:       candidate.model,
		authIndex:   candidate.authIndex,
		modelAlias:  candidate.modelAlias,
	}
}

func addClaudeUsageTokenDelta[T comparable](deltas map[T]claudeUsageTokenDelta, key T, delta claudeUsageTokenDelta) {
	current := deltas[key]
	current.input += delta.input
	current.cached += delta.cached
	current.total += delta.total
	deltas[key] = current
}

func updateClaudeBackfillUsageEvent(tx *gorm.DB, candidate claudeUsageTokenBackfillCandidate) error {
	if err := tx.Table("usage_events").
		Where("id = ?", candidate.id).
		Updates(map[string]any{
			"input_tokens":  candidate.newInputTokens,
			"cached_tokens": candidate.newCachedTokens,
			"total_tokens":  candidate.newTotalTokens,
		}).Error; err != nil {
		return fmt.Errorf("update Claude usage token event %d: %w", candidate.id, err)
	}
	return nil
}

func applyClaudeBackfillOverviewDeltas(tx *gorm.DB, table string, deltas map[claudeUsageTokenAggregateKey]claudeUsageTokenDelta) error {
	for key, delta := range deltas {
		if delta.isZero() {
			continue
		}
		result := tx.Table(table).
			Where("bucket_start = ? AND api_group_key = ? AND model = ? AND auth_index = ? AND model_alias = ?", timeutil.FormatStorageTime(key.bucketStart), key.apiGroupKey, key.model, key.authIndex, key.modelAlias).
			Updates(claudeBackfillTokenUpdateMap(delta))
		if result.Error != nil {
			return fmt.Errorf("update Claude usage token %s bucket %s: %w", table, timeutil.FormatStorageTime(key.bucketStart), result.Error)
		}
	}
	return nil
}

func applyClaudeBackfillIdentityDeltas(tx *gorm.DB, deltas map[claudeUsageTokenIdentityKey]claudeUsageTokenDelta) error {
	for key, delta := range deltas {
		if delta.isZero() {
			continue
		}
		result := tx.Table("usage_identities").
			Where("auth_type = ? AND identity = ?", key.authType, key.identity).
			Updates(claudeBackfillTokenUpdateMap(delta))
		if result.Error != nil {
			return fmt.Errorf("update Claude usage token identity %d/%s: %w", key.authType, key.identity, result.Error)
		}
	}
	return nil
}

func claudeBackfillTokenUpdateMap(delta claudeUsageTokenDelta) map[string]any {
	return map[string]any{
		"input_tokens":  gorm.Expr("COALESCE(input_tokens, 0) + ?", delta.input),
		"cached_tokens": gorm.Expr("CASE WHEN COALESCE(cached_tokens, 0) + ? < 0 THEN 0 ELSE COALESCE(cached_tokens, 0) + ? END", delta.cached, delta.cached),
		"total_tokens":  gorm.Expr("COALESCE(total_tokens, 0) + ?", delta.total),
	}
}
