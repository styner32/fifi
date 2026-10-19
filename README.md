# fifi

금융 데이터 분석, 한국투자증권 API 연동, DART 공시 처리 및 AI 분석을 위한 통합 Go/TypeScript 플랫폼입니다.

---

## 디렉토리 구조 (Project Structure)

```
.
├── cmd/                        # Go 애플리케이션 진입점 (Entrypoints)
│   ├── main.go                 # 주식/선물옵션/기업분석 실행 진입점 (go run ./cmd)
│   ├── agent/                  # 에이전트 리포트 CLI (premarket, rsi, credit-balance 등)
│   ├── market-collector/       # 시장 facts·주요 공시 수집 및 백필
│   ├── dart-filing-api/        # DART 공시 REST API 서버
│   ├── dart-filing-worker/     # DART 공시 백그라운드 수집 및 AI 분석 워커
│   ├── dart-filing-worker-cli/ # DART 공시 데이터 수집/백필 CLI 도구
│   ├── fifi-mcp/               # 시장·공시 도구 6개를 제공하는 통합 MCP 서버
│   ├── market-mcp/             # 통합 MCP 서버의 기존 실행 경로 호환 wrapper
│   └── dart-filing-mcp/        # 통합 MCP 서버의 기존 실행 경로 호환 wrapper
├── internal/                   # 비즈니스 로직 및 내부 패키지
│   ├── domesticstock/          # 국내 주식 시세, KOSPI 마스터, PBR, DCF 입력 수집
│   ├── domesticfutureoption/   # 국내 선물/옵션 시세, 마스터 캐시 및 근월물 산출
│   ├── companyanalysis/        # SEC/FRED/Stooq 기반 미국 기업 분석 및 가치평가
│   ├── dcf/                    # DCF 가치평가 엔진 및 몬테카를로 시뮬레이션
│   ├── dartfiling/             # DART 공시 데이터 파싱, AI 분석 및 워커 로직
│   ├── external/kosis/         # KOSIS 통계 데이터 연동
│   ├── marketreport/           # CLI·MCP 공통 수집·저장 서비스
│   ├── fifimcp/                # 통합 MCP 도구 등록·프로토콜 서버
│   └── db/                     # GORM DB 연결 및 모델 정의
├── web/                        # Vite + React (TypeScript) SPA 웹 프론트엔드
├── external/                   # 외부 서브모듈 및 참고 저장소
│   └── open-trading-api/       # 한국투자증권 open-trading-api 서브모듈
├── docs/                       # 기술 및 도메인 문서
├── Dockerfile_*                # Docker 컨테이너 빌드 파일
├── docker-compose.yml          # DB (PostgreSQL) 및 Redis 로컬 개발 환경
├── Makefile                    # 마이그레이션 및 실행 자동화 타스크
├── go.mod / go.sum             # Go 패키지 의존성
└── AGENTS.md                   # 에이전트 및 프로젝트 개발 지침
```

---

## 로컬에서 시작하기

Go 1.27.2, Node 24.22.0 LTS, npm 12.2.0과 Docker Compose를 사용합니다. 저장소 루트에서 실행합니다.

```bash
make help
make setup
make services-up
make migrate-up
make test-db-prepare
make check
```

`setup`은 Go 의존성, npm lock 기반 설치와 고정된 서브모듈 초기화를 수행합니다. `services-up`은 로컬 PostgreSQL·Redis만 시작합니다. API·워커·웹은 아래 명령으로 각각 실행합니다.

`.env`에 로컬 API 키와 설정을 둡니다. Make 명령의 설정 우선순위는 명시한 환경변수·Make 할당 → `.env` → 로컬 기본값입니다. 테스트는 별도의 `TEST_DATABASE_URL`과 `TEST_REDIS_URL`을 사용하며 DB 이름은 `_test`로 끝나야 합니다. 자세한 명령과 데이터 보존 방식은 [로컬 개발 가이드](docs/MAINTENANCE.md)를 참고하세요.

## 서브모듈 관리

본 프로젝트는 한국투자증권 Open Trading API reference 코드를 `external/open-trading-api`에 Git Submodule로 포함하고 있습니다.

저장소를 새로 클론한 경우 아래 명령어로 서브모듈을 초기화합니다:

```bash
make submodule-init
```

상위 원본 레포지토리의 최신 `main` 커밋으로 업데이트하려면 아래 명령어를 사용합니다:

```bash
make submodule-update
```

---

## 실행 및 테스트 (Run & Test)

Go 테스트는 **Ginkgo v2 + Gomega의 BDD 스타일**로 작성합니다. 신규·수정 테스트는 `Describe` → `Context` → `It`을 사용하고, 반복 사례는 `DescribeTable`로 표현합니다. 작성 규칙과 실행 예시는 [테스트 가이드](docs/TESTING.md)를 참고하세요.

### 1. 기본 실행 및 전체 테스트

```bash
# 격리된 테스트 DB 준비 후 전체 Ginkgo 스위트 실행
make test-db-prepare
make test
make test-race

# 국내 주식 / 선물옵션 / 기업분석 통합 실행
make run
```

### 2. 에이전트 리포트 CLI (`cmd/agent`)
상세 설명은 [`AGENT_CLI_GUIDE.md`](docs/AGENT_CLI_GUIDE.md) 또는 에이전트 가이드를 참고하세요.

```bash
# 장전 관측 데이터·품질 상태 리포트
make premarket

# 장중 펄스·지정일 종합 스냅샷
make now
make takesnapshot DATE=20261016

# JSON 출력과 보고서 이력 저장 생략
make now ARGS="--json --no-save"

# RSI 히스토리컬 시리즈 엔진 (--interval 1d, 60m, 15m, 5m)
make rsi ARGS="--symbol kospi --period 14 --interval 60m"

# 동일 기준일 신용융자 / 고객예탁금 비율 (결측은 null)
make leverage ARGS="--days 60"
# make creditbalance도 기본적으로 같은 비율을 조회합니다.

# 안전장치 (사이드카 & 서킷브레이커) 모니터링
make safety-devices
```

### 3. DART Filing Worker CLI (`cmd/dart-filing-worker-cli`)
```bash
# 1. 전체 기업 마스터(companies 테이블) 백필
make dart-filing-cli-companies
# 또는
go run ./cmd/dart-filing-worker-cli companies

# 2. 특정 기업 공시 수집 및 AI 분석 저장 (예: 삼성전자 corp_code="00126380", 최근 5건)
make dart-filing-cli-company CORP_CODE="00126380" LIMIT=5
# 또는
go run ./cmd/dart-filing-worker-cli company 00126380 5

# 3. 전체 최근 공시 수집 및 분석 저장
make dart-filing-cli-reports

# 4. 단일 공시 건 AI 분석 Dry-run 테스트 (DB 미저장)
make dart-filing-cli RECEIPT_NO="20240321000725"
# 또는
go run ./cmd/dart-filing-worker-cli dry-run 20240321000725
```

### 4. DART Filing REST API 서버 & 워커
```bash
# API 서버 실행
make dart-filing-api

# 워커 실행
make dart-filing-worker
```

### 5. Web Frontend (`web`)
Vite + React 기반 프론트엔드 애플리케이션:

```bash
# 개발 서버 실행
make dart-filing-web

# 로컬 확인용 빌드·lint
make dart-filing-web-build
make web-lint
```

---

## 데이터베이스 마이그레이션 (DB Migrations)

`DATABASE_URL` 환경 변수를 설정한 후 아래 Makefile 타스크를 사용합니다:

```bash
make migrate-up
make migrate-status
make migrate-down
make migrate-create NAME=migration_name
```

## 로컬 수집기와 MCP

```bash
make market-collector-once
make market-collector
make market-collector-backfill
make mcp-build
make mcp
```

통합 `fifi` MCP 서버 하나가 시장 도구 3개와 공시 조회 도구 3개를 제공합니다. `make mcp-build`는 `bin/fifi-mcp`를 빌드하고 `make mcp`는 stdout을 JSON-RPC 전용으로 사용하는 서버를 실행합니다. `market-mcp`·`dart-filing-mcp`는 같은 통합 서버를 시작하는 호환 명령입니다. 기존 두 MCP 연결은 `fifi` 연결 하나로 교체하면 됩니다. 설정 예시는 [MCP 가이드](docs/MARKET_MCP.md)에 있습니다.

시장 도구는 `premarket`, `now`, `takesnapshot`의 공통 시장 수집기를 호출합니다. 공시 조회는 실행 중인 DART REST API에 연결하며 `make dart-filing-api`로 따로 시작합니다. 공시 API를 사용할 수 없는 동안에도 MCP 초기화·도구 목록·시장 도구는 동작합니다.

- [통합 MCP 연결과 도구](docs/MARKET_MCP.md)
- [로컬 준비·실행·검증 명령](docs/MAINTENANCE.md)
- [장전](docs/PREMARKET_DATA_CONTRACT.md), [장중](docs/NOW_DATA_CONTRACT.md), [스냅샷](docs/SNAPSHOT_DATA_CONTRACT.md) 데이터 계약

`make services-down`은 컨테이너를 중지하고 로컬 DB·Redis 볼륨을 보존합니다. Redis의 기존 데이터는 최초 준비 시 별도 Redis 8 볼륨으로 복제하고 원본을 유지합니다. 워커는 재시작 시 기존 작업을 유지하며, 새 공시 배치는 `DART_WORKER_BOOTSTRAP_FETCH_REPORTS=true make dart-filing-worker`로 요청할 수 있습니다.
