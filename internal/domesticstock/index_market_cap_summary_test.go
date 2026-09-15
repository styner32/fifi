package domesticstock

import (
	"context"
	"encoding/json"
	"github.com/fifi/internal/auth"
	"golang.org/x/text/encoding/korean"
	"os"
	"path/filepath"
	"testing"
)

func indexMasterLine(code, group, pref, member, cap string) string {
	return buildKOSPIMasterLineFull(code, "FIXTURE", group, pref, member, "1", "1", "19990101", cap)
}
func TestIndexUniverseRebuildsFullDenominator(t *testing.T) {
	raw := []byte(indexMasterLine("005930", "ST", "0", "Y", "500") + indexMasterLine("005935", "ST", "1", "N", "100") + indexMasterLine("000660", "ST", "0", "Y", "300") + indexMasterLine("111111", "EF", "0", "N", "900") + indexMasterLine("222222", "RT", "0", "Y", "25") + indexMasterLine("333333", "ST", "0", "N", "800"))
	s, e := buildKOSPIIndexMarketCapSummary(raw, "20260915")
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Constituents) != 3 || s.TotalMarketCap != 825 {
		t.Fatalf("wrong universe or denominator %+v", s)
	}
	for _, x := range s.Constituents {
		if x.Code == "005935" || x.PreferredClass != "0" || !x.KOSPIIndexMember {
			t.Fatalf("invalid member %+v", x)
		}
	}
	// BaseDate is a financial period, never the weighting date.
	if s.BusinessDate != "20260915" || s.WeightStatus != KOSPIIndexWeightStatus {
		t.Fatal("unverified weights presented as official")
	}
}
func TestIndexUniverseRejectsIncompleteOrDuplicateMembers(t *testing.T) {
	for _, bad := range []string{indexMasterLine("005930", "ST", "0", "Y", "0"), indexMasterLine("005930", "ST", "0", "Y", "NaN"), indexMasterLine("005930", "ST", "", "Y", "10"), indexMasterLine("005930", "ST", "1", "Y", "10"), indexMasterLine("005930", "ST", "0", "Y", "10") + indexMasterLine("005930", "ST", "0", "Y", "20"), "truncated"} {
		if _, e := buildKOSPIIndexMarketCapSummary([]byte(bad), "20260915"); e == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
func TestIndexUniverseRefreshesDatedSidecarWithoutChangingSharedScope(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kospi.mst")
	t.Setenv(kospiMasterCacheEnvKey, base)
	date := "20260915"
	raw, _ := korean.EUCKR.NewEncoder().Bytes([]byte(indexMasterLine("005930", "ST", "0", "Y", "500") + indexMasterLine("005935", "ST", "1", "N", "100")))
	path := resolveKOSPIMasterCachePath(base, date)
	os.WriteFile(path, raw, 0600)
	os.WriteFile(resolveKOSPIMasterJSONPath(path), []byte(`{"business_date":"20260915","records":[]}`), 0600)
	svc := NewService(auth.NewKIClient("fixture", "fixture", "https://example.test", ""))
	all, e := svc.KOSPIMarketCapSummary(context.Background(), date)
	if e != nil || all.TotalMarketCap != 600 {
		t.Fatalf("market-wide scope changed %+v %v", all, e)
	}
	idx, e := svc.KOSPIIndexMarketCapSummary(context.Background(), date)
	if e != nil || idx.TotalMarketCap != 500 {
		t.Fatalf("index scope wrong %+v %v", idx, e)
	}
	b, _ := os.ReadFile(resolveKOSPIMasterJSONPath(path))
	var cached kospiMasterJSONCache
	json.Unmarshal(b, &cached)
	if cached.SchemaVersion != 2 || len(cached.Records) != 2 || cached.Records[1].PreferredClass != "1" || cached.Records[1].KOSPIIndexMember {
		t.Fatal("classification not preserved in sidecar")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(raw) {
		t.Fatal("master changed")
	}
}
