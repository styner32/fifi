# AGENT.md — 선물옵션 동시만기일 데이터 수집 에이전트

> **목적**: 한국투자증권(KIS) Open API를 사용하여 선물·옵션 동시 만기일(쿼드러플 위칭데이)에 필요한 핵심 수급 데이터를 자동 수집하고 분석 리포트를 생성한다.
>
> **만기일 일정**: 매년 3, 6, 9, 12월 둘째 주 목요일

---

## 1. 사전 준비

### 1.1 KIS Developers 서비스 신청

```
포털: https://apiportal.koreainvestment.com
GitHub 샘플: https://github.com/koreainvestment/open-trading-api
```

### 1.2 환경 변수 (`config.yaml`)

```yaml
# 실전투자
app_key: "YOUR_APP_KEY" # 36자리
app_secret: "YOUR_APP_SECRET" # 180자리
hts_id: "YOUR_HTS_ID"

# 계좌
acct_stock: "00000000" # 증권계좌 앞 8자리
acct_future: "00000000" # 선물옵션계좌 앞 8자리
prod_code: "01" # 01: 종합, 03: 국내선물옵션

# 서버
base_url: "https://openapi.koreainvestment.com:9443" # 실전
# base_url: "https://openapivts.koreainvestment.com:29443"  # 모의
```

### 1.3 의존성

```
pip install requests pyyaml pandas websockets tabulate
```

---

## 2. 인증 (OAuth 토큰 발급)

```
POST /oauth2/tokenP

Body:
{
  "grant_type": "client_credentials",
  "appkey": "{app_key}",
  "appsecret": "{app_secret}"
}

Response → access_token (유효기간: 약 24시간)
```

**헤더 공통 포맷** (이후 모든 REST 호출에 적용):

```
authorization: Bearer {access_token}
appkey: {app_key}
appsecret: {app_secret}
tr_id: {각 API별 거래ID}
content-type: application/json; charset=utf-8
```

---

## 3. 핵심 데이터 수집 API 매핑

만기일 당일 수집해야 할 데이터와 대응 API를 정리한다.

### 3.1 프로그램매매 종합현황 (시간별)

| 항목                  | 값                                                                                                     |
| --------------------- | ------------------------------------------------------------------------------------------------------ |
| **용도**              | 차익/비차익 프로그램 순매수·순매도 추이 실시간 모니터링                                                |
| **URL**               | `GET /uapi/domestic-stock/v1/quotations/comp-program-trade-today`                                      |
| **tr_id**             | `FHPPG04650200`                                                                                        |
| **핵심 파라미터**     | `MKSC_SHRN_ISCD`: 시장코드 (코스피: `0001`, 코스닥: `1001`)                                            |
| **핵심 응답 필드**    | `acml_tr_pbmn` (누적거래대금), `prsm_nslg_pbmn` (차익순매수대금), `nprsm_nslg_pbmn` (비차익순매수대금) |
| **폴링 주기**         | 장중 1분~5분                                                                                           |
| **만기일 체크포인트** | 차익매도 급증 시 현물 하방 압력 시그널                                                                 |

### 3.2 투자자별 매매동향 (종목별)

| 항목               | 값                                                                                                     |
| ------------------ | ------------------------------------------------------------------------------------------------------ |
| **용도**           | 외국인·기관·개인 순매수/순매도 확인                                                                    |
| **URL**            | `GET /uapi/domestic-stock/v1/quotations/inquire-investor`                                              |
| **tr_id**          | `FHKST01010900`                                                                                        |
| **핵심 파라미터**  | `FID_INPUT_ISCD`: 종목코드 (코스피200: `0001`)                                                         |
| **핵심 응답 필드** | `frgn_ntby_qty` (외국인순매수수량), `orgn_ntby_qty` (기관순매수수량), `prsn_ntby_qty` (개인순매수수량) |

### 3.3 외국인 매매종목 가집계

| 항목              | 값                                                                 |
| ----------------- | ------------------------------------------------------------------ |
| **용도**          | 외국인 순매수 상위/하위 종목 파악                                  |
| **URL**           | `GET /uapi/domestic-stock/v1/quotations/foreign-institution-total` |
| **tr_id**         | `FHPTJ04400000`                                                    |
| **핵심 파라미터** | `FID_INPUT_ISCD`: `0000` (전체), `FID_TRGT_CLS_CODE`: `1` (외국인) |

### 3.4 국내선물옵션 현재가 (코스피200 선물)

| 항목                  | 값                                                                                               |
| --------------------- | ------------------------------------------------------------------------------------------------ |
| **용도**              | 선물 현재가, 베이시스(선물-현물 괴리) 계산                                                       |
| **URL**               | `GET /uapi/domestic-futureoption/v1/quotations/inquire-price`                                    |
| **tr_id**             | `FHMIF10000000`                                                                                  |
| **핵심 파라미터**     | `FID_INPUT_ISCD`: 선물 종목코드 (예: `101V06` = 2026년 6월물, 근월물 코드는 마스터파일에서 확인) |
| **핵심 응답 필드**    | `futs_prpr` (선물현재가), `bstp_nmix_prpr` (기초지수현재가)                                      |
| **베이시스 계산**     | `basis = futs_prpr - bstp_nmix_prpr`                                                             |
| **만기일 체크포인트** | 백워데이션(basis < 0) 심화 → 차익매도 가능성                                                     |

### 3.5 국내선물옵션 시간별 체결

| 항목              | 값                                                                    |
| ----------------- | --------------------------------------------------------------------- |
| **용도**          | 선물 체결 추이, 거래량 급변 감지                                      |
| **URL**           | `GET /uapi/domestic-futureoption/v1/quotations/inquire-time-fuopccnl` |
| **tr_id**         | `FHMIF10020000`                                                       |
| **핵심 파라미터** | `FID_INPUT_ISCD`: 선물 종목코드                                       |

### 3.6 국내선물옵션 투자자별 매매동향

| 항목                  | 값                                                             |
| --------------------- | -------------------------------------------------------------- |
| **용도**              | 외국인/기관 선물 순매수·순매도 포지션 확인 (핵심 중의 핵심)    |
| **URL**               | `GET /uapi/domestic-futureoption/v1/quotations/inquire-member` |
| **tr_id**             | `FHMIF10070000`                                                |
| **핵심 파라미터**     | `FID_INPUT_ISCD`: 선물 종목코드                                |
| **만기일 체크포인트** | 외국인 선물 순매수 유지 여부 → 시장 하방 경직성 판단의 1순위   |

### 3.7 코스피200 지수 현재가

| 항목              | 값                                                     |
| ----------------- | ------------------------------------------------------ |
| **용도**          | 현물 지수 실시간 추적, 베이시스 계산의 기준            |
| **URL**           | `GET /uapi/domestic-stock/v1/quotations/inquire-price` |
| **tr_id**         | `FHKST01010100`                                        |
| **핵심 파라미터** | `FID_INPUT_ISCD`: `0001` (코스피)                      |

### 3.8 주식현재가 호가/예상체결

| 항목                  | 값                                                                    |
| --------------------- | --------------------------------------------------------------------- |
| **용도**              | 동시호가(15:20~15:30) 시 예상 체결가/수량 모니터링                    |
| **URL**               | `GET /uapi/domestic-stock/v1/quotations/inquire-asking-price-exp-ccn` |
| **tr_id**             | `FHKST01010200`                                                       |
| **핵심 파라미터**     | `FID_INPUT_ISCD`: 종목코드                                            |
| **만기일 체크포인트** | 마감 10분 전 동시호가 예상체결가 급변 감지                            |

---

## 4. 실시간 WebSocket 구독

REST 폴링 외에 실시간 시세를 받으려면 WebSocket 사용.

```
WebSocket URL: ws://ops.koreainvestment.com:21000  (실전)
              ws://ops.koreainvestment.com:31000  (모의)
```

### 4.1 WebSocket 접속키 발급

```
POST /oauth2/Approval

Body:
{
  "grant_type": "client_credentials",
  "appkey": "{app_key}",
  "secretkey": "{app_secret}"
}

Response → approval_key
```

### 4.2 핵심 실시간 TR (구독 등록)

| TR_ID      | 설명                 | 만기일 활용                  |
| ---------- | -------------------- | ---------------------------- |
| `H0STCNT0` | 국내주식 실시간 체결 | 시총 상위 종목 체결 모니터링 |
| `H0STASP0` | 국내주식 실시간 호가 | 동시호가 수급 변화 감지      |
| `H0IFCNT0` | 국내선물 실시간 체결 | 선물 체결 속도·방향          |
| `H0IFASP0` | 국내선물 실시간 호가 | 선물 호가 스프레드 변화      |

### 4.3 구독 메시지 포맷

```json
{
  "header": {
    "approval_key": "{approval_key}",
    "custtype": "P",
    "tr_type": "1",
    "content-type": "utf-8"
  },
  "body": {
    "input": {
      "tr_id": "H0IFCNT0",
      "tr_key": "{선물종목코드}"
    }
  }
}
```

---

## 5. 에이전트 실행 흐름

```
┌─────────────────────────────────────────────────────┐
│                  만기일 에이전트 타임라인               │
├──────────┬──────────────────────────────────────────┤
│ 08:30    │ 토큰 발급, 선물 근월물 종목코드 확인         │
│          │ WebSocket 연결 & 실시간 TR 구독             │
├──────────┼──────────────────────────────────────────┤
│ 09:00    │ 장 시작 — 1차 스냅샷 수집                   │
│          │ - 프로그램매매 현황                          │
│          │ - 외국인/기관 선물 포지션                     │
│          │ - 베이시스 (선물 - 현물)                     │
├──────────┼──────────────────────────────────────────┤
│ 09:00    │ [루프] 5분 간격 REST 폴링                   │
│  ~       │ - 프로그램매매 종합 (차익/비차익)             │
│ 14:50    │ - 투자자별 매매동향 (외국인 선물)             │
│          │ - 베이시스 변화 추적                         │
│          │ - 변동성 임계값 초과 시 알림                  │
├──────────┼──────────────────────────────────────────┤
│ 14:50    │ [경고 모드] 폴링 주기 1분으로 단축            │
│  ~       │ - 프로그램 매수/매도 급변 감지               │
│ 15:20    │ - 종가 방향성 예측 리포트 생성               │
├──────────┼──────────────────────────────────────────┤
│ 15:20    │ [동시호가 감시 모드]                         │
│  ~       │ - 예상체결가 실시간 모니터링                  │
│ 15:30    │ - 바스켓 매매 물량 급변 감지                  │
│          │ - 종가 왜곡 알림                             │
├──────────┼──────────────────────────────────────────┤
│ 15:30    │ 장 마감 — 최종 리포트 생성                   │
│  ~       │ - 당일 수급 요약                             │
│ 15:45    │ - 6월물 전환 포지션 분석                     │
│          │ - 다음 거래일 전략 시사점                     │
└──────────┴──────────────────────────────────────────┘
```

---

## 6. 핵심 분석 로직

### 6.1 베이시스 모니터링

```python
def calc_basis(futures_price: float, spot_index: float) -> dict:
    basis = futures_price - spot_index
    basis_pct = (basis / spot_index) * 100

    signal = "NEUTRAL"
    if basis_pct < -0.3:
        signal = "DEEP_BACKWARDATION"   # 차익매도 압력 극대
    elif basis_pct < 0:
        signal = "BACKWARDATION"        # 차익매도 가능성
    elif basis_pct > 0.5:
        signal = "CONTANGO"             # 차익매수 유입 가능

    return {
        "basis": round(basis, 2),
        "basis_pct": round(basis_pct, 4),
        "signal": signal,
        "timestamp": datetime.now().isoformat()
    }
```

### 6.2 프로그램매매 순매수 추이 알림

```python
def check_program_alert(current: dict, prev: dict, threshold_bn: float = 500) -> str | None:
    """
    차익/비차익 순매수 변화가 threshold (억원) 이상이면 알림
    """
    arb_delta = current["arb_net_buy"] - prev["arb_net_buy"]  # 차익
    nonarb_delta = current["nonarb_net_buy"] - prev["nonarb_net_buy"]  # 비차익

    alerts = []
    if abs(arb_delta) > threshold_bn:
        direction = "매수" if arb_delta > 0 else "매도"
        alerts.append(f"⚠ 차익 프로그램 {direction} 급변: {arb_delta:+,.0f}억")
    if abs(nonarb_delta) > threshold_bn:
        direction = "매수" if nonarb_delta > 0 else "매도"
        alerts.append(f"⚠ 비차익 프로그램 {direction} 급변: {nonarb_delta:+,.0f}억")

    return "\n".join(alerts) if alerts else None
```

### 6.3 외국인 선물 포지션 추적

```python
def analyze_foreign_futures(data: dict) -> dict:
    """
    외국인 선물 순매수가 시장 방향을 결정하는 핵심 변수.
    기관 매도를 외국인이 흡수하는지 여부가 만기일 종가의 키.
    """
    frgn_net = data["frgn_buy_amt"] - data["frgn_sell_amt"]
    inst_net = data["inst_buy_amt"] - data["inst_sell_amt"]

    if frgn_net > 0 and abs(frgn_net) > abs(inst_net):
        stance = "BULLISH_DEFENSE"    # 외국인 매수가 기관 매도 흡수
    elif frgn_net < 0 and inst_net < 0:
        stance = "DOUBLE_SELL"        # 외국인+기관 동반 매도 → 위험
    elif frgn_net > 0 and inst_net > 0:
        stance = "DOUBLE_BUY"         # 동반 매수 → 강세
    else:
        stance = "MIXED"

    return {
        "frgn_net_bn": round(frgn_net / 1e8, 1),
        "inst_net_bn": round(inst_net / 1e8, 1),
        "stance": stance
    }
```

### 6.4 동시호가 왜곡 감지 (15:20~15:30)

```python
def detect_closing_auction_distortion(
    pre_auction_price: float,
    last_regular_price: float,
    threshold_pct: float = 0.5
) -> dict:
    """
    동시호가 예상체결가가 정규장 마지막 체결가 대비
    threshold_pct 이상 괴리되면 수급 왜곡 경고
    """
    gap_pct = ((pre_auction_price - last_regular_price) / last_regular_price) * 100

    distorted = abs(gap_pct) > threshold_pct
    direction = "상방왜곡" if gap_pct > 0 else "하방왜곡"

    return {
        "pre_auction_price": pre_auction_price,
        "gap_pct": round(gap_pct, 3),
        "distorted": distorted,
        "direction": direction if distorted else "정상"
    }
```

---

## 7. 선물 종목코드 참조

KIS에서 선물/옵션 종목코드는 마스터파일을 통해 매일 갱신된다.

| 상품                      | 코드 형식         | 예시                    |
| ------------------------- | ----------------- | ----------------------- |
| 코스피200 선물 (근월물)   | `101{YM}`         | `101V03` = 2026년 3월물 |
| 코스피200 선물 (차근월물) | `101{YM}`         | `101V06` = 2026년 6월물 |
| 코스피200 콜옵션          | `201{YM}{행사가}` | —                       |
| 코스피200 풋옵션          | `301{YM}{행사가}` | —                       |

> **월 코드**: 1~9월 = `01`~`09`, 10월 = `10`, 11월 = `11`, 12월 = `12`
>
> **연도 코드**: `V` = 2026 (연도 코드는 마스터파일 참조)
>
> **마스터파일 다운로드**: KIS Developers 포탈 → API문서 → 종목정보 마스터파일
> 또는 `https://new.real.download.dws.co.kr/common/master/fo_stk_code_mts.2.dat` 형태로 매일 갱신

---

## 8. Rate Limit 주의사항

```
REST API: 초당 20건 (실전), 초당 5건 (모의)
WebSocket: 최대 40개 종목 동시 구독
토큰 유효기간: 약 24시간 (장 시작 전 갱신 권장)
```

- 5분 간격 폴링 기준 6개 API × 1회 = 6건/5분 → Rate Limit 충분
- 동시호가 1분 간격 전환 시에도 6건/분 → 초과 없음
- 모의투자 계좌는 Rate Limit이 낮으므로 실전계좌 권장

---

## 9. 프로젝트 구조 (권장)

```text
.
├── cmd/
│   ├── main.go                    # bootstrap (.env 로드 + runApp 호출)
│   ├── app.go                     # quad witching 포함 전체 실행 orchestration
│   ├── config.go                  # env/default 로딩
│   ├── render_quadwitching.go     # quad witching 요약 출력
│   └── path_helpers.go            # business-date/path helper
├── internal/
│   ├── domesticstock/
│   │   ├── market_snapshot.go     # 현물 지수/장 상태/RSI
│   │   ├── quotation_analysis.go  # 투자자/외인/동시호가
│   │   ├── kospi_master_cache.go  # dated KOSPI master + JSON sidecar
│   │   └── pbr.go                 # KOSPI weighted PBR 계산
│   ├── domesticfutureoption/
│   │   ├── quotes.go              # 선물 시세/전광판/예상체결
│   │   ├── contract_resolution.go # 근월물 KOSPI200 선물 결정
│   │   └── master_cache.go        # dated index futures master + JSON sidecar
│   └── quadwitching/
│       ├── schedule.go            # 실행 윈도우 계산
│       └── export.go              # snapshot JSON export
└── README.md
```

---

## 10. 만기일 당일 체크리스트 (운영용)

- [ ] 08:30 — 토큰 발급 확인, 선물 근월물 종목코드 확인
- [ ] 08:50 — WebSocket 연결 상태 확인
- [ ] 09:00 — 프로그램매매 초기값 기록, 외국인 선물 포지션 첫 스냅샷
- [ ] 09:00~14:50 — 5분 간격 모니터링 루프 정상 동작 확인
- [ ] 14:50 — 폴링 1분 간격 전환, 경고 모드 진입
- [ ] 15:20 — 동시호가 감시 모드 진입, 예상체결가 추적 시작
- [ ] 15:30 — 최종 종가 기록, 최종 리포트 생성
- [ ] 15:45 — 6월물 외국인 순매수 잔고 확인, 익일 전략 시사점 정리

---

## 11. API 상세 호출 예시 (Python)

### 11.1 토큰 발급

```python
import requests
import yaml

with open("config.yaml") as f:
    cfg = yaml.safe_load(f)

def get_token():
    url = f"{cfg['base_url']}/oauth2/tokenP"
    body = {
        "grant_type": "client_credentials",
        "appkey": cfg["app_key"],
        "appsecret": cfg["app_secret"]
    }
    res = requests.post(url, json=body)
    return res.json()["access_token"]
```

### 11.2 프로그램매매 종합현황 조회

```python
def get_program_trade(token: str, market: str = "0001"):
    """market: 0001=코스피, 1001=코스닥"""
    url = f"{cfg['base_url']}/uapi/domestic-stock/v1/quotations/comp-program-trade-today"
    headers = {
        "authorization": f"Bearer {token}",
        "appkey": cfg["app_key"],
        "appsecret": cfg["app_secret"],
        "tr_id": "FHPPG04650200",
        "content-type": "application/json; charset=utf-8"
    }
    params = {
        "FID_COND_MRKT_DIV_CODE": "J",
        "FID_INPUT_ISCD": market
    }
    res = requests.get(url, headers=headers, params=params)
    return res.json()
```

### 11.3 선물 현재가 + 베이시스 계산

```python
def get_futures_price(token: str, futures_code: str):
    url = f"{cfg['base_url']}/uapi/domestic-futureoption/v1/quotations/inquire-price"
    headers = {
        "authorization": f"Bearer {token}",
        "appkey": cfg["app_key"],
        "appsecret": cfg["app_secret"],
        "tr_id": "FHMIF10000000",
        "content-type": "application/json; charset=utf-8"
    }
    params = {
        "FID_COND_MRKT_DIV_CODE": "F",
        "FID_INPUT_ISCD": futures_code
    }
    res = requests.get(url, headers=headers, params=params)
    data = res.json().get("output", {})

    futures_price = float(data.get("futs_prpr", 0))
    spot_index = float(data.get("bstp_nmix_prpr", 0))

    return {
        "futures_price": futures_price,
        "spot_index": spot_index,
        "basis": round(futures_price - spot_index, 2),
        "basis_pct": round((futures_price - spot_index) / spot_index * 100, 4) if spot_index else 0
    }
```

### 11.4 투자자별 매매동향 조회

```python
def get_investor_trend(token: str, stock_code: str = "0001"):
    url = f"{cfg['base_url']}/uapi/domestic-stock/v1/quotations/inquire-investor"
    headers = {
        "authorization": f"Bearer {token}",
        "appkey": cfg["app_key"],
        "appsecret": cfg["app_secret"],
        "tr_id": "FHKST01010900",
        "content-type": "application/json; charset=utf-8"
    }
    params = {
        "FID_COND_MRKT_DIV_CODE": "J",
        "FID_INPUT_ISCD": stock_code
    }
    res = requests.get(url, headers=headers, params=params)
    return res.json()
```

---

## 12. 확장 포인트

### KIS Open API 외 보조 데이터 소스

| 데이터                   | 출처                              | 용도                     |
| ------------------------ | --------------------------------- | ------------------------ |
| VKOSPI (변동성지수)      | KRX / 한국거래소 정보데이터시스템 | 옵션 내재변동성 확인     |
| KOSPI200 선물 미결제약정 | KRX data.krx.co.kr                | 만기 전 미청산 물량 규모 |
| 야간선물 (CME)           | KIS 해외선물 API 또는 외부        | 갭 상승/하락 예측        |

### NXT(넥스트레이드) 연동

만기일 종가 왜곡 후 NXT 시간외거래(15:40~)에서 저가 매수 기회가 발생할 수 있다. KIS API에서 NXT 시간외 주문도 지원하므로 에이전트 확장 가능.

```
NXT 메인마켓: 10:00:30 ~ 15:20:00
NXT 시간외: 15:40 ~ 20:00 (유안타/한투 등 지원 증권사)
```

---

## 13. 참고 링크

- [KIS Developers 포탈](https://apiportal.koreainvestment.com)
- [KIS Open API GitHub](https://github.com/koreainvestment/open-trading-api)
- [KRX 정보데이터시스템](https://data.krx.co.kr)
- [python-kis 커뮤니티 라이브러리](https://github.com/Soju06/python-kis)
- [KIS MCP Server](https://smithery.ai/server/@KISOpenAPI/kis-code-assistant-mcp)

---
