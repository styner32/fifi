package domesticfutureoption

import "github.com/fifi/internal/auth"

const (
	inquirePricePath                = "/uapi/domestic-futureoption/v1/quotations/inquire-price"
	inquireTimeFuopCCNLPath         = "/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopccnl"
	inquireMemberPath               = "/uapi/domestic-futureoption/v1/quotations/inquire-member"
	inquireTimeFuopChartPricePath   = "/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopchartprice"
	displayBoardTopPath             = "/uapi/domestic-futureoption/v1/quotations/display-board-top"
	displayBoardFuturesPath         = "/uapi/domestic-futureoption/v1/quotations/display-board-futures"
	expPriceTrendPath               = "/uapi/domestic-futureoption/v1/quotations/exp-price-trend"
	indexFutureMasterDownloadURL    = "https://new.real.download.dws.co.kr/common/master/fo_idx_code_mts.mst.zip"
	indexFutureMasterFilename       = "fo_idx_code_mts.mst"
	defaultIndexFutureMasterCache   = ".cache/fo_idx_code_mts.mst"
	quadWitchingFuturesCodeEnvKey   = "QUAD_WITCHING_FUTURES_CODE"
	indexFutureMasterCacheEnvKey    = "INDEX_FUTURE_MASTER_CACHE_FILE"
	defaultFuturesMarketDivCode     = "F"
	defaultBoardScreenCode          = "20503"
	defaultBoardMarketClassCode     = "MKI"
	defaultTimeChartHourClassCode   = "60"
	defaultTimeChartIncludePastData = "Y"
	defaultTimeChartIncludeFakeTick = "N"
)

const (
	inquirePriceTRID              = "FHMIF10000000"
	inquireTimeFuopCCNLTRID       = "FHMIF10020000"
	inquireMemberTRID             = "FHMIF10070000"
	inquireTimeFuopChartPriceTRID = "FHKIF03020200"
	displayBoardTopTRID           = "FHPIF05030000"
	displayBoardFuturesTRID       = "FHPIF05030200"
	expPriceTrendTRID             = "FHPIF05110100"
)

type Service struct {
	client *auth.KIClient
}

type MasterRecord struct {
	InfoType            string `json:"info_type"`
	ShortCode           string `json:"short_code"`
	StandardCode        string `json:"standard_code"`
	Name                string `json:"name"`
	ATMClassCode        string `json:"atm_class_code"`
	StrikePrice         string `json:"strike_price"`
	MonthClassCode      string `json:"month_class_code"`
	UnderlyingShortCode string `json:"underlying_short_code"`
	UnderlyingName      string `json:"underlying_name"`
}

type ResolvedContract struct {
	BusinessDate    string       `json:"business_date"`
	Source          string       `json:"source"`
	MasterCachePath string       `json:"master_cache_path,omitempty"`
	MasterJSONPath  string       `json:"master_json_path,omitempty"`
	Record          MasterRecord `json:"record"`
}

type masterJSONField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type masterJSONCache struct {
	BusinessDate string            `json:"business_date"`
	GeneratedAt  string            `json:"generated_at"`
	SourcePath   string            `json:"source_path"`
	RecordCount  int               `json:"record_count"`
	Fields       []masterJSONField `json:"fields"`
	Records      []MasterRecord    `json:"records"`
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}
