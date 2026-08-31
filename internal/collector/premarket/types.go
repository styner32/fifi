package premarket

import (
	"context"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
)

type DomesticStock interface {
	InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error)
	InquirePrice(context.Context, string) (*auth.RESTResponse, error)
	InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error)
	ResolveVKOSPICode(context.Context, []string) (string, error)
	KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
	MarketFunds(context.Context, string) (*auth.RESTResponse, error)
}

type DomesticFuture interface {
	ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error)
	InquirePrice(context.Context, string, string) (*auth.RESTResponse, error)
}

type YahooQuotes interface {
	GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error)
	GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error)
}

type NaverFinance interface {
	GetIndexQuote(context.Context, string) (*naver.IndexQuote, error)
}

type KOFIAClient interface {
	GetMarketFundsForDate(context.Context, string) (*kofia.MarketFundsRow, error)
}

type Deps struct {
	Stock    DomesticStock
	Future   DomesticFuture
	Yahoo    YahooQuotes
	Naver    NaverFinance
	KOFIA    KOFIAClient
	Clock    func() time.Time
	StoreDir string
}

type Options struct {
	StoreDir string
	Date     string
	JSON     bool
	NoSave   bool
}

type SemiMember struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	ChangePct  float64 `json:"change_pct"`
	Weight     float64 `json:"weight"`
	IsAvailable bool   `json:"is_available"`
}

type Tier1Direction struct {
	SKHYClose        float64      `json:"skhy_close"`
	SKHYRet          float64      `json:"skhy_ret"`
	SKHYPremium      float64      `json:"skhy_premium"`
	SKHYPremiumChg   float64      `json:"skhy_premium_chg"`
	HasSKHYShock     bool         `json:"has_skhy_shock"`
	USHoliday        bool         `json:"us_holiday"`
	SemiComposite    float64      `json:"semi_composite"`
	SemiMembers      []SemiMember `json:"semi_members"`
	NQ100Change      float64      `json:"nq100_change"`
	Divergence       float64      `json:"divergence"`
	HasDivAlert      bool         `json:"has_div_alert"`
	EWYClose         float64      `json:"ewy_close"`
	EWYChange        float64      `json:"ewy_change"`
	EWYVolumeRatio   float64      `json:"ewy_volume_ratio"`
	HasEWYFlowEvent  bool         `json:"has_ewy_flow_event"`
	NDFClose         float64      `json:"ndf_close"`
	NDFGap           float64      `json:"ndf_gap"`
	VIX              float64      `json:"vix"`
	DScore           int          `json:"d_score"`
	QualityFlags     []string     `json:"quality_flags,omitempty"`
}

type EchoEvent struct {
	SourceDate  string  `json:"source_date"`
	TargetDate  string  `json:"target_date"`
	SourceDrop  float64 `json:"source_drop"`
	Pressure    float64 `json:"pressure"`
}

type Tier2Amplification struct {
	CreditLoanBalanceEok float64     `json:"credit_loan_balance_eok"`
	CreditLoanPctile     float64     `json:"credit_loan_pctile"`
	MarginReceivableEok  float64     `json:"margin_receivable_eok"`
	ForcedSellAmountEok  float64     `json:"forced_sell_amount_eok"`
	ForcedSellRatioPct   float64     `json:"forced_sell_ratio_pct"`
	CustomerDepositEok   float64     `json:"customer_deposit_eok"`
	Deposit5DayTrendEok  float64     `json:"deposit_5day_trend_eok"`
	DepositPeakDropPct   float64     `json:"deposit_peak_drop_pct"`
	LevTurnoverRatio     float64     `json:"lev_turnover_ratio"`
	VKOSPI               float64     `json:"vkospi"`
	SigmaDaily           float64     `json:"sigma_daily"`
	SpreadVIX            float64     `json:"spread_vix"`
	VKOSPIPctile250d     float64     `json:"vkospi_pctile_250d"`
	GapMA120             float64     `json:"gap_ma120"`
	GapMA200             float64     `json:"gap_ma200"`
	HasMA120Proximity    bool        `json:"has_ma120_proximity"`
	ProximitySamsung     float64     `json:"proximity_samsung"`
	ProximityHynix       float64     `json:"proximity_hynix"`
	HasMarginCascade     bool        `json:"has_margin_cascade"`
	EchoCalendar         []EchoEvent `json:"echo_calendar"`
	AScore               int         `json:"a_score"`
	SScore               int         `json:"s_score"`
	QualityFlags         []string    `json:"quality_flags,omitempty"`
}

type Tier3Character struct {
	CDS5Y             float64  `json:"cds_5y"`
	HasCapitalFlight  bool     `json:"has_capital_flight"`
	ForeignFuturesOI  float64  `json:"foreign_futures_oi"`
	DivergencePhase   string   `json:"divergence_phase"`
	DivergenceCaption string   `json:"divergence_caption"`
	QualityFlags      []string `json:"quality_flags,omitempty"`
}

type VulnerabilityMatrix struct {
	DScore         int      `json:"d_score"`
	AScore         int      `json:"a_score"`
	SScore         int      `json:"s_score"`
	OverallGrade   string   `json:"overall_grade"` // "CRITICAL", "RED", "AMBER", "GREEN"
	ConfidencePct  float64  `json:"confidence_pct"`
	MissingCount   int      `json:"missing_count"`
	TotalFields    int      `json:"total_fields"`
	SelfCheckFail  bool     `json:"self_check_fail"`
	Suppressed     bool     `json:"suppressed"`
}

type PremarketReport struct {
	Timestamp time.Time           `json:"timestamp"`
	Date      string              `json:"date"`
	Tier1     Tier1Direction      `json:"tier1"`
	Tier2     Tier2Amplification  `json:"tier2"`
	Tier3     Tier3Character      `json:"tier3"`
	VUL       VulnerabilityMatrix `json:"vul"`
}
