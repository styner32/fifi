# fifi

금융 데이터 분석, 한국투자증권 API 연동, DART 공시 처리 및 AI 분석을 위한 통합 Go/TypeScript 플랫폼입니다.

---

## 디렉토리 구조 (Project Structure)

```
.
├── cmd/                        # Go 애플리케이션 진입점 (Entrypoints)
│   ├── main.go                 # 주식/선물옵션/기업분석 실행 진입점 (go run ./cmd)
│   ├── agent/                  # 에이전트 리포트 CLI (premarket, rsi, credit-balance 등)
│   ├── dart-filing-api/        # DART 공시 REST API 서버
│   ├── dart-filing-worker/     # DART 공시 백그라운드 수집 및 AI 분석 워커
│   ├── dart-filing-worker-cli/ # DART 공시 데이터 수집/백필 CLI 도구
│   └── dart-filing-mcp/        # DART 공시 MCP 서버
├── internal/                   # 비즈니스 로직 및 내부 패키지
│   ├── domesticstock/          # 국내 주식 시세, KOSPI 마스터, PBR, DCF 입력 수집
│   ├── domesticfutureoption/   # 국내 선물/옵션 시세, 마스터 캐시 및 근월물 산출
│   ├── companyanalysis/        # SEC/FRED/Stooq 기반 미국 기업 분석 및 가치평가
│   ├── dcf/                    # DCF 가치평가 엔진 및 몬테카를로 시뮬레이션
│   ├── dartfiling/             # DART 공시 데이터 파싱, AI 분석 및 워커 로직
│   ├── kosis/                  # KOSIS 통계 데이터 연동
│   └── db/                     # GORM DB 연결 및 모델 정의
├── web/                        # Vite + React (TypeScript) SPA 웹 프론트엔드
├── external/                   # 외부 서브모듈 및 참고 저장소
│   └── open-trading-api/       # 한국투자증권 open-trading-api 서브모듈
├── docs/                       # 기술 및 도메인 문서
├── Dockerfile_*                # Docker 컨테이너 빌드 파일
├── docker-compose.yml          # DB (PostgreSQL) 및 Redis 로컬 개발 환경
├── Makefile                    # 마이그레이션 및 실행 자동화 타스크
├── go.mod / go.sum             # Go 패키지 의존성
└── AGENTS.md / GEMINI.md       # 에이전트 및 프로젝트 개발 지침
```

---

## 서브모듈 관리 (Submodules)

본 프로젝트는 한국투자증권 Open Trading API reference 코드를 `external/open-trading-api`에 Git Submodule로 포함하고 있습니다.

저장소를 새로 클론한 경우 아래 명령어로 서브모듈을 초기화합니다:

```bash
git submodule update --init --recursive
```

---

## 실행 및 테스트 (Run & Test)

### 1. 기본 실행 및 전체 테스트
```bash
# 전체 테스트 실행
go test ./...

# 국내 주식 / 선물옵션 / 기업분석 통합 실행
go run ./cmd
```

### 2. 에이전트 리포트 CLI (`cmd/agent`)
상세 설명은 [`AGENT_CLI_GUIDE.md`](cmd/agent/README.md) 또는 에이전트 가이드를 참고하세요.

```bash
# 장전 취약점 스코어보드 리포트
go run ./cmd/agent report premarket

# RSI 히스토리컬 시리즈 엔진 (--interval 1d, 60m, 15m, 5m)
go run ./cmd/agent report rsi --symbol kospi --period 14 --interval 60m

# 실질 시스템 레버리지 (신용잔고 ÷ 고객예탁금) 리포트
go run ./cmd/agent report credit-balance --ratio --days 60

# 안전장치 (사이드카 & 서킷브레이커) 모니터링
go run ./cmd/agent report safety-devices
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
go run ./cmd/dart-filing-worker-cli reports

# 4. 단일 공시 건 AI 분석 Dry-run 테스트 (DB 미저장)
make dart-filing-cli RECEIPT_NO="20240321000725"
# 또는
go run ./cmd/dart-filing-worker-cli dry-run 20240321000725
```

### 4. DART Filing REST API 서버 & 워커
```bash
# API 서버 실행
go run ./cmd/dart-filing-api

# 워커 실행
go run ./cmd/dart-filing-worker
```

### 5. Web Frontend (`web`)
Vite + React 기반 프론트엔드 애플리케이션:

```bash
# 개발 서버 실행
make dart-filing-web

# 프로덕션 빌드
make dart-filing-web-build
```

---

## 데이터베이스 마이그레이션 (DB Migrations)

`DATABASE_URL` 환경 변수를 설정한 후 아래 Makefile 타스크를 사용합니다:

```bash
make migrate-up
make migrate-down
make migrate-create NAME=migration_name
```
