package premarket_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fifi/internal/collector/premarket"
)

func TestPublicReportMissingDataContract(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	r := premarket.Collect(context.Background(), premarket.Deps{Clock: func() time.Time { return time.Date(2026, 9, 14, 8, 15, 0, 0, time.FixedZone("KST", 32400)) }}, premarket.Options{StoreDir: dir, NoSave: true})
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["schema_version"] != float64(2) {
		t.Fatal("missing JSON contract version")
	}
	for _, key := range []string{"d_score", "a_score", "s_score", "confidence_pct"} {
		if payload["vul"].(map[string]any)[key] != nil {
			t.Errorf("%s must be null", key)
		}
	}
	if len(r.HardData) != 14 {
		t.Fatalf("got %d rows", len(r.HardData))
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("no-save created a report directory")
	}
}
