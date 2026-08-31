# 근실시간 시장 펄스 (Intraday Market Pulse) — 구현 설계서

> 최근 1–2시간 내 KOSPI/KOSDAQ 변동을 **수급·환율·미국선물**과 묶어 분석하는 근실시간 CLI.
> `make takesnapshot`(일간 종합 스냅샷)과 **다른** 도구.

---

## 1. 목표와 `takesnapshot`과의 차이

| 구분 | `make takesnapshot` (기존) | `make now` (신규, 본 설계) |
|---|---|---|
| 시점 | 하루 1회, **일 단위** 종합 | 수시 실행, **최근 1–2시간 구간** |
| 비교축 | **전일** 스냅샷과 비교 | **같은 날 1–2시간 전** 적립분과 비교 |
| 수급 | 당일 누적 순매수 (정적) | 최근 1–2h **델타(가속/둔화)** |
| 지수 | 일간 OHLC/등락 | 최근 1–2h **구간 변동** (분봉) |
| 핵심질문 | "오늘 시장은 어땠나" | "**방금 무엇이 움직였고, 무엇이 그걸 끌었나**" |

핵심 차별점: 시간대별 수급(`inquire-investor-time-by-market`)은 **시계열이 아니라 "현재까지 누적된 1행"** 만 반환한다(아래 §3.A 프로브 결과). 따라서 본 도구는 **실행할 때마다 펄스를 적립(persist)** 하고, **1–2시간 전 적립분과 차분**하여 구간 델타를 만든다. 지수·환율·미국선물은 Yahoo 5분봉으로 **첫 실행에도** 구간 변동을 즉시 계산한다.

---

## 2. CLI 인터페이스

- Makefile 타깃: **`make now`**
- 내부 실행: `go run ./cmd/agent report intraday-pulse [flags]`
- `cmd/agent/main.go`의 `run()` switch에 `case "intraday-pulse": return runIntradayPulse(args[2:])` 추가.

플래그(모두 선택):

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--store-dir` | `.cache/pulse` (env `PULSE_OUTPUT_DIR`) | 펄스 적립 디렉터리 |
| `--lookback` | `2h` | 분석 최대 룩백 (1h/2h 둘 다 산출) |
| `--no-save` | false | 적립/렌더 저장 생략(읽기 전용 분석) |
| `--no-history` (`--no-prev`) | false | 이전 적립/누적 레코드 무시 (독립 실행 및 디버깅용) |
| `--json` | false | 사람용 리포트 대신 JSON만 출력 |

Makefile 추가:
```make
now:
	go run ./cmd/agent report intraday-pulse
```

`runIntradayPulse`는 기존 `runMarketSnapshot`과 동일하게 KIS 클라이언트/Yahoo 클라이언트를 구성한다(인증 토큰 `EnsureAuthToken`, `auth.NewKIClient`, `yahoo.NewClient`). KOFIA/Naver는 불필요.

---

## 3. 데이터 소스 명세 (라이브 프로브로 검증, 2026-06-23 12:38 KST 장중)

### A. 수급 — KIS `inquire-investor-time-by-market` (TRID `FHPTJ04030000`)

`domesticstock.Service.InquireInvestorTimeByMarket(ctx, marketDiv, indexDiv)` 이미 존재.

- KOSPI: `marketDiv="KSP"`, `indexDiv="0001"`
- KOSDAQ: `marketDiv="KSQ"`, `indexDiv="1001"`

**반환: `output` = 1개 row(누적 스냅샷, 시계열 아님).** 금액 필드는 **백만원(million KRW)** → `/100 = 억원`.

| 필드 (`*_ntby_tr_pbmn`) | 주체 | 예시(KOSPI, 백만원) |
|---|---|---|
| `frgn` | 외국인 | -3,556,053 → **-3.56조** |
| `orgn` | 기관계 | -1,540,481 → **-1.54조** |
| `prsn` | 개인 | +5,103,306 → **+5.10조** |
| `scrt` | 금융투자 | -996,696 |
| `ivtr` | 투신 | -128,601 |
| `fund` | 연기금 등 | -128,970 |
| `pe_fund` | 사모 | -208,066 |
| `insu` | 보험 | -59,629 |
| `bank` | 은행 | -14,571 |
| `mrbn` | 기타금융 | -3,947 |
| `etc_corp` | 기타법인 | -6,772 |

(`*_seln_tr_pbmn` 매도, `*_shnu_tr_pbmn` 매수, `*_ntby_qty` 순매수수량도 존재하나 본 도구는 `ntby_tr_pbmn`만 사용.)

### B. 지수 — KIS `inquire-index-price` (TRID `FHPUP02100000`)

`domesticstock.Service.InquireIndexPrice(ctx, indexCode)` 이미 존재. KOSPI `0001`, KOSDAQ `1001`.

**반환: `output` = 단일 객체(배열 아님).**

| 필드 | 의미 | 예시(KOSPI) |
|---|---|---|
| `bstp_nmix_prpr` | 현재가 | 8565.06 |
| `bstp_nmix_prdy_vrss` | 전일대비(p) | -549.49 |
| `bstp_nmix_prdy_ctrt` | 전일대비(%) | -6.03 |
| `bstp_nmix_oprc/hgpr/lwpr` | 시/고/저 | 9083.54 / 9175.45 / 8511.14 |
| `acml_tr_pbmn` | 누적거래대금(백만원) → /100 억 | 36,585,627 → **36.6조** |
| `ascn_issu_cnt` / `down_issu_cnt` / `stnr_issu_cnt` | 상승/하락/보합 종목수 | 87 / 823 / 8 |

전일 종가 = `prpr - prdy_vrss`.

### C. 환율·미국선물·매크로 — Yahoo (`yahoo.Client`)

- `GetQuotes([]symbol)` → `map[symbol]Quote{Price, PreviousClose, ChangePercent}` (현재가 + 전일대비%).
- `GetChartHistory(symbol, "1d", "5m")` → `[]DailyClose{DateUnix, Close}` (unix 오름차순). **5분봉으로 최근 1–2h 구간 변동 계산.**

검증된 심볼:

| 심볼 | 라벨 | 비고 |
|---|---|---|
| `^KS11` | KOSPI(분봉) | 46pt, 09:00–현재. 지수 구간변동용 |
| `^KQ11` | KOSDAQ(분봉) | 〃 |
| `KRW=X` | 원/달러 | **희소(54pt, 약 5–10분 불규칙)** → 타깃 인접점 선택 필요 |
| `NQ=F` | 나스닥100 선물 | 272pt(~24h) |
| `ES=F` | S&P500 선물 | 〃 |
| `YM=F` | 다우 선물 | 〃 |
| `^N225` | 닛케이225 | 아시아 동조성 |
| `CL=F` | WTI 원유 | 에너지/인플레 |
| `^TNX` | 미국채 10Y | 금리→외국인/환율 |

> ⚠️ 선물(`NQ=F` 등)은 ~24시간 연속이라 5분봉 "1d"가 **전일~당일을 가로지른다.** 구간 변동은 반드시 **절대 unix 타임스탬프 기준**으로, 시리즈의 **마지막 점**을 앵커로 (lastTS − 1h/2h) 지점을 찾아 계산한다(달력 날짜로 자르지 말 것).

---

## 4. 패키지/파일 구조

신규 패키지 `internal/collector/pulse/` (기존 `snapshot` 패턴 답습):

| 파일 | 책임 |
|---|---|
| `types.go` | `Deps`, `Pulse`, `Market`, `FlowSnapshot`, `IndexLevel`, `Window`, `FlowDelta` 인터페이스/구조체 |
| `util.go` | `rowsOf/firstRowOf/numOf`(문자열 숫자 파싱, 콤마 제거), `fmtEok/fmtPct/arrow` |
| `flow.go` | `collectFlow(market)` — §3.A 파싱, 백만원→억 |
| `index.go` | `collectIndex(indexCode)` — §3.B 파싱 |
| `market.go` | Yahoo `GetQuotes`+`GetChartHistory` → `Window` 계산(§6.1), 9심볼 병렬 |
| `store.go` | 적립(JSONL append) + 1–2h 전 레코드 로드(§5) |
| `collect.go` | `Collect(ctx, Deps, Options)` 오케스트레이션 + 수급 델타 결합 |
| `analyze.go` | 규칙기반 "시장반영" 코멘트 생성(§6.3) |
| `render.go` | 한국어 리포트 렌더 |
| `pulse_suite_test.go` + `*_test.go` | 단위 테스트(§9) |

`util.go`/`numOf` 등은 `snapshot` 패키지에 unexported 동명 함수가 있으나 **공유하지 말고 pulse 내부에 별도 정의**(패키지 결합 회피, snapshot 헬퍼는 비공개).

---

## 5. 영속화(적립) 포맷 — 수급 델타의 기반

파일: `{store-dir}/pulse_YYYYMMDD.jsonl` (KST 날짜, 실행마다 1줄 append).

레코드(델타 계산에 필요한 최소항목만):
```jsonc
{
  "ts": "2026-06-23T12:38:11+09:00",
  "kospi_idx": 8565.06, "kosdaq_idx": 907.47,
  "kospi_flow":  { "foreign": -35560, "institution": -15405, "individual": 51033, ... },  // 억
  "kosdaq_flow": { "foreign": -18, "institution": 1142, "individual": -1140, ... },
  "usdkrw": 1534.43
}
```

로드 규칙: 당일 파일의 모든 레코드를 읽어, 각 윈도(1h, 2h)에 대해 **타깃시각(now−Δ)에 가장 가까우면서 그 이전(at-or-before)** 레코드를 고른다. 없으면(첫 실행/장초반) 해당 윈도 델타는 `nil`.

`--no-save`면 append/MD 저장 생략(읽기 전용). 저장 시 최신 MD도 `pulse_YYYYMMDD.md`로 덮어쓰기(takesnapshot과 동일 패턴).

---

## 6. 알고리즘

### 6.1 Yahoo 윈도우 (`market.go`)

입력: `series []DailyClose`(오름차순), `quote`(현재가/전일대비%). 출력: `Window`.

```
앵커 lastTS = series[last].DateUnix, current = quote.Price (없으면 series[last].Close)
changePct = quote.ChangePercent          // 전일 종가 대비
for Δ in {1h, 2h}:
    target = lastTS - Δ
    ref = series에서 ts ≤ target 중 ts 최대인 점 (at-or-before)
          없으면(룩백이 시리즈 시작보다 과거) → 첫 점 사용 + "부분 데이터" 표시
    movePct = (current - ref.Close) / ref.Close * 100
```
- **희소 시리즈(`KRW=X`)**: 위 규칙으로 자연 처리(가장 가까운 과거점). 단 target과 ref의 간격이 과도(예: >45분)하면 `Reason`에 근사 표기.
- 심볼 9개는 errgroup/뮤텍스로 병렬 호출(`GetQuotes`의 기존 패턴 재사용).

### 6.2 수급 델타 (`collect.go`)

```
cur = 이번 실행 FlowSnapshot (KOSPI/KOSDAQ 각각)
for Δ in {1h, 2h}:
    prev = store.loadNearest(now-Δ)          // §5
    if prev == nil: FlowDeltaΔ = nil; continue
    FlowDeltaΔ = {
       RefTS: prev.ts, Elapsed: now-prev.ts,
       Foreign:     cur.Foreign     - prev.flow.Foreign,
       Institution: cur.Institution - prev.flow.Institution,
       Individual:  cur.Individual  - prev.flow.Individual,
       IndexDelta:  cur.idx - prev.idx,        // 교차검증용
    }
```
"가속/둔화" 판정: 1h 순매수율(델타1h) vs 직전 1h율((델타2h − 델타1h)) 비교.
- |델타1h| > |델타2h − 델타1h| → **가속(accelerating)**, 반대면 **둔화(decelerating)**.

### 6.3 시장반영 분석 규칙 (`analyze.go`, 결정적/설명가능)

입력 요약치로 한국어 불릿 4–6개 생성. 예시 규칙(임계값은 튜닝 대상):

1. **지수 모멘텀**: KOSPI `Move1h`
   - `≤ -0.5%` → "최근 1시간 코스피 {x} 하락, 하방 모멘텀 우위"
   - `≥ +0.5%` → "반등 시도"; else "횡보".
2. **수급 주도주체**: KOSPI `FlowDelta1h.Foreign`
   - `≤ -1000억` → "외국인 최근 1h {y}억 순매도 ({가속|둔화})"; 기관/개인도 동일 패턴.
3. **외국인 × 환율 연계**: 외국인 순매도 ∧ `USDKRW.Move1h > 0`(원화 약세) → "원화 약세 동반 외국인 이탈 → 환차손 회피성 매도 가능성".
4. **미국선물 동조/디커플링**: `sign(NQ.Move1h) == sign(KOSPI.Move1h)` → "미선물과 동조"; 부호 상반 → "디커플링: 미선물 {a} vs 코스피 {b} (갭 메우기 여지)".
5. **금리/유가 보조**: `^TNX` 급등 → "금리 상승이 위험자산 부담"; `CL=F` 급변 → 에너지/인플레 코멘트.
6. **종합 리스크 라벨**: 위 신호 부호 합산 → `위험회피(Risk-off) / 중립 / 위험선호(Risk-on)` 한 줄.

각 불릿은 산출 근거(수치)를 함께 표기해 "왜"가 드러나게 한다.

---

## 7. 데이터 모델 (구조체 스케치)

```go
type FlowSnapshot struct { Foreign, Institution, Individual,
    FinInvest, InvTrust, Pension, PrivEquity, Insurance, Bank, EtcCorp float64; OK bool } // 억

type IndexLevel struct { Price, PrevClose, ChangePct, Open, High, Low,
    TradingValue float64; Advancers, Decliners, Unchanged int; OK bool } // TradingValue 억

type Window struct { Symbol, Label string; Current, ChangePct float64;
    LastTS time.Time; Move1hPct, Move2hPct *float64; OK bool; Reason string }

type FlowDelta struct { RefTS time.Time; Elapsed float64 /*분*/;
    Foreign, Institution, Individual, IndexDelta float64 }

type Market struct { Name string; Index IndexLevel; IntradayWindow Window;
    Flow FlowSnapshot; FlowDelta1h, FlowDelta2h *FlowDelta }

type Pulse struct { Now time.Time; Date string;
    KOSPI, KOSDAQ Market; USDKRW Window; Macro []Window;
    StoredCount int; Analysis []string; Errors map[string]string }
```
`Deps{ Stock DomesticStock; Yahoo YahooQuotes; Clock func() time.Time; StoreDir string }` — 인터페이스는 **필요 메서드만**(테스트 페이크 용이): `InquireInvestorTimeByMarket`, `InquireIndexPrice` / `GetQuotes`, `GetChartHistory`.

---

## 8. 렌더링 레이아웃 (한국어, 예시)

```
🫀 시장 펄스  2026-06-23 13:38 KST   (당일 적립 12회 · 직전 12:38)

📈 지수 (전일대비 · 최근1h/2h)
  KOSPI   8,565.06  ▼-6.03%   1h ▼-1.8%  2h ▼-3.1%   거래대금 36.6조
          시 9,083 / 고 9,175 / 저 8,511   상승 87 · 하락 823
  KOSDAQ    907.47  ▼-6.29%   1h ▼-2.0%  2h ▼-3.4%   상승 148 · 하락 1,561

💰 수급 — 최근 1h 델타 (괄호=당일 누적)
  KOSPI   외국인 ▼-1.20조 가속 (누적 -3.56조) · 기관 ▼-0.40조 (-1.54조) · 개인 ▲+1.55조 (+5.10조) · 기타법인 ▼-67.7억 (합계 0억)
  └ 기관 세부(누적): 금융투자 -9,967 · 보험 -596 · 투신 -1,286 · 기타금융 -39 · 은행 -146 · 연기금 -1,290 · 사모 -2,081 (합계 -1.54조)
  KOSDAQ  외국인 ▼-300억 (-18억) · 기관 ▲+150억 (+1,142억) · 개인 ▼-... · 기타법인 ... (합계 0억)
  └ 기관 세부(누적): 금융투자 ... · 보험 ... · 투신 ... · 기타금융 ... · 은행 ... · 연기금 ... · 사모 ... (합계 +1,142억)

💱 환율   원/달러 1,534.43  1h ▲+0.3원  2h ▼-1.7원   (원화 강세 소폭)

🌐 미국선물·매크로 (최근1h/2h)
  나스닥선물 30,557  1h ▲+0.7%  2h ▲+1.2%   S&P선물 ▲...   다우선물 ▲...
  닛케이 ...   WTI ...   미10Y ...

🧭 시장반영
  • 외국인 최근 1h -1.2조 순매도 '가속' — 당일 -3.56조의 1/3이 직전 1시간에 집중.
  • 미선물 1h +0.7% 반등에도 코스피 1h -1.8% → 디커플링(국내 수급 이탈 우위).
  • 원/달러 강보합 → 환율發 압력은 제한적.
  • 종합: 위험회피(Risk-off), 외국인 매도 가속이 주도.

💾 .cache/pulse/pulse_20260623.jsonl (+1행) · pulse_20260623.md 갱신
```

첫 실행(베이스라인 없음): 수급 델타 자리에 "기준선 적립 — 잠시 후 재실행 시 구간 델타 표시" 표기. 지수/환율/미국선물 구간은 즉시 표시.

---

## 9. 엣지케이스

- **첫 실행/장초반**: 1h 또는 2h 전 레코드 없음 → 해당 델타 `nil`, 안내 문구.
- **장마감 후/주말**: KIS 지수·수급은 종가 누적, Yahoo 분봉은 직전 세션. `Now` 기준 안내. (선물은 거의 24h라 정상 동작.)
- **`KRW=X` 희소/결측**: 인접 과거점 사용 + 간격 과다 시 근사 표기. `GetQuotes`의 `Price`로 현재가 보강.
- **Yahoo 부분 실패**: 심볼별 독립 처리(하나 실패해도 나머지 진행), `Errors`에 기록.
- **선물 날짜 경계**: §3.C 경고 — 절대 unix 기준 윈도우.
- **단위 혼동 금지**: 모든 금액은 내부적으로 **억원**으로 정규화(백만원 ÷100). `acml_tr_pbmn`도 동일.
- **음수 콤마 문자열**: `numOf`에서 콤마 제거 후 `ParseFloat`.

---

## 10. 테스트 플랜 (Ginkgo, 기존 `*_suite_test.go` 패턴)

- `flow_test.go`: 모킹된 `output` 1행 → 억 변환/부호/주체매핑 검증.
- `index_test.go`: 단일 객체 파싱, 전일종가=현재−prdy_vrss, 거래대금 억 변환.
- `market_test.go`: 합성 5분봉으로 윈도우(1h/2h) 경계·희소·날짜경계, at-or-before 선택 검증.
- `store_test.go`: JSONL append/load, `loadNearest(now-Δ)` 선택 규칙, 첫 실행 nil.
- `analyze_test.go`: 입력 조합별(외인매도+원화약세 등) 코멘트/리스크 라벨 결정성.
- 모킹: `Deps`에 페이크 `DomesticStock`/`YahooQuotes` 주입, `Clock` 고정.
- `testhelpers.MockTransport`(기존) 재사용 가능.

---

## 11. 작업 체크리스트

1. `internal/collector/pulse/` 패키지 생성(§4 파일들).
2. `types.go`/`util.go` → `flow.go`/`index.go`/`market.go` → `store.go` → `collect.go` → `analyze.go` → `render.go` 순 구현.
3. `cmd/agent/main.go`에 `intraday-pulse` 케이스 + `runIntradayPulse`(클라이언트 구성 재사용).
4. `Makefile`에 `now:` 타깃 + `.PHONY`.
5. 단위 테스트(§10) + `go build ./... && go test ./...`.
6. 장중 라이브 실행으로 렌더/적립 확인(2회 이상 실행해 델타 동작 검증).

> 단위/엔드포인트/필드는 본 문서 §3의 **라이브 검증치**를 신뢰원으로 사용할 것. Yahoo는 비공식 API(스냅샷 도구와 동일 리스크) — 장기적으로 공식 제공처 대체 검토.
