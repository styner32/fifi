package snapshot

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRenderEmptySnapshotShowsAllSectionsWithNA(t *testing.T) {
	s := &Snapshot{
		Timestamp: time.Date(2026, 5, 15, 15, 30, 0, 0, time.FixedZone("KST", 9*60*60)),
		Errors: map[string]error{
			"price": errors.New("price failed"),
			"flow":  errors.New("flow failed"),
		},
	}
	out := Render(s)
	for _, want := range []string{
		"# Market Snapshot — 2026-05-15 15:30 KST",
		"## 1. 일중 가격 흐름",
		"## 2. 수급 (코스피)",
		"## 3. 시장 충격 지표",
		"## 4. 글로벌 동조성 (전일 대비)",
		"## 5. 누적 지표",
		"## 6. 매크로",
		"n/a",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() missing %q:\n%s", want, out)
		}
	}
}

func TestRenderKeepsAvailableSectionsWhenOtherSectionsFail(t *testing.T) {
	s := &Snapshot{
		Timestamp: time.Date(2026, 5, 15, 15, 30, 0, 0, time.FixedZone("KST", 9*60*60)),
		Flow:      &FlowSection{ForeignEok: -10, InstitutionEok: -20, IndividualEok: 30},
		Errors:    map[string]error{"price": errors.New("price failed")},
	}
	out := Render(s)
	if !strings.Contains(out, "| 외국인 | -10 |") || !strings.Contains(out, "n/a (price failed)") {
		t.Fatalf("Render() did not preserve partial output:\n%s", out)
	}
}
