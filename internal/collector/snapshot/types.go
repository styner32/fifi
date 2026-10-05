package snapshot

import (
	"context"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/market/facts"
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
	Clock          func() time.Time
	DomesticStock  DomesticStock
	DomesticFuture DomesticFuture
	Yahoo          YahooQuotes
	Naver          NaverFinance
	KOFIA          KOFIAClient
	Facts          facts.Store
}

type Options struct {
	AsOf                           time.Time // collection start; assigned by Collect
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

// Standardized Status Constants
const (
	StatusPreliminary               QualityStatus = "PRELIMINARY"
	StatusPreliminaryNotReconciled  QualityStatus = "PRELIMINARY_NOT_RECONCILED"
	StatusReconciledProvisional     QualityStatus = "RECONCILED_PROVISIONAL"
	StatusFinal                     QualityStatus = "FINAL"
	StatusPartiallyReconciled       QualityStatus = "PARTIALLY_RECONCILED"
	StatusSourceScopeConflict       QualityStatus = "SOURCE_SCOPE_CONFLICT"
	StatusParsedZeroUnconfirmed     QualityStatus = "PARSED_ZERO_UNCONFIRMED"
	StatusObservedZero              QualityStatus = "OBSERVED_ZERO"
	StatusSourceParserFailure       QualityStatus = "SOURCE_PARSER_FAILURE"
	StatusStaleContextOnly          QualityStatus = "STALE_CONTEXT_ONLY"
	StatusProvisionalLastValid      QualityStatus = "PROVISIONAL_LAST_VALID"
	StatusRawSpread                 QualityStatus = "RAW_SPREAD"
	StatusAlignmentUnverified       QualityStatus = "ALIGNMENT_UNVERIFIED"
	StatusNotABasis                 QualityStatus = "NOT_A_BASIS"
	StatusCrossTimeSpread           QualityStatus = "CROSS_TIME_SPREAD"
	StatusFairValueNotEvaluated     QualityStatus = "FAIR_VALUE_NOT_EVALUATED"
	StatusNotEvaluated              QualityStatus = "NOT_EVALUATED"
	StatusInsufficientTimestamps    QualityStatus = "INSUFFICIENT_TIMESTAMPS"
	StatusExpiredForDay             QualityStatus = "EXPIRED_FOR_DAY"
	StatusTriggerHistoryUnknown     QualityStatus = "TRIGGER_HISTORY_UNKNOWN"
	StatusMixedTimePostClose        QualityStatus = "MIXED_TIME_POST_CLOSE"
	StatusLaggedContextOnly         QualityStatus = "LAGGED_CONTEXT_ONLY"
	StatusModelOutputUnverified     QualityStatus = "MODEL_OUTPUT_UNVERIFIED"
	StatusFuturesIdentityUnverified QualityStatus = "FUTURES_CONTRACT_IDENTITY_UNVERIFIED"
	StatusTimestampMissing          QualityStatus = "TIMESTAMP_MISSING"
	StatusIndicativeQuote           QualityStatus = "INDICATIVE_QUOTE"
	StatusBelowCustomAlertThreshold QualityStatus = "BELOW_CUSTOM_ALERT_THRESHOLD"
)

type LateSessionSection struct {
	BasisOK             bool      `json:"basis_ok"`
	Futures1530OK       bool      `json:"futures_1530_ok"`
	ProgramOK           bool      `json:"program_ok"`
	InstitutionOK       bool      `json:"institution_ok"`
	ForeignOK           bool      `json:"foreign_ok"`
	FuturesStandardCode string    `json:"futures_standard_code"`
	FuturesObservedAt   time.Time `json:"futures_observed_at"`
	SpotObservedAt      time.Time `json:"spot_observed_at"`
	ProgramSourceTime   string    `json:"program_source_time"`
	ProgramStatus       string    `json:"program_status"`

	BusinessDate                  string  `json:"business_date"`
	FuturesContractCode           string  `json:"futures_contract_code,omitempty"`
	FuturesContractMonth          string  `json:"futures_contract_month,omitempty"`
	FuturesExpiryDate             string  `json:"futures_expiry_date,omitempty"`
	FuturesContractIdentityStatus string  `json:"futures_contract_identity_status,omitempty"`
	IsExpiringContract            bool    `json:"is_expiring_contract,omitempty"`
	Kospi200MonthlyOptionsExpiry  bool    `json:"kospi200_monthly_options_expiry,omitempty"`
	Kospi200StandardFuturesExpiry bool    `json:"kospi200_standard_futures_expiry,omitempty"`
	BasisPoint                    float64 `json:"basis_point"`
	BasisRate                     float64 `json:"basis_rate"`
	FuturesPrice                  float64 `json:"futures_price"`
	SpotPrice                     float64 `json:"spot_price"`
	FuturesPrice1530              float64 `json:"futures_price_1530,omitempty"`
	BasisPoint1530                float64 `json:"basis_point_1530,omitempty"`
	BasisAlignmentStatus          string  `json:"basis_alignment_status,omitempty"`
	CrossTimeSpreadStatus         string  `json:"cross_time_spread_status,omitempty"`

	KOSPINetArbitrageTotal           float64  `json:"kospi_net_arbitrage_total"`
	KOSPINetNonArbitrageForeign      float64  `json:"kospi_net_non_arbitrage_foreign"`
	KOSPINetNonArbitrageOrgan        float64  `json:"kospi_net_non_arbitrage_organ"`
	KOSPINetNonArbitrageTotal        float64  `json:"kospi_net_non_arbitrage_total"`
	KOSPIProgramTotalNet             float64  `json:"kospi_program_total_net"`
	OriginalSnapshotArbitrageEok     float64  `json:"original_snapshot_arbitrage_eok"`
	OriginalSnapshotNonArbitrageEok  float64  `json:"original_snapshot_non_arbitrage_eok"`
	OriginalSnapshotTotalEok         float64  `json:"original_snapshot_total_eok"`
	NaverFollowupArbitrageEok        *float64 `json:"naver_followup_arbitrage_eok,omitempty"`
	NaverFollowupNonArbitrageEok     *float64 `json:"naver_followup_non_arbitrage_eok,omitempty"`
	NaverFollowupTotalEok            *float64 `json:"naver_followup_total_eok,omitempty"`
	CrossSourceDifferenceEok         float64  `json:"cross_source_difference_eok,omitempty"`
	CrossSourceStatus                string   `json:"cross_source_status,omitempty"`
	CanonicalTotalEok                *float64 `json:"canonical_total_eok"`
	ProgramReconciledStatus          string   `json:"program_reconciled_status,omitempty"`
	InstitutionNonArbitrageStatus    string   `json:"institution_non_arbitrage_status,omitempty"`
	InstitutionNonArbitrageSemantics string   `json:"institution_non_arbitrage_semantics,omitempty"`

	LateProgramNetEok         *float64            `json:"late_program_net_eok"`
	CloseSessionProgramNetEok *float64            `json:"close_session_program_net_eok"`
	CloseSessionForeignNetEok *float64            `json:"close_session_foreign_net_eok"`
	CloseSessionOrganNetEok   *float64            `json:"close_session_organ_net_eok"`
	PrimaryPattern            string              `json:"primary_pattern"`
	CapitulationScore         *float64            `json:"capitulation_score"`
	ShortSqueezeScore         *float64            `json:"short_squeeze_score"`
	WindowDressingScore       *float64            `json:"window_dressing_score"`
	RebalancingScore          *float64            `json:"rebalancing_score"`
	ExpirationArbitrageScore  *float64            `json:"expiration_arbitrage_score"`
	PatternDetected           bool                `json:"pattern_detected"`
	PatternEvaluated          bool                `json:"pattern_evaluated"`
	PatternReason             string              `json:"pattern_reason,omitempty"`
	PatternMissingInputs      map[string][]string `json:"pattern_missing_inputs,omitempty"`
	Status                    QualityStatus       `json:"status,omitempty"`
	QualityFlags              []string            `json:"quality_flags,omitempty"`
}

type Snapshot struct {
	BusinessDate  string
	SessionStatus string
	CompletedAt   time.Time
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
