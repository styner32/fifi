# 로컬 통합 fifi MCP

`fifi` 서버 하나에서 시장 도구 3개와 공시 조회 도구 3개를 사용합니다. 시장 도구는 CLI와 공통 수집·렌더링 서비스를 사용하며, 장중 펄스는 별도 현재 관측 모드로 실행합니다. 공시 도구는 DART REST API의 저장된 공시를 조회합니다. LLM은 반환된 출처·시각·단위·결측·품질 상태를 근거로 해석합니다. MCP는 OpenAI 분석이나 공시 수집 배치를 자동 실행하지 않습니다.

| 도구 | 입력 | 결과 |
|---|---|---|
| `market_premarket_report` | `{}` | 장전 데이터·공식 일정·미검증 입력 |
| `market_intraday_pulse` | `{}` (`use_history`는 호환용으로만 허용) | 호출 시점의 현재 관측·전일 대비·안전장치; 관측 간 변화 제외 |
| `market_daily_snapshot` | `{}` 또는 `{"date": "20261016"}` | 지정일 snapshot; 날짜 생략 시 KST 오늘 |
| `disclosures_reports_by_corp_name` | `corp_name`, 선택 `limit` | 기업명으로 최근 공시 조회 |
| `reports_summary_all_companies` | 선택 `corp_code`, `start_date`, `end_date`, `limit` | 저장된 공시 요약 목록 |
| `reports_summary_and_raw_by_receipt_number` | `receipt_number` | 공시 한 건의 요약과 원문 |

시장 snapshot 날짜는 `YYYYMMDD` / `YYYY-MM-DD`를 받으며 실제 날짜·미래 여부를 검증합니다. 명시한 휴장일을 다른 날짜로 이동하지 않습니다. 과거 보고서는 현재 전용 시세를 제외합니다. 공시 목록의 날짜 필터는 `YYYY-MM-DD`이며 시작일 포함·종료일 미포함입니다. 기존 여섯 도구의 이름과 입출력 계약을 유지합니다.

## 빌드와 연결

```bash
make setup
make mcp-build
```

`mcp-build`는 통합 서버 `bin/fifi-mcp`와 기존 바이너리 경로의 호환 진입점을 빌드합니다. 터미널에서 직접 시작할 때는 `make mcp`를 사용합니다. 실행 target은 Make의 명령 출력도 억제하여 stdout을 JSON-RPC 전용으로 유지합니다.

Codex 로컬 stdio 설정은 아래 항목 하나를 사용합니다. 기존 시장·공시 서버를 각각 등록했다면 이전 두 항목을 이 `fifi` 항목 하나로 교체합니다. 프로젝트 위치가 다르면 command와 cwd의 절대 경로를 함께 바꿉니다. 저장소가 전역 Codex 설정을 자동 변경하지는 않습니다.

```toml
[mcp_servers.fifi]
command = "/Users/sunjinlee/workspace/fifi/bin/fifi-mcp"
cwd = "/Users/sunjinlee/workspace/fifi"
startup_timeout_sec = 20
tool_timeout_sec = 150
```

시작 디렉터리의 `.env`와 프로세스 환경변수를 사용합니다. Make를 통한 실행은 명시한 환경변수·Make 할당 → `.env` → 로컬 기본값 순서입니다. 바이너리를 직접 연결할 때도 프로세스 환경변수가 `.env`보다 우선합니다. `APP_KEY`, `APP_SECRET`이 현재 시장 KIS 수집에 필요합니다. 장전 수집은 KIS 미설정·인증 실패 시 공개 데이터로 부분 결과를 제공할 수 있습니다. 키를 도구 입력이나 문서에 넣지 않습니다. `DATABASE_URL`은 검증된 facts fallback 조회용이며 DB 불통은 경고와 함께 해당 fallback을 비활성화합니다.

연결 방식은 [공식 Codex MCP 문서](https://learn.chatgpt.com/docs/extend/mcp?surface=cli), 프로토콜은 [공식 Go SDK](https://github.com/modelcontextprotocol/go-sdk)를 참고하세요.

기존 `make market-mcp`, `make dart-filing-mcp`와 `cmd/market-mcp`, `cmd/dart-filing-mcp` 실행 경로도 동일한 여섯 도구를 제공하는 통합 서버를 시작합니다. 각 `-build` 호환 명령은 기존 바이너리 경로를 빌드합니다. 일상 실행과 연결에는 `make mcp`, `bin/fifi-mcp`를 사용하면 됩니다.

## 시장 출력과 저장

시장 도구 세 개의 공통 출력은 `{schema_version: 1, report_type, report, artifacts: [{kind, path}], warnings}`입니다. `report`는 공통 JSON serializer를 사용하며 MCP `structuredContent`와 JSON 텍스트 양쪽으로 반환합니다. 장중 펄스는 아래의 현재 관측 모드를 적용합니다. 데이터 계약의 schema 2, `null`, `NOT_EVALUATED`, `RAW_SPREAD`, `POST_CLOSE_UNRECONCILED`, `COMPOSITE_NOT_CIRCULABLE`을 보존합니다.

- MCP 기본 저장 위치는 `.cache/premarket`, `.cache/mcp/pulse`, `.cache/snapshots`입니다. 각각 `PREMARKET_STORE_DIR`, `MCP_PULSE_OUTPUT_DIR`, `SNAPSHOT_OUTPUT_DIR`로 서버 환경에서 변경합니다. `PULSE_OUTPUT_DIR`는 CLI 펄스 이력 위치이며 MCP 펄스 저장 경로를 바꾸지 않습니다.
- 장전은 `store.v2.json`과 `premarket.YYYYMMDD.{json,md}`, MCP 펄스는 `pulse_current.YYYYMMDDTHHMMSS.NNNNNNNNN.{json,md}`, snapshot은 `market_snapshot.YYYYMMDD.{json,md}`를 저장합니다. MCP 펄스 파일명은 KST 수집 시작 시각을 사용합니다. 교체하는 장전·snapshot 보고서는 이전 파일을 보존합니다. MCP 펄스는 CLI의 `pulse_YYYYMMDD.jsonl`을 읽거나 추가하지 않습니다.
- 저장 실패는 `warnings`에 남깁니다. 일부 지표 실패는 유효 부분 결과와 상태를 반환합니다. 수집 자체가 불가능하면 `isError=true`와 진단을 반환합니다.
- 인증·마스터·KOFIA·휴장일 캐시는 갱신할 수 있습니다. MCP는 공시 동기화·OpenAI 분석·주문·facts DB 관측 쓰기를 호출하지 않습니다. CLI `takesnapshot`의 공시 후속 작업과 구분합니다.
- 시장 도구는 저장 부작용이 있으므로 읽기 전용으로 광고하지 않습니다. 공시 도구는 읽기 조회입니다. stdout은 JSON-RPC 전용이며 진단과 DB 로그는 stderr입니다.

수집은 최대 120초이며 파일 잠금 대기도 이 제한에 포함됩니다. CLI/MCP 공통 잠금은 `MARKET_REPORT_LOCK_FILE` (기본 `.cache/market-report.lock`)입니다. 같은 데이터를 공유하는 프로세스들은 같은 절대 잠금 경로를 사용해야 합니다.

`market_intraday_pulse`는 매 호출에서 현재 데이터를 새로 조회합니다. `make now`의 로컬 관측 이력을 읽거나 적립하지 않으며, 직전·당일 첫 수집·1h/2h 대비 변화와 공급자 분봉으로 계산하는 장중 구간 변화도 제공하지 않습니다. 현재값, 전일 대비 등락, 출처·관측 시각·조회 시각·결측 상태와 현재 안전장치 근거는 유지합니다. 보고서 작성 시각이 현재라도 각 데이터의 실제 관측 시각이 현재와 같다는 뜻은 아닙니다.

응답의 `report.comparison_mode`는 `CURRENT_ONLY`입니다. 출력 호환성을 위해 기존 비교 필드는 제거하지 않고 변화값·구간 기준 시각을 `null`로 유지합니다. `stored_count`는 0이며 현재 관측값과 출처 메타데이터는 유지합니다.

기존 클라이언트를 위해 `use_history` 입력은 생략·`null`·`true`·`false`를 계속 허용하지만 폐기 예정 입력이며 모든 값이 무시됩니다. `{}`로 호출하면 됩니다. `make now`는 기존 이력 사용·저장 및 유효한 기준점이 있는 구간 비교를 계속 수행합니다. 장전·snapshot 도구의 이력 동작은 변경하지 않습니다.

## 공시 API 연결

공시 도구도 같은 `fifi` 서버에 등록됩니다. API 주소는 `DART_FILING_BASE_URL` → 기존 `KOSIS_BASE_URL` → `http://localhost:8080/api/v1` 순서입니다. 로컬 DB를 준비하고 API를 별도 터미널에서 시작합니다.

```bash
make services-up
make migrate-up
make dart-filing-api
```

MCP는 API 서버를 자동 시작하지 않습니다. API 연결이 끊기면 해당 공시 호출에만 오류를 반환하며 MCP 초기화, 여섯 도구 목록 조회와 시장 도구 호출은 계속 동작합니다. 공시 조회 결과는 기존 API JSON 객체를 `structuredContent`와 JSON 텍스트로 반환합니다. 공시 조회를 위해 워커나 새 OpenAI 분석 배치를 실행할 필요는 없습니다.

검증 방법은 [로컬 개발 가이드](MAINTENANCE.md)와 [테스트 가이드](TESTING.md)를 참고하세요. 도구 호출 성공은 데이터 정합성·공식 확정·LLM 예측 정확성을 보장하지 않습니다.

## 스키마 호환성 확인

```bash
make mcp-build
npx @modelcontextprotocol/inspector@2.11.0 --cli ./bin/fifi-mcp --cwd "$PWD" --method tools/list --strict --format json --stored-auth-only
```

Inspector 2.11.0에서 여섯 도구의 스키마 오류·경고가 모두 0건임을 확인했습니다. JSON 출력에 `schemaFindings`가 없으면 스키마 지적 사항이 없습니다. 경고만 있으면 종료 코드가 0일 수 있으므로 종료 코드만으로 판단하지 않습니다. 이 검사는 초기화와 도구 목록을 확인하며 실제 데이터 수집·저장 성공은 별도 도구 호출로 확인합니다.

시장 출력의 nullable 배열과 펄스 입력의 nullable boolean은 `anyOf`로 표현합니다. 펄스 `use_history`의 허용 값은 유지하되 현재 관측 모드에서는 무시합니다. 공시 출력은 `additionalProperties: true`로 자유 형식 JSON 객체 계약을 명시합니다. 기존의 빈 스키마 `{}`와 허용하는 값이 같으며 배열·숫자·문자열·객체·boolean·`null`을 임의로 제한하지 않습니다. 바이너리를 다시 빌드한 후 연결된 클라이언트의 MCP 서버를 재시작해야 새 스키마를 읽습니다.
