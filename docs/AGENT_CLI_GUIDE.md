# Agent CLI Guide (`go run ./cmd/agent report`)

This guide documents the newly added agent report CLI commands under [`cmd/agent`](../cmd/agent). These commands provide real-time market risk diagnostics, leverage indicators, vulnerability metrics, and technical indicators.

---

## Command Overview

```bash
go run ./cmd/agent report <subcommand> [flags...]
```

### Supported Subcommands

| Subcommand | Description | Primary Metric / Output |
| :--- | :--- | :--- |
| **`premarket`** | 개장 전 시장 취약도 지표 보드 (v1.0) | 3-Tier 취약도 벡터 (방향성, 증폭, 확인) & 종합 위험 등급 (`RED`/`AMBER` 등) |
| **`rsi`** | KOSPI / KOSDAQ / 개별 종목 14일 RSI 시계열 엔진 | Wilder/SMA 방식 선택, 타임프레임별(`1d`, `60m`, `15m`, `5m`) RSI 도출 |
| **`credit-balance`** | 시스템 실질 레버리지 (`신용잔고 ÷ 투자자예탁금`) 시계열 | 완충 현금 대비 빚 비중(%), 출처 명시(`CreditSrc`/`DepositSrc`), QC 자동 검증 |
| **`safety-devices`** | 시장 안전장치 (사이드카/서킷브레이커) 모니터링 | 발동 이력, CB1~3 발동선 남은 거리(p, %p), 매도 사이드카 잔여기회 |
| **`intraday-pulse`** | 장중 펄스 및 위험 상태 모니터링 | 실시간 지수, 변동성, 안전장치 통합 관측 |
| **`market-snapshot`** | 시장 종합 스냅샷 리포트 | 주식/선물/옵션/글로벌 통합 스냅샷 |

---

## 1. Premarket Vulnerability Board (`report premarket`)

개장 전 해외 증시, 야간 선물, ADR, 변동성, 미수금/반대매매 현황을 수집하여 시장의 **시스템 취약도(Vulnerability Score)**를 산출합니다.

### Usage

```bash
go run ./cmd/agent report premarket
```

### Features & Output

1. **Tier 1 Directional Vector (방향성 벡터)**:
   - SK하이닉스 ADR(`SKHY_ADR`), 반도체지수(`US_SEMI_COMPOSITE`), 나스닥 야간 디버전스, EWY, NDF 환율 갭.
2. **Tier 2 Amplification Vector (증폭 벡터)**:
   - 신용융자 잔고, 미수금/위탁매매 반대매매 비중, VKOSPI 일간 변동성 $\sigma_{daily} = \text{VKOSPI}/\sqrt{252}$, 250일 백분위, T+2 이코노믹 캘린더 이벤트.
3. **Tier 3 Confirmation Vector (확인 벡터)**:
   - KOR CDS 5Y, 환율-주가 괴리도.
4. **Vulnerability Score Matrix**:
   - $D, A, S$ 가중점수 기반 종합 위험 등급 (`CRITICAL`, `RED`, `AMBER`, `GREEN`) 및 자기검증 Gating (`[SELF-CHECK FAIL]` 감지시 조기 리턴).

---

## 2. RSI Historical Series Engine (`report rsi`)

KOSPI 지수, KOSDAQ 지수 또는 개별 종목의 상대강도지수(RSI) 시계열을 도출합니다.

### Usage

```bash
# KOSPI 지수 14일 RSI (Wilder 평활화 기본)
go run ./cmd/agent report rsi --symbol kospi --period 14

# KOSDAQ 60분봉 14일 RSI (SMA 방식)
go run ./cmd/agent report rsi --symbol kosdaq --interval 60m --method sma

# 특정 종목 (SK하이닉스) JSON 출력
go run ./cmd/agent report rsi --symbol 000660 --days 30 --json
```

### Flags

| Flag | Default | Description |
| :--- | :---: | :--- |
| `--symbol` | `kospi` | 종목 코드 또는 얼라이어스 (`kospi`, `kosdaq`, `samsung`, `skhynix`, `005930` 등) |
| `--period` | `14` | RSI 산출 기간 (일 기본: 14) |
| `--days` | `30` | 조회할 역사적 일수 |
| `--method` | `wilder` | 산출 공식 (`wilder`: 지수평활 / `sma` or `cutler`: 단순평균) |
| `--interval` | `1d` | 봉 타임프레임 주기 (`1d`, `60m`, `15m`, `5m`) |
| `--json` | `false` | JSON 형식 출력 여부 |

#### Interval Timeframe Options (`--interval`)

* **`1d`** (Default): 일봉 데이터 기준 산출 (중장기 추세 분석용)
* **`60m`**: 60분봉 데이터 기준 산출 (시간 단위 추세 및 모멘텀 분석용)
* **`15m`**: 15분봉 데이터 기준 산출 (장중 단기 모멘텀 분석용)
* **`5m`**: 5분봉 데이터 기준 산출 (고빈도 장중 파동 분석용)

Example:
```bash
# 60분봉 14기간 RSI 산출
go run ./cmd/agent report rsi --symbol kospi --period 14 --interval 60m

# 15분봉 14기간 RSI 산출 (Cutler/SMA 방식)
go run ./cmd/agent report rsi --symbol 000660 --period 14 --interval 15m --method sma
```

---

## 3. Real System Leverage Report (`report credit-balance --ratio`)

시장의 완충 현금(투자자예탁금) 대비 빚(신용융자 잔고)의 비율인 **시스템 실질 레버리지 (`신용잔고 ÷ 투자자예탁금`)** 시계열을 도출합니다.

### Usage

```bash
# 최근 30영업일 실질 레버리지 시계열 및 위험 진단
go run ./cmd/agent report credit-balance --ratio

# 최근 60영업일 조회
go run ./cmd/agent report credit-balance --ratio --days 60

# 특정 종목 신용잔고 조회
go run ./cmd/agent report credit-balance 005930 000660
```

### Key Technical Specs

* **지표 산식**: $\text{System Leverage (\%)} = \left(\frac{\text{Credit Balance (조원)}}{\text{Customer Deposit (조원)}}\right) \times 100$
* **컬럼별 출처 분리 (`CreditSrc` / `DepositSrc`)**:
  * `anchor`: 금투협/KRX 공식 공표 수치 (6/24 38.6328조, 7/9 36.63조, 7/27 32.7411조 등)
  * `api`: KOFIA / KIS API 실측 수집치
  * `unreleased_t1`: T+1 시차로 공표 대기 중인 날짜
* **자동 QC 검증 시스템 (`validateLeverageReportQC`)**:
  * QC 1: 합성 선형 감쇠 검정 (`uniquePct >= 5`)
  * QC 2: 예탁금 단위 이봉 검정 (50~200조원 조원 정규화)
  * QC 3: 진단 라벨 엔트로피 검정 (단일 라벨 비중 $\le 80\%$)

---

## 4. Market Safety Devices Monitor (`report safety-devices`)

매도 사이드카(Sidecar) 소진 여부 및 서킷브레이커(Circuit Breaker 1~3단계) 발동선까지의 잔여 거리(p, %p)를 관측합니다.

### Usage

```bash
go run ./cmd/agent report safety-devices
```

### Key Metrics

* **KOSPI 8% (CB1) 발동선**: 5,541.77p (잔여 거리p 및 %p 표시)
* **KOSPI 15% (CB2) 발동선**: 5,119.63p
* **KOSPI 20% (CB3) 발동선**: 4,778.32p
* **사이드카 현황**: 매도 사이드카 소진 여부 및 발동 조건 감시

---

## Verification & Testing

모든 CLI 모듈은 Go 유닛 테스트 수트에 포함되어 있습니다:

```bash
cd go
go test ./...
```
