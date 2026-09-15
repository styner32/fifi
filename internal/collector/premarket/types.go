package premarket

import (
	"context"
	"time"

	"github.com/fifi/internal/auth"

	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
)

type DomesticStock interface {
	MarketTime(context.Context) (*auth.RESTResponse, error)
	InquireIndexDailyPrice(context.Context, string, string) ([]map[string]any, error)
	InquireVKOSPIDailyPrice(context.Context, string, string) ([]map[string]any, error)
	ResolveVKOSPICode(context.Context, []string) (string, error)
	MarketFunds(context.Context, string) (*auth.RESTResponse, error)
	InquireInvestorDailyByMarket(context.Context, string) (*auth.RESTResponse, error)
}

type YahooQuotes interface {
	GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error)
	GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error)
}

type NaverFinance interface {
	GetIndexDailyHistory(context.Context, string, int) ([]naver.DailyClose, error)
}

type KOFIAClient interface {
	GetMarketFundsForDate(context.Context, string) (*kofia.MarketFundsRow, error)
}

type Deps struct {
	Stock       DomesticStock
	Yahoo       YahooQuotes
	Naver       NaverFinance
	KOFIA       KOFIAClient
	Clock       func() time.Time
	StoreDir    string
	DailyPrices DailyPrices
	Calendar    EventCalendar
}

type Options struct {
	StoreDir string
	Date     string
	JSON     bool
	NoSave   bool
}

type SemiMember struct {
	BusinessDate string   `json:"business_date,omitempty"`
	Symbol       string   `json:"symbol"`
	Name         string   `json:"name"`
	ChangePct    *float64 `json:"change_pct"`
	Weight       float64  `json:"weight"`
	IsAvailable  bool     `json:"is_available"`
}

type Tier1Direction struct {
	Status                string       `json:"status"`
	SemiWeightCoveragePct float64      `json:"semi_weight_coverage_pct"`
	SKHYClose             *float64     `json:"skhy_close"`
	SKHYRet               *float64     `json:"skhy_ret"`
	SKHYPremium           *float64     `json:"skhy_premium"`
	SKHYPremiumChg        *float64     `json:"skhy_premium_chg"`
	HasSKHYShock          *bool        `json:"has_skhy_shock"`
	USHoliday             *bool        `json:"us_holiday"`
	SemiComposite         *float64     `json:"semi_composite"`
	SemiMembers           []SemiMember `json:"semi_members"`
	NQ100Change           *float64     `json:"nq100_change"`
	Divergence            *float64     `json:"divergence"`
	HasDivAlert           *bool        `json:"has_div_alert"`
	EWYClose              *float64     `json:"ewy_close"`
	EWYChange             *float64     `json:"ewy_change"`
	EWYVolumeRatio        *float64     `json:"ewy_volume_ratio"`
	HasEWYFlowEvent       *bool        `json:"has_ewy_flow_event"`
	NDFClose              *float64     `json:"ndf_close"`
	NDFGap                *float64     `json:"ndf_gap"`
	VIX                   *float64     `json:"vix"`
	DScore                *int         `json:"d_score"`
	QualityFlags          []string     `json:"quality_flags,omitempty"`
}

type EchoEvent struct {
	SourceDate string   `json:"source_date"`
	TargetDate string   `json:"target_date"`
	SourceDrop *float64 `json:"source_drop"`
	Pressure   *float64 `json:"pressure"`
}

type Tier2Amplification struct {
	AStatus              string      `json:"a_status"`
	SStatus              string      `json:"s_status"`
	CreditSampleCount    int         `json:"credit_sample_count"`
	VKOSPISampleCount    int         `json:"vkospi_sample_count"`
	CreditLoanBalanceEok *float64    `json:"credit_loan_balance_eok"`
	CreditLoanPctile     *float64    `json:"credit_loan_pctile"`
	MarginReceivableEok  *float64    `json:"margin_receivable_eok"`
	ForcedSellAmountEok  *float64    `json:"forced_sell_amount_eok"`
	ForcedSellRatioPct   *float64    `json:"forced_sell_ratio_pct"`
	CustomerDepositEok   *float64    `json:"customer_deposit_eok"`
	Deposit5DayTrendEok  *float64    `json:"deposit_5day_trend_eok"`
	DepositPeakDropPct   *float64    `json:"deposit_peak_drop_pct"`
	LevTurnoverRatio     *float64    `json:"lev_turnover_ratio"`
	VKOSPI               *float64    `json:"vkospi"`
	SigmaDaily           *float64    `json:"sigma_daily"`
	SpreadVIX            *float64    `json:"spread_vix"`
	VKOSPIPctile250d     *float64    `json:"vkospi_pctile_250d"`
	GapMA120             *float64    `json:"gap_ma120"`
	GapMA200             *float64    `json:"gap_ma200"`
	HasMA120Proximity    *bool       `json:"has_ma120_proximity"`
	ProximitySamsung     *float64    `json:"proximity_samsung"`
	ProximityHynix       *float64    `json:"proximity_hynix"`
	HasMarginCascade     *bool       `json:"has_margin_cascade"`
	EchoCalendar         []EchoEvent `json:"echo_calendar"`
	AScore               *int        `json:"a_score"`
	SScore               *int        `json:"s_score"`
	QualityFlags         []string    `json:"quality_flags,omitempty"`
}

type Tier3Character struct {
	Status            string   `json:"status"`
	CDS5Y             *float64 `json:"cds_5y"`
	HasCapitalFlight  *bool    `json:"has_capital_flight"`
	ForeignFuturesOI  *float64 `json:"foreign_futures_oi"`
	DivergencePhase   string   `json:"divergence_phase"`
	DivergenceCaption string   `json:"divergence_caption"`
	QualityFlags      []string `json:"quality_flags,omitempty"`
}

type VulnerabilityMatrix struct {
	CoveragePct   float64  `json:"coverage_pct"`
	MissingInputs []string `json:"missing_inputs"`
	DScore        *int     `json:"d_score"`
	AScore        *int     `json:"a_score"`
	SScore        *int     `json:"s_score"`
	OverallGrade  string   `json:"overall_grade"` // "CRITICAL", "RED", "AMBER", "GREEN"
	ConfidencePct *float64 `json:"confidence_pct"`
	MissingCount  int      `json:"missing_count"`
	TotalFields   int      `json:"total_fields"`
	SelfCheckFail *bool    `json:"self_check_fail"`
	Suppressed    bool     `json:"suppressed"`
}

type HardDataRow struct {
	Item      string `json:"item"`
	Value     string `json:"value"`
	ChangeDir string `json:"change_dir"`
	RefTime   string `json:"ref_time"`
}

type PremarketReport struct {
	SchemaVersion     int                    `json:"schema_version"`
	CutoffAt          *time.Time             `json:"cutoff_at"`
	DomesticCloseDate string                 `json:"domestic_close_date"`
	Inputs            map[string]Observation `json:"inputs"`
	DomesticInputs    map[string]Observation `json:"domestic_inputs"`
	Calendar          CalendarReport         `json:"calendar"`
	Errors            map[string]string      `json:"errors,omitempty"`
	Timestamp         time.Time              `json:"timestamp"`
	Date              string                 `json:"date"`
	HardData          []HardDataRow          `json:"hard_data"`
	Tier1             Tier1Direction         `json:"tier1"`
	Tier2             Tier2Amplification     `json:"tier2"`
	Tier3             Tier3Character         `json:"tier3"`
	VUL               VulnerabilityMatrix    `json:"vul"`
}

// Nil means missing; zero is an observed numeric value. Daily bar timestamps
// identify the provider bar, not necessarily a closing execution tick.
type Observation struct {
	Value            *float64   `json:"value"`
	ChangePct        *float64   `json:"change_pct"`
	PreviousClose    *float64   `json:"previous_close"`
	Source           string     `json:"source"`
	Unit             string     `json:"unit"`
	BusinessDate     string     `json:"business_date,omitempty"`
	ReferenceDate    string     `json:"reference_date,omitempty"`
	ReferenceEndDate string     `json:"reference_end_date,omitempty"`
	SampleCount      int        `json:"sample_count,omitempty"`
	AsOf             *time.Time `json:"as_of,omitempty"`
	ReferenceAsOf    *time.Time `json:"reference_as_of,omitempty"`
	FetchedAt        time.Time  `json:"fetched_at"`
	Session          string     `json:"session"`
	Status           string     `json:"status"`
	Reason           string     `json:"reason,omitempty"`
}

func (o Observation) available() bool {
	return o.Value != nil && finite(*o.Value) && (o.Status == "VALID" || o.Status == "LAGGED_CONTEXT_ONLY")
}
