# Agent CLI

`go run ./cmd/agent report <명령> [플래그]`로 실행합니다. 데이터 의미는 각 데이터 계약이 기준입니다. 성공적으로 출력됐다는 사실은 시각·단위·거래소 확정·모델 정확성이 검증됐다는 뜻이 아닙니다.

| 명령 / Make 타깃 | 기능 | 출력·저장·부작용 |
|---|---|---|
| `premarket` / `make premarket` | 장전 관측·공식 일정·결측 상태 | `--json`, `--no-save`, `--store-dir`; 기본 `.cache/premarket` |
| `intraday-pulse` / `make now` | 현재 관측과 당일 이력 비교 | `--json`, `--no-save`, `--no-history` (`--no-prev` 별칭), `--store-dir`; 기본 `.cache/pulse` |
| `market-snapshot` / `make takesnapshot` | 요청일 일별 데이터와 조회 시점 문맥 | `--date YYYYMMDD`; 기본 `.cache/snapshots`; CLI는 이어서 공시 동기화를 시도 |
| `filings-snapshot` / `make filings-snapshot` | 주요 기업 공시 | DART·OpenAI·DB 작업 가능; `--no-sync`는 이미 저장된 공시 조회 |
| `rsi` | Yahoo 가격의 RSI | `--symbol`, `--period`, `--days`, `--method`, `--interval`, `--json`; 보고서 저장 없음 |
| `credit-balance --ratio` | 동일 기준일의 신용융자/예탁금 비율 | `--days` (1~250), `--json`; 보고서 저장 없음 |
| `credit-balance <종목...>` / `make creditbalance` | 종목별 신용잔고 | KIS 인증 필요; 기본 `.cache/credit_balance`에 저장 |
| `safety-devices` | 펄스의 안전장치 관측 | 펄스와 같은 수집·저장; 공식 발동 기록은 별도 입력 |

`--no-save`는 보고서·이력 저장을 생략하는 옵션입니다. 인증 토큰·KOFIA·휴장일·종목 마스터 등의 원천 캐시 갱신과 구분합니다. JSON 결측은 `null`, 실제 응답의 0은 0입니다.

## 시장 보고서

```bash
make premarket
make now
make now-debug
make takesnapshot DATE=20261016
go run ./cmd/agent report premarket --json --no-save
go run ./cmd/agent report intraday-pulse --json --no-save
```

- [장전 계약](PREMARKET_DATA_CONTRACT.md): 검증되지 않은 D/A/S 점수·예측 신뢰도는 `null`, 종합은 `COMPOSITE_NOT_CIRCULABLE`입니다. 신용·VKOSPI 백분위는 각각 60개·250개 유효 과거 원자료 기준일이 필요합니다.
- [장중 계약](NOW_DATA_CONTRACT.md): 실제 관측 시각과 적절한 기준점이 없으면 시간 구간 변화를 계산하지 않습니다. 첫 수집은 시초가가 아닙니다.
- [Snapshot 계약](SNAPSHOT_DATA_CONTRACT.md): 과거 요청에 현재 전용 시세를 섞지 않고, 장후에도 `POST_CLOSE_UNRECONCILED`를 보존합니다. 날짜는 KST이며 명시한 휴장일을 임의로 전일로 바꾸지 않습니다.
- 공시 작업 전체 생략: `MARKET_SNAPSHOT_SKIP_FILINGS=true`. 실시간 DART/OpenAI 동기화만 생략: `MARKET_SNAPSHOT_SKIP_FILINGS_SYNC=true`. 자세한 내용은 [공시 수집기](FILING_COLLECTOR.md).

## RSI

```bash
go run ./cmd/agent report rsi --symbol kospi --period 14 --days 30
go run ./cmd/agent report rsi --symbol kosdaq --interval 60m --method sma
go run ./cmd/agent report rsi --symbol 000660 --json
```

기본 심볼은 `^KS11`, 기간은 14, 출력 관측 수는 30, 주기는 `1d`, 방식은 `wilder`입니다. `sma`/`cutler`도 지원합니다. `kospi`, `kosdaq`, `samsung`/`005930`, `skhynix`/`000660` 별칭을 지원합니다. Yahoo 조회에는 KIS 인증이 필요하지 않습니다. 조회 오류·표본 부족·잘못된 관측은 오류이며 합성 가격을 생성하지 않습니다. 당일 봉은 완결 종가가 아닐 수 있습니다.

## 신용 비율

```bash
go run ./cmd/agent report credit-balance --ratio --days 30 --json
go run ./cmd/agent report credit-balance 005930 000660
```

비율 = 동일 기준일 신용융자 잔고 / 양수인 투자자예탁금 × 100. 신용은 KIS `mktfunds.crdt_loan_rmnd`, 예탁금은 KIS 또는 같은 날짜의 KOFIA 값입니다. KIS 억원은 `/10000`, KOFIA `KRW_MILLION`은 `/1000000`으로 조원에 변환합니다. 크기로 단위를 추정하지 않습니다.

신용·예탁금·증감·비율은 결측 시 JSON `null`입니다. 날짜 없는 값, 다른 날짜, 알 수 없는 단위, 충돌하는 원자료로 비율을 만들지 않습니다. 공표 지연은 확인 전 미공표/T+1로 단정하지 않습니다. 위험 진단은 `NOT_EVALUATED`이며 과거 고정 앵커·가격 대체·라벨 분포 QC는 사용하지 않습니다. 이 nullable 출력은 예전 수치 0 기반 출력과 다릅니다.

## 안전장치

```bash
go run ./cmd/agent report safety-devices --no-save
```

임계 가격은 당일 전일종가에 비율을 적용합니다. 고정된 과거 지수 예시를 현재 발동선으로 사용하지 않습니다. `OFFICIAL_EVENTS_FILE`의 날짜·출처·발동/해제 시각을 검증하며 공식 자동 연동 여부는 [장중 계약](NOW_DATA_CONTRACT.md)을 따릅니다.

## MCP·수집기·검증

[시장 MCP](MARKET_MCP.md)는 세 시장 수집기를 로컬 LLM 도구로 제공합니다. 공시 MCP와 별도이며 시장 MCP는 공시 동기화·OpenAI 분석을 실행하지 않습니다. 백그라운드 시장 facts 수집과 pulse 파일 적립은 서로 다른 경로입니다.

신규·수정 Go 테스트는 기존 Ginkgo v2/Gomega suite를 사용합니다. [테스트 가이드](TESTING.md)와 [유지보수·이전](MAINTENANCE.md)을 따르세요.
