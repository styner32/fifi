# 로컬 개발·실행·검증 가이드

이 저장소는 로컬에서 시장 보고서, DART API·워커·웹과 MCP를 실행합니다. 저장소 루트의 `make help`가 명령 목록이며, 인자 없는 `make`도 같은 도움말을 표시합니다. 이 문서의 경로는 기존 링크와의 호환성을 위해 유지합니다.

## 최초 준비와 설정

Go 1.27.2, Node 24.22.0 LTS, npm 12.2.0, Ruby와 Docker Compose를 준비합니다. 버전 기준은 `.go-version`, `.nvmrc`, `.node-version`과 `web/package.json`입니다. 시스템 전역 도구 선택은 사용 중인 버전 관리자로 합니다.

```bash
make setup
make services-up
make migrate-up
make test-db-prepare
make check
```

- `setup`: Go 모듈 다운로드, `npm ci`, 저장소에 고정된 서브모듈 초기화.
- `services-up`: 로컬 DB·Redis를 준비하고 정상 응답을 기다림. 최초 Redis 준비는 기존 데이터를 별도 Redis 8 볼륨으로 복제하며 원본을 보존함.
- `migrate-up`: 앱 DB에 누락된 migration 적용. 고정된 `golang-migrate` v4.20.1을 Go로 실행하므로 별도 `migrate` 바이너리 설치는 필요 없음.
- `test-db-prepare`: 로컬 Compose PostgreSQL에 `_test` DB를 생성하고 migration 적용.
- `check`: 격리된 DB를 사용하는 전체 Go 테스트, 문서 링크, 웹 build·lint.

`.env`에는 API 키와 로컬 연결 설정을 둡니다. Make로 실행하는 앱·마이그레이션·테스트의 환경설정 우선순위는 **명시한 프로세스 환경변수·Make 할당 → `.env` → 로컬 기본값**입니다. 예를 들어 `make now DATABASE_URL=...`의 값이 `.env`보다 우선합니다. `DATE`, `ARGS`, `NPM` 등 실행 옵션은 Make 변수로 지정합니다. 실제 비밀값은 명령 예시나 문서에 저장하지 않습니다.

테스트는 앱 설정과 분리된 `TEST_DATABASE_URL`, `TEST_REDIS_URL`을 사용합니다. 테스트 DB 이름은 `_test`로 끝나야 합니다. 다른 로컬 테스트 인스턴스를 사용할 때는 두 변수를 명시하고 해당 DB에 migration을 적용합니다. 테스트의 명시 환경은 `.env.test`보다 우선합니다.

## 일상 명령

| 용도 | 명령 |
|---|---|
| 로컬 DB·Redis 시작·중지 | `make services-up`, `make services-down` |
| 서비스 상태·로그 | `make services-status`, `make services-logs` |
| DB migration·현재 버전 | `make migrate-up`, `make migrate-status` |
| migration 파일 추가 | `make migrate-create NAME=설명` |
| 마지막 migration 되돌리기 | `make migrate-down` |
| Go 전체 테스트 | `make test` 또는 `make test-integration` |
| Go 전체 race 검사 | `make test-race` |
| Go·문서·웹 종합 검사 | `make check` |
| Go·npm 취약점 검사 | `make audit` |
| 웹 의존성 설치·lint | `make web-install`, `make web-lint` |
| API·워커·웹 실행 | `make dart-filing-api`, `make dart-filing-worker`, `make dart-filing-web` |
| 웹 빌드 | `make dart-filing-web-build` |
| 통합 MCP 빌드 | `make mcp-build` → `bin/fifi-mcp` |
| 통합 MCP 서버 실행 | `make mcp` |

`services-up`은 API·워커를 자동 실행하거나 새 OpenAI 분석 배치를 투입하지 않습니다. 필요한 앱을 각각 별도 터미널에서 실행합니다. `services-down`은 컨테이너를 중지하고 DB·Redis 볼륨을 보존합니다. 보고서와 인증·마스터 캐시는 로컬 파일에 남습니다.

DB·Redis만 시작할 때 `.env.docker`는 필요하지 않습니다. 앱을 Docker 컨테이너로 실행할 때는 별도로 해당 파일에 컨테이너용 연결 설정을 둡니다.

워커는 기존 큐를 유지하며 재시작 시 공시 수집 배치를 자동 투입하지 않습니다. 새 배치가 필요하면 `DART_WORKER_BOOTSTRAP_FETCH_REPORTS=true make dart-filing-worker`로 명시합니다.

## 시장 보고서와 수집기

```bash
make premarket
make now
make now-debug
make takesnapshot DATE=20261016
make rsi ARGS="--symbol kospi --period 14 --interval 60m"
make leverage ARGS="--days 60 --json"
make safety-devices
```

`make creditbalance`의 기본 조회도 신용융자·예탁금 비율입니다. 종목별 조회는 `make creditbalance SYMBOLS="005930 000660"` 또는 `make creditbalance WATCHLIST=watchlist.txt`로 지정합니다. watchlist는 개인 로컬 파일입니다.

CLI 플래그는 `ARGS`로 전달할 수 있습니다. `make now ARGS="--json --no-save"`는 JSON을 출력하고 보고서 이력 저장을 생략합니다. 인증·원자료 캐시는 별도로 갱신할 수 있습니다. 임의의 Agent 하위 명령은 `make agent ARGS="report rsi --symbol kosdaq"`로 실행합니다.

```bash
make market-collector-once
make market-collector ARGS="--interval 10m"
make market-collector-backfill
make filings-snapshot DATE=20261016 ARGS="--no-sync"
make dart-filing-cli-reports
```

facts 수집기·백필은 실제 `DATABASE_URL` 연결을 확인합니다. 백필은 schema 2의 원본 날짜·시각을 검증하고 처리·수락 시도·제외·오류 수와 사유를 stderr에 남깁니다. 수락 시도 수는 중복으로 이미 존재하는 행도 포함할 수 있습니다.

`make takesnapshot`의 공시 후속 작업은 로컬 DB 저장과 OpenAI 분석을 실행할 수 있습니다. 시장 자료만 필요하면 `MARKET_SNAPSHOT_SKIP_FILINGS=true make takesnapshot`을 사용합니다. 통합 MCP는 시장 수집 도구 3개와 공시 API 조회 도구 3개를 제공합니다. 공시 API는 `make dart-filing-api`로 따로 실행하며, API 연결 실패는 해당 공시 호출에만 영향을 줍니다. 연결 방법은 [통합 MCP 가이드](MARKET_MCP.md)를 참고하세요.

MCP 연결은 `fifi` 서버 하나로 설정합니다. 기존 `make market-mcp`, `make dart-filing-mcp`와 해당 `-build` 명령은 같은 통합 서버를 위한 호환 경로입니다. 기존 두 연결 항목은 `fifi` 항목 하나로 교체합니다.

## 로컬 데이터와 검증 기준

PostgreSQL은 기존 로컬 DB 볼륨을 사용합니다. Redis 8용 볼륨을 처음 준비할 때 기존 Redis persistence 데이터를 복제하고, 이후 실행은 준비된 볼륨을 재사용합니다. 원본 DB·Redis 볼륨과 `.cache`의 보고서를 자동 삭제하지 않습니다.

실행 중인 기존 Redis의 persistence 모드는 자동 조회합니다. 중지된 원본에 RDB와 AOF가 함께 남아 있으면 마지막 설정에 맞춰 `REDIS_APPENDONLY=yes make redis-prepare` 또는 `REDIS_APPENDONLY=no make redis-prepare`를 한 번 지정합니다.

Facts migration 011은 기존 관측을 `provenance_version=0`으로 보존합니다. 검증된 출처·실제 관측 날짜·시각이 있는 버전 1만 정상 fallback에 사용합니다. 동일 시각의 원본과 교정본은 공존할 수 있으며, 이전 unique key와 충돌하는 down migration은 행을 삭제하는 대신 중단합니다.

Go 테스트는 Ginkgo v2/Gomega를 사용합니다. 전체 DB suite는 fixture 테이블을 공유하므로 `-p 1`로 순차 실행합니다. 고정 fixture 회귀와 실제 API·LLM 연결 검증을 구분합니다. 자세한 작성 규칙은 [테스트 가이드](TESTING.md)에 있습니다.

의존성은 manifest·lock을 기준으로 설치합니다. React 19.3.1, TypeScript 6.0.3, Vite 8.3.4, React plugin 6.1.2, ESLint 10.13.0, typescript-eslint 8.71.2가 현재 정비 기준입니다. TypeScript 7은 typescript-eslint의 지원 범위 밖이므로 강제 설치하지 않습니다. `make audit`는 govulncheck v1.9.0과 npm lock의 취약점을 확인합니다.

## 통합 MCP 검증 (2026-10-18 KST)

`internal/fifimcp`와 공통 보고서 서비스의 race 검사를 통과했습니다. 한 세션에서 도구 6개 호출, 도구별 입력·출력 계약과 실패 격리를 검증했습니다. 시장·공시 JSON의 큰 정수와 결측 보존도 회귀 검사에 포함됩니다.

`make mcp-build` 후 단일 바이너리의 초기화·도구 6개 조회·공시 HTTP mock 경로 3개·시장 도구 오류 후 공시 재호출을 stdio로 확인했습니다. 실제 wire의 structured/text JSON 모두 `9007199254740993`과 `null`을 보존했습니다. `make -C`의 정식·호환 실행 명령에서도 JSON-RPC 전용 stdout과 공시 API 불통 후 세션 유지를 확인했습니다. 실제 계정·DB·외부 시장 API는 이 통합 검사에 사용하지 않았습니다.

## 로컬 Make 명령 검증 (MCP 통합 전, 2026-10-18 KST)

아래 기록은 MCP 통합 전 별도 서버 두 개를 사용한 로컬 명령 검증입니다. 별도 Compose 프로젝트·포트·볼륨에서 확인했으며 기존 로컬 DB·Redis는 사용하지 않았습니다.

| 검증 | 결과 |
|---|---|
| `services-up` / 재실행 | 빈 볼륨에서 DB·Redis 정상 응답 확인. 재실행 시 준비된 Redis 재시작 없음 |
| DB 준비·migration | 테스트 DB 생성·재실행, 앱 DB migration 011 및 버전 조회 통과. 테스트가 아닌 DB는 실행 전 거부 |
| `check`, `test-race`, `build` | 전체 Go suite·race, 문서 링크, 웹 build/lint, 앱·CLI·MCP 바이너리 빌드 통과 |
| 통합 전 Make MCP 실행 | 별도 서버 두 개의 초기화·각 도구 3개 조회, `make -C`를 포함한 JSON-RPC 전용 stdout 확인 |
| 옵션·초기 설정 | Make 인자 전달·기본값 7개 사례, 환경설정 Ginkgo suite, Redis helper mock 14개 사례 통과. `.env.docker` 없는 Compose 설정 검사 통과 |
| `services-down` | DB·Redis 볼륨 3개 보존 확인. 이후 검증 전용 프로젝트만 정리 |

## 기존 구현의 검증 기록 (2026-10-18 KST)

아래 표는 로컬 Make 명령 정리와 MCP 통합 전에 수행한 검증 결과입니다. MCP 항목은 당시 별도 서버 구성을 대상으로 합니다.

| 검증 | 결과와 범위 |
|---|---|
| 당시 `make check` | 격리된 PostgreSQL 18.6에서 전체 Go suite, 문서 링크, 고정 Node/npm의 웹 build/lint 통과 |
| Race 검사 | Agent·worker·collector·두 MCP·공통 서비스·facts·달력·시장 수집기·KOFIA 통과. 실제 facts DB 회귀 포함 |
| 취약점 검사 | npm 전체 0건. Go 호출 경로 0건, imported package 0건. 미사용 `x/crypto/openpgp`의 module-only advisory GO-2026-5932는 남아 있으며 실행 경로 취약점으로 해석하지 않음 |
| 실제 시장 MCP | 세 도구 호출·JSON-only stdout·structured/text 동등성·산출물 저장 확인. snapshot은 20261016 요청 |
| PostgreSQL | 전체 migration 적용, pg_dump를 별도 DB로 복원. provenance 충돌 롤백은 예상대로 거부되고 원본·교정본 모두 보존 |
| Redis RDB·AOF | 격리된 Stack 7.4.7 / Asynq 0.25.1 → Redis 8.10.2 / Asynq 0.26.0. 두 모드 모두 pending/scheduled/retry/completed 각 1건의 ID·payload·재시도·시각·retention·카운터 및 재시작 후 상태 일치. 원본 fingerprint 불변 |
| 이미지 | API·worker·collector의 새 Go/Alpine 이미지 빌드 통과 |

Redis 리허설에서 AOF incremental 파일 소유권 차이를 재현해, Redis 8 시작 시 복제 target의 소유권만 정리하도록 보완했습니다. AOF 검증은 복제 target의 dump.rdb를 제거한 조건에서도 통과했습니다. 격리된 테스트 volume으로 검증했습니다.

실제 시장 호출에서는 breadth 원자료 미발표, 일부 지수 구성군 시가총액 누락, facts DB 불가가 명시적으로 남았습니다. 역사 날짜의 global/macro/late_session은 현재 전용 조회를 배제한 결과입니다. 이 결측이나 특별 거래시간·공식 정지 이력의 미검증 상태를 수치·정상 판정으로 대체하지 않았습니다. 글로벌 Codex 설정과 기존 로컬 DB·Redis 원본은 변경하지 않았습니다.
