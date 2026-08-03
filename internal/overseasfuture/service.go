package overseasfuture

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/fifi/internal/auth"
)

const (
	inquirePricePath            = "/uapi/overseas-futureoption/v1/quotations/inquire-price"
	inquirePriceTRID            = "HHDFC55010000"
	futureMasterDownloadURL     = "https://new.real.download.dws.co.kr/common/master/ffcode.mst.zip"
	crudeOilSeriesCodeEnvKey    = "CRUDE_OIL_SRS_CD"
	crudeOilProductCodeEnvKey   = "CRUDE_OIL_PRODUCT_CODE"
	defaultCrudeOilProductCode  = "CL"
	defaultFutureMasterFileName = "ffcode.mst"
)

type Service struct {
	client *auth.KIClient
}

type MasterRecord struct {
	SeriesCode   string
	ExchangeCode string
	ProductCode  string
	IsMostActive bool
	IsRecent     bool
	IsSpread     bool
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func (s *Service) InquirePrice(ctx context.Context, seriesCode string) (*auth.RESTResponse, error) {
	seriesCode = strings.TrimSpace(seriesCode)
	if seriesCode == "" {
		return nil, errors.New("seriesCode is required")
	}

	params := map[string]string{
		"SRS_CD": seriesCode,
	}

	return s.client.Get(ctx, inquirePricePath, inquirePriceTRID, "", params)
}

func (s *Service) ResolveCrudeOilSeriesCode(ctx context.Context) (string, error) {
	if override := strings.TrimSpace(os.Getenv(crudeOilSeriesCodeEnvKey)); override != "" {
		return override, nil
	}

	productCode := strings.TrimSpace(os.Getenv(crudeOilProductCodeEnvKey))
	if productCode == "" {
		productCode = defaultCrudeOilProductCode
	}

	return s.ResolveSeriesCodeByProduct(ctx, productCode)
}

func (s *Service) ResolveSeriesCodeByProduct(ctx context.Context, productCode string) (string, error) {
	productCode = NormalizeProductCode(productCode)
	if productCode == "" {
		return "", errors.New("productCode is required")
	}

	records, err := s.LoadMasterRecords(ctx)
	if err != nil {
		return "", err
	}

	return ResolveSeriesCodeByProductFromRecords(records, productCode)
}

func ResolveSeriesCodeByProductFromRecords(records []MasterRecord, productCode string) (string, error) {
	productCode = NormalizeProductCode(productCode)
	if productCode == "" {
		return "", errors.New("productCode is required")
	}

	candidates := make([]MasterRecord, 0)
	for _, record := range records {
		if !strings.EqualFold(record.ProductCode, productCode) {
			continue
		}
		if record.IsSpread {
			continue
		}
		candidates = append(candidates, record)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no overseas future series code found for product %s", productCode)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftScore := candidateScore(candidates[i])
		rightScore := candidateScore(candidates[j])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if candidates[i].ExchangeCode != candidates[j].ExchangeCode {
			return candidates[i].ExchangeCode < candidates[j].ExchangeCode
		}
		return candidates[i].SeriesCode < candidates[j].SeriesCode
	})

	return candidates[0].SeriesCode, nil
}

func NormalizeProductCode(productCode string) string {
	productCode = strings.ToUpper(strings.TrimSpace(productCode))
	return strings.TrimSuffix(productCode, "=F")
}

func (s *Service) LoadMasterRecords(ctx context.Context) ([]MasterRecord, error) {
	rawZip, err := s.downloadFutureMaster(ctx)
	if err != nil {
		return nil, err
	}

	rawMaster, err := extractZipFile(rawZip, defaultFutureMasterFileName)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(rawMaster))
	records := make([]MasterRecord, 0, 256)
	for scanner.Scan() {
		record, ok := parseMasterRecord(scanner.Bytes())
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, errors.New("ffcode master did not contain any parsable records")
	}

	return records, nil
}

func (s *Service) downloadFutureMaster(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, futureMasterDownloadURL, nil)
	if err != nil {
		return nil, err
	}

	httpClient := http.DefaultClient
	if s.client != nil && s.client.Client != nil {
		httpClient = s.client.Client
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("future master download failed (status %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func extractZipFile(rawZip []byte, fileName string) ([]byte, error) {
	readerAt := bytes.NewReader(rawZip)
	zipReader, err := zip.NewReader(readerAt, int64(len(rawZip)))
	if err != nil {
		return nil, err
	}

	for _, file := range zipReader.File {
		if !strings.EqualFold(file.Name, fileName) {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		return io.ReadAll(fileReader)
	}

	return nil, fmt.Errorf("%s not found in master archive", fileName)
}

func parseMasterRecord(row []byte) (MasterRecord, bool) {
	if len(row) < 107 || len(row) < 92 {
		return MasterRecord{}, false
	}

	record := MasterRecord{
		SeriesCode:   trimField(row[0:32]),
		ExchangeCode: trimField(row[len(row)-92 : len(row)-82]),
		ProductCode:  trimField(row[len(row)-82 : len(row)-72]),
		IsMostActive: trimField(row[len(row)-7:len(row)-6]) == "1",
		IsRecent:     trimField(row[len(row)-6:len(row)-5]) == "1",
		IsSpread:     trimField(row[len(row)-5:len(row)-4]) == "1",
	}

	if record.SeriesCode == "" || record.ProductCode == "" {
		return MasterRecord{}, false
	}

	return record, true
}

func trimField(raw []byte) string {
	return strings.TrimSpace(string(bytes.TrimSpace(raw)))
}

func candidateScore(record MasterRecord) int {
	score := 0
	if record.IsMostActive {
		score += 4
	}
	if record.IsRecent {
		score += 2
	}
	return score
}
