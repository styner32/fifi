# 주요 기업 공시 수집기 (Filings Collector)

`make market-collector`와 같은 바이너리(`cmd/market-collector`)의 `filings` 서브커맨드로, 지정한 주요 기업의 최근 DART 공시를 가져와 기존 공시 분석 파이프라인(`internal/dartfiling/tasks`)으로 저장·분석합니다.

## 실행

```bash
make market-collector-filings-once ARGS="--dry-run"   # 목록만 조회 (DB·OpenAI 사용 안 함)
make market-collector-filings-once                    # 1회 수집 + 분석 + 저장
make market-collector-filings                         # 데몬 (기본 1시간 주기)
make market-collector-filings ARGS="--interval 30m --limit 10"
```

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--targets` | `$FILING_TARGETS_FILE` → `docs/filing_targets.txt` | 수집 대상 문서 |
| `--limit` | `5` | 기업별 최근 공시 최대 건수 (실행 1회당) |
| `--days` | `5` | 조회 기간 (일) |
| `--dry-run` | `false` | DART 목록만 출력 |
| `--interval` | `1h` (`$FILINGS_COLLECTOR_INTERVAL`) | 데몬 주기 |

`once` 모드는 실패한 기업이 하나라도 있으면 0이 아닌 종료 코드를 반환합니다. `run` 모드는 실패를 로그로 남기고 다음 주기에 다시 시도합니다.

## 대상 기업 추가

[`docs/filing_targets.txt`](filing_targets.txt)에 한 줄씩 추가합니다.

```text
012450  # 한화에어로스페이스
123456 00123456  # corp_code를 직접 지정 (종목코드로 매핑되지 않는 경우)
```

- 종목코드는 DART `corpCode.xml`의 `stock_code`로 `corp_code`에 자동 매핑되며, 결과는 `.cache/dart_corp_code.<YYYYMMDD>.json`에 KST 날짜별로 캐시됩니다 (기본 경로는 `DART_CORP_CODE_CACHE_FILE`로 변경).
- 형식이 잘못된 줄이 있으면 줄 번호와 함께 실행을 중단합니다. 매핑되지 않는 종목코드는 경고를 남기고 건너뜁니다.
- 데몬은 매 주기마다 문서를 다시 읽으므로 재시작하지 않아도 변경 사항이 반영됩니다.

## 처리 흐름과 저장

기업별로 `TaskProcessor.FetchAndAnalyzeReports`를 호출합니다. 이 경로는 `make dart-filing-cli-company`와 같습니다.

1. DART `list.json`으로 해당 `corp_code`의 최근 공시를 조회
2. 이미 `raw_reports.receipt_number`에 있는 공시는 건너뜀 (중복 저장 방지)
3. 원문 다운로드 → XBRL/HTML 파싱 → `raw_reports`에 저장
4. OpenAI 분석 (128KB를 넘으면 Batch API 사용) → `analyses`에 저장

## OpenAI 모델

- 기본값: `gpt-6-luna` (`openai.DefaultModel`)
- `OPENAI_MODEL` 환경변수로 덮어쓸 수 있습니다. dart-filing worker/CLI/API에도 같은 규칙이 적용됩니다.

## 필요한 환경 변수

| 변수 | dry-run | 일반 |
|---|---|---|
| `DART_API_KEY` | 필요 | 필요 |
| `OPENAI_API_KEY` | - | 필요 |
| `DATABASE_URL` | - | 필요 (기본값은 `internal/dartfiling/config`) |

## 주의

- 공시 1건마다 OpenAI를 한 번 호출합니다. 10개 기업 × `--limit 5`이면 실행 1회에 최대 50건입니다. 이미 저장된 공시는 건너뛰므로, 두 번째 실행부터는 새로 올라온 공시만 분석합니다.
- 기존 파이프라인과 동일하게, 분석이 실패해도 `raw_reports`는 이미 저장된 상태로 남습니다. 따라서 다음 실행에서는 그 공시를 다시 분석하지 않습니다.
- Docker 이미지(`Dockerfile_market_collector`)에는 `docs/`가 포함되지 않습니다. 컨테이너에서 실행하려면 대상 문서를 마운트하고 `FILING_TARGETS_FILE`로 경로를 지정하세요.

## 스냅샷 출력 (`make takesnapshot` / `make filings-snapshot`)

`make takesnapshot` 실행 시 시장 스냅샷(`market_snapshot.<YYYYMMDD>.{md,json}`) 생성 직후 당일(또는 `--date` 기준일) 주요 기업 공시를 자동으로 동기화·요약하여 함께 출력하고 별도 파일로 저장합니다.

```bash
make takesnapshot                                    # 시장 스냅샷 + 당일 공시 스냅샷 함께 실행
make filings-snapshot                                # 당일 공시 스냅샷만 단독 실행 (DART 동기화 포함)
make filings-snapshot DATE=20261014 ARGS="--no-sync" # 이미 DB에 저장된 공시만으로 스냅샷 생성
```

생성 파일 (기본 경로 `.cache/snapshots/`):
- `.cache/snapshots/market_filings.<YYYYMMDD>.md` — 당일 공시 요약 테이블 및 기업별 상세 분석 마크다운
- `.cache/snapshots/market_filings.<YYYYMMDD>.json` — 구조화된 JSON 스냅샷 (`summary_ko`, `key_terms`, `key_dates` 등 포함)

`MARKET_SNAPSHOT_SKIP_FILINGS_SYNC=true`로 설정하면 `make takesnapshot` 실행 시 DART/OpenAI 실시간 동기화를 건너뛰고 DB(`raw_reports`, `analyses`)에 이미 저장된 당일 공시만 조회해 파일로 정리합니다.


`MARKET_SNAPSHOT_SKIP_FILINGS=true`는 공시 후속 작업 전체를 생략합니다. 시장 MCP는 이 후속 작업을 호출하지 않습니다. 워커 재시작은 기존 큐를 보존하며, 시작 시 새 수집 배치를 넣으려면 `DART_WORKER_BOOTSTRAP_FETCH_REPORTS=true`를 명시합니다.
