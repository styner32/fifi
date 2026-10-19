# 📈 DCF Valuation Engine - Agent Instructions

## 1. Role & Objective (역할 및 목적)

당신은 Golang 백엔드 환경에서 기업 및 자산의 내재 가치(Intrinsic Value)를 평가하는 **DCF(할인현금흐름) 계산 엔진**을 구현하는 AI 에이전트입니다.
재무 공학 지식과 Go 언어의 강력한 동시성(Concurrency) 패러다임을 결합하여, 빠르고 정확하며 대규모 연산이 가능한 가치 평가 파이프라인을 구축해야 합니다.

---

## 2. Data Modeling (데이터 구조 설계)

DCF 계산을 위해 외부 API나 DB에서 수집할 데이터를 다음의 Go 구조체(`struct`)로 매핑합니다.
_주의: DCF는 본질적으로 '추정치'를 다루므로 속도와 편의성을 위해 `float64`를 기본으로 사용합니다. (단, 회계상 1원 단위의 오차도 허용되지 않는 경우에만 `github.com/shopspring/decimal`을 고려하되, 몬테카를로 시뮬레이션 시 성능 저하에 유의하세요.)_

```go
package dcf

// 1. 기업 재무 데이터 (미래 현금흐름 추정의 뼈대)
type FinancialData struct {
    Revenue      float64 `json:"revenue"`       // 최근 매출액
    EBIT         float64 `json:"ebit"`          // 영업이익
    EffectiveTax float64 `json:"effective_tax"` // 유효 법인세율 (예: 22% -> 0.22)
    DnA          float64 `json:"dna"`           // 감가상각비 및 무형자산상각비
    CapEx        float64 `json:"capex"`         // 자본적 지출
    ChangeInNWC  float64 `json:"change_in_nwc"` // 순운전자본 증감
    SharesOut    float64 `json:"shares_out"`    // 총 유통 주식 수
    NetDebt      float64 `json:"net_debt"`      // 순부채 (총 차입금 - 현금성 자산)
}

// 2. 거시 경제 및 시장 데이터 (할인율/WACC 산출용)
type MarketData struct {
    RiskFreeRate  float64 `json:"risk_free_rate"` // 무위험 수익률 (10년물 국채 금리)
    Beta          float64 `json:"beta"`           // 개별 기업의 주가 변동성
    MarketPremium float64 `json:"market_premium"` // 시장 위험 프리미엄 (MRP)
    CostOfDebt    float64 `json:"cost_of_debt"`   // 타인자본비용 (차입 이자율)
    EquityWeight  float64 `json:"equity_weight"`  // 총자본 중 자기자본 비중
    DebtWeight    float64 `json:"debt_weight"`    // 총자본 중 타인자본 비중
}

// 3. 모델링 가정 (Assumptions)
type Assumptions struct {
    ForecastYears  int     `json:"forecast_years"`  // FCF 추정 기간 (보통 5년)
    TerminalGrowth float64 `json:"terminal_growth"` // 영구 성장률 (보통 1.5% ~ 2.5%, 0.015~0.025)
}
```

## 3. Core Algorithms (핵심 재무 알고리즘)

모든 재무 연산은 상태를 변경하지 않는 순수 함수(Pure Function) 로 분리하여 구현하고, Go의 math 패키지를 활용하세요.

FCF (잉여현금흐름) 산출:
FCF = (EBIT \* (1 - EffectiveTax)) + DnA - CapEx - ChangeInNWC

Ke (자기자본비용, CAPM) 산출:
Ke = RiskFreeRate + (Beta \* MarketPremium)

WACC (가중평균자본비용) 산출:
WACC = (EquityWeight _ Ke) + (DebtWeight _ CostOfDebt \* (1 - EffectiveTax))

TV (영구가치, 고든성장모형) 산출:
TV = (최종 연도 FCF \* (1 + TerminalGrowth)) / (WACC - TerminalGrowth)
(안전장치: WACC <= TerminalGrowth일 경우 분모가 0 또는 음수가 되므로 반드시 Error 처리 로직을 포함할 것)

PV (현재가치) 할인 및 적정 주가 산출:

각 연도의 현금흐름 현재가치: PV = FCF / math.Pow(1+WACC, float64(year))

기업가치(EV): Enterprise Value = PV(FCF의 총합) + PV(TV)

주주가치(Equity Value): Equity Value = EV - NetDebt

1주당 적정주가(Target Price): Target Price = Equity Value / SharesOut

## 4. Go-Specific Implementation (Go 특화 고급 알고리즘 구현)

A. 미래 현금흐름 예측 (Forecasting)
Go는 Python 생태계(Scikit-learn, Prophet)에 비해 머신러닝 패키지가 무겁지 않습니다. 따라서 다음 아키텍처 중 하나를 선택하여 구현하세요.

Go Native (경량 모델): 과거 3~5년 데이터의 CAGR(연평균 성장률) 및 **이동평균(Moving Average)**을 산출하여 향후 5년간의 Revenue, EBIT Margin을 선형 추정(Linear Projection)합니다. (gonum.org/v1/gonum/stat 패키지 활용 가능)

MSA 기반 (고도화 모델): 딥러닝/시계열 기반 예측이 필수라면, 예측 모듈만 Python 기반의 마이크로서비스(FastAPI 등)로 분리하고, Go 에이전트가 gRPC/REST로 추정치 배열([]float64)만 받아오도록 통신 계층을 설계하세요.

B. 몬테카를로 시뮬레이션 (리스크 분석) 🚀 [핵심 강점]
단일 추정값의 오류를 방지하기 위해, Go의 **고루틴(Goroutine)**을 적극 활용하여 수만 번의 시나리오를 고속으로 병렬 처리해야 합니다. Python 대비 압도적인 성능 우위를 확보할 수 있는 구간입니다.

난수 생성: Go 1.22+ 이상의 math/rand/v2에서 제공하는 rand.NormFloat64()를 사용하여 핵심 변수(매출 성장률, WACC, 영구성장률 등)에 정규분포 노이즈를 주입합니다.

동시성 제어: sync.WaitGroup과 버퍼가 있는 채널(channel) 또는 Worker Pool 패턴을 사용해 10,000 ~ 50,000회의 DCF 연산을 동시 실행합니다.

신뢰구간 도출: 채널로 수집된 수만 개의 '적정 주가' 데이터를 슬라이스로 묶고 정렬(sort.Float64s)하여, 하위 10%(비관적), 중앙값 50%, 상위 90%(낙관적)의 신뢰구간 지표를 반환합니다.

C. 역산 DCF (Reverse DCF / Root-finding)
현재 시장에서 거래되는 실제 주가를 목표값(Target)으로 두고, 시장이 기대하는 '내재 성장률'을 역으로 추적합니다.

구현 방식: 외부 최적화 라이브러리 없이, Go의 for 문을 활용한 수치해석의 **이분법(Bisection Method)**을 직접 구현합니다.

성장률 하한선(-50%)과 상한선(100%)을 두고 중앙값으로 산출한 DCF 주가와 '실제 주가'의 오차가 허용 범위(예: epsilon = 0.001) 이내로 수렴할 때까지 범위를 좁혀가며 반복 연산(Iteration)을 수행합니다.

## 5. Recommended Dependencies (권장 패키지 목록)

모듈 개발 시 다음 패키지를 우선적으로 고려하십시오.

표준 라이브러리:

math: 재무 할인 및 복리 연산

math/rand/v2: 스레드 세이프하고 빠른 몬테카를로 난수 생성

sync: 고루틴 병렬 처리 동기화 및 Mutex 자원 락

외부 패키지:

gonum.org/v1/gonum/...: 행렬 연산, 기초 통계 산출, 보간법 등 필요시 도입

github.com/piquette/finance-go: (선택) Yahoo Finance API를 통한 실시간 주가 및 재무 데이터 크롤링 래퍼 라이브러리
