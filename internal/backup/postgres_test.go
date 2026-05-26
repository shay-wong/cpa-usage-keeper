package backup

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestPostgresDumpCommandArgsUseFastCompressionAndRedactPassword(t *testing.T) {
	args, env, err := postgresDumpCommandArgs("postgres://keeper:secret@postgres:5432/cpa_usage_keeper?sslmode=prefer", "/backups/database.dump", Options{UsageLogs: true})
	if err != nil {
		t.Fatalf("postgresDumpCommandArgs returned error: %v", err)
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "secret") {
		t.Fatalf("expected password to be removed from dump args, got %q", joinedArgs)
	}
	for _, required := range []string{"--format=custom", "--compress=1", "--file /backups/database.dump", "keeper@postgres:5432"} {
		if !strings.Contains(joinedArgs, required) {
			t.Fatalf("expected dump args to include %q, got %q", required, joinedArgs)
		}
	}
	if len(env) != 1 || env[0] != "PGPASSWORD=secret" {
		t.Fatalf("expected password passed through PGPASSWORD env, got %+v", env)
	}
}

func TestPostgresConnectionArgsRedactsPasswordFromDatabaseArgument(t *testing.T) {
	args, env, err := postgresConnectionArgs("postgres://keeper:secret@postgres:5432/cpa_usage_keeper?sslmode=prefer")
	if err != nil {
		t.Fatalf("postgresConnectionArgs returned error: %v", err)
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "secret") {
		t.Fatalf("expected password to be removed from command args, got %q", joinedArgs)
	}
	if !strings.Contains(joinedArgs, "keeper@postgres:5432") {
		t.Fatalf("expected database args to keep safe connection context, got %q", joinedArgs)
	}
	if len(env) != 1 || env[0] != "PGPASSWORD=secret" {
		t.Fatalf("expected password passed through PGPASSWORD env, got %+v", env)
	}
}

func TestPostgresRestoreSQLCommandArgsDoNotConnectToDatabase(t *testing.T) {
	args := postgresRestoreSQLCommandArgs("/backups/database.dump", Options{APIKeys: true})
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "secret") || strings.Contains(joinedArgs, "--dbname") || strings.Contains(joinedArgs, "postgres://") {
		t.Fatalf("expected SQL restore args not to include credentials or direct database connection, got %q", joinedArgs)
	}
	for _, required := range []string{"--data-only", "--file -", "--table cpa_api_keys", "/backups/database.dump"} {
		if !strings.Contains(joinedArgs, required) {
			t.Fatalf("expected SQL restore args to include %q, got %q", required, joinedArgs)
		}
	}
}

func TestPostgresRestoreSQLWrapsTruncateAndDataImportInSingleTransactionInput(t *testing.T) {
	var input bytes.Buffer
	writePostgresRestoreTransactionInput(&input, Options{UsageLogs: true}, []byte("COPY usage_events (id) FROM stdin;\n\\.\n"))
	got := input.String()
	if !strings.Contains(got, `TRUNCATE TABLE "usage_events" RESTART IDENTITY CASCADE;`) {
		t.Fatalf("expected restore transaction to truncate selected table, got %q", got)
	}
	if !strings.Contains(got, "COPY usage_events") {
		t.Fatalf("expected restore transaction to include pg_restore data SQL, got %q", got)
	}
}

func TestPostgresTablesForOptionsMapsStorageDomains(t *testing.T) {
	got := PostgresTablesForOptions(Options{UsageLogs: true, ModelPrices: true})
	want := []string{
		"usage_events",
		"usage_overview_hourly_stats",
		"usage_overview_daily_stats",
		"usage_overview_health_stats",
		"usage_overview_aggregation_checkpoints",
		"model_price_settings",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected postgres table mapping: got %+v want %+v", got, want)
	}
}

func TestPostgresExcludedTableDataMapsUnselectedDomains(t *testing.T) {
	got := postgresExcludedTableData(Options{UsageLogs: true, ModelPrices: true})
	want := []string{"usage_request_details", "usage_identities", "cpa_api_keys", "redis_usage_inboxes"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected postgres excluded tables: got %+v want %+v", got, want)
	}
}
