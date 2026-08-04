package snapshot

import (
	"context"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
)

type DomesticStock interface {
	InquireIndexDailyPrice(context.Context, string, string) ([]map[string]any, error)
	InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error)
	InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error)
	InquireVKOSPIDailyPrice(context.Context, string, string) ([]map[string]any, error)
	InquireInvestorDailyByMarket(context.Context, string) (*auth.RESTResponse, error)
	InquirePrice(context.Context, string) (*auth.RESTResponse, error)
	KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
	ResolveVKOSPICode(context.Context, []string) (string, error)
	MarketFunds(context.Context, string) (*auth.RESTResponse, error)
	CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error)
	InvestorProgramTradeToday(context.Context, string) (*auth.RESTResponse, error)
	InquireInvestorTimeByMarket(context.Context, string, string) (*auth.RESTResponse, error)
}

type DomesticFuture interface {
	ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error)
	InquirePrice(context.Context, string, string) (*auth.RESTResponse, error)
	InquireTimeFuopChartPrice(ctx context.Context, marketDivCode, inputISCD, hourClsCode, includePastData, includeFakeTick, inputDate, inputHour string) (*auth.RESTResponse, error)
}


type YahooQuotes interface {
	GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error)
	GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error)
}

// NaverFinance provides VKOSPI and domestic index data from Naver Finance.
// TODO: unofficial API — may break; consider KRX official data as long-term alternative.
type NaverFinance interface {
	GetIndexQuote(context.Context, string) (*naver.IndexQuote, error)
	GetIndexDailyHistory(context.Context, string, int) ([]naver.DailyClose, error)
}

type Deps struct {
	DomesticStock  DomesticStock
	DomesticFuture DomesticFuture
	Yahoo          YahooQuotes
	Naver          NaverFinance
	KOFIA          KOFIAClient
}

type Options struct {
	Date                           string
	SidecarStatus                  string
	SidecarTime                    string
	SemiconductorForeignNetSellEok *float64
	MonthlyForeignNetSellEok       *float64
	MonthlyForeignNote             string
	ForeignHoldingChangePP         *float64
	SamsungSKHynixCapRatio         *float64
	USDKRWMonthStart               *float64
	OutputDir                      string
	PulseStoreDir                  string
}


type LateSessionSection struct {
	BusinessDate                  string        `json:"business_date"`
	BasisPoint                    float64       `json:"basis_point"`
	BasisRate                     float64       `json:"basis_rate"`
	FuturesPrice                  float64       `json:"futures_price"`
	SpotPrice                     float64       `json:"spot_price"`
	FuturesPrice1530              float64       `json:"futures_price_1530,omitempty"`
	BasisPoint1530                float64       `json:"basis_point_1530,omitempty"`
	BasisAlignmentStatus          string        `json:"basis_alignment_status,omitempty"` // "VERIFIED" or "ALIGNMENT_UNVERIFIED"

	KOSPINetArbitrageTotal        float64       `json:"kospi_net_arbitrage_total,omitempty"`
	KOSPINetNonArbitrageForeign   float64       `json:"kospi_net_non_arbitrage_foreign"`
	KOSPINetNonArbitrageOrgan     float64       `json:"kospi_net_non_arbitrage_organ"`
	KOSPINetNonArbitrageTotal     float64       `json:"kospi_net_non_arbitrage_total"`
	KOSPIProgramTotalNet          float64       `json:"kospi_program_total_net,omitempty"`
	ProgramReconciledStatus       string        `json:"program_reconciled_status,omitempty"` // "RECONCILED" or "NOT_RECONCILED"

	LateProgramNetEok             *float64      `json:"late_program_net_eok"`
	CloseSessionProgramNetEok     *float64      `json:"close_session_program_net_eok"`
	CloseSessionForeignNetEok     *float64      `json:"close_session_foreign_net_eok"`
	CloseSessionOrganNetEok       *float64      `json:"close_session_organ_net_eok"`
	PrimaryPattern                string        `json:"primary_pattern"`
	CapitulationScore             *float64      `json:"capitulation_score,omitempty"`
	ShortSqueezeScore             *float64      `json:"short_squeeze_score,omitempty"`
	WindowDressingScore           *float64      `json:"window_dressing_score,omitempty"`
	RebalancingScore              *float64      `json:"rebalancing_score,omitempty"`
	ExpirationArbitrageScore      *float64      `json:"expiration_arbitrage_score,omitempty"`
	PatternDetected               bool          `json:"pattern_detected"`
	PatternEvaluated              bool          `json:"pattern_evaluated"`
	PatternReason                 string        `json:"pattern_reason,omitempty"`
	Status                        QualityStatus `json:"status,omitempty"`
	QualityFlags                  []string      `json:"quality_flags,omitempty"`
}


type Snapshot struct {
	Timestamp     time.Time
	Price         *PriceSection
	Flow          *FlowSection
	Impact        *ImpactSection
	Global        *GlobalSection
	Cumulative    *CumulativeSection
	Macro         *MacroSection
	Volatility    *VolatilitySection
	Credit        *CreditSection
	Regime        *RegimeSection
	Concentration *ConcentrationSection
	LateSession   *LateSessionSection
	Errors        map[string]error
}
