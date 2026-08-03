# Useful Domestic-Stock Endpoints (Market Status)

This is a practical endpoint set for real-time market status, breadth, flow, risk, and momentum analysis.

## Core Snapshot

| Path | Purpose |
|---|---|
| `/uapi/domestic-stock/v1/quotations/market-time` | 장 운영 시간/영업일 |
| `/uapi/domestic-stock/v1/quotations/inquire-index-price` | KOSPI/KOSDAQ/KOSPI200/VKOSPI 현재지수 |
| `/uapi/domestic-stock/v1/quotations/inquire-index-timeprice` | 업종 분봉 지수 |
| `/uapi/domestic-stock/v1/quotations/inquire-index-daily-price` | 업종 일/주/월 지수 |
| `/uapi/domestic-stock/v1/quotations/inquire-index-tickprice` | 업종 틱 지수 |
| `/uapi/domestic-stock/v1/quotations/mktfunds` | 증시자금 종합 |
| `/uapi/domestic-stock/v1/quotations/inquire-vi-status` | VI 발동 현황 |

## Flow / Sentiment

| Path | Purpose |
|---|---|
| `/uapi/domestic-stock/v1/quotations/comp-program-trade-today` | 프로그램매매 시간대 동향 |
| `/uapi/domestic-stock/v1/quotations/comp-program-trade-daily` | 프로그램매매 일별 동향 |
| `/uapi/domestic-stock/v1/quotations/investor-program-trade-today` | 투자자 프로그램매매 당일 동향 |
| `/uapi/domestic-stock/v1/quotations/inquire-investor-daily-by-market` | 시장별 투자자매매 동향 |
| `/uapi/domestic-stock/v1/quotations/foreign-institution-total` | 외인/기관 매매집계 |
| `/uapi/domestic-stock/v1/quotations/frgnmem-trade-estimate` | 외국계 매매 가집계 |
| `/uapi/domestic-stock/v1/quotations/frgnmem-trade-trend` | 외국계/회원사 체결 동향 |
| `/uapi/domestic-stock/v1/quotations/investor-trend-estimate` | 종목 외인/기관 추정 집계 |

## Momentum / Breadth / Ranking

| Path | Purpose |
|---|---|
| `/uapi/domestic-stock/v1/ranking/volume-power` | 체결강도 상위 |
| `/uapi/domestic-stock/v1/ranking/volume-rank` | 거래량 순위 |
| `/uapi/domestic-stock/v1/ranking/fluctuation` | 상승/하락률 랭킹 |
| `/uapi/domestic-stock/v1/ranking/near-new-highlow` | 신고/신저 근접 |
| `/uapi/domestic-stock/v1/ranking/market-cap` | 시가총액 상위 |
| `/uapi/domestic-stock/v1/quotations/capture-uplowprice` | 상/하한가 포착 |

## Execution / Microstructure

| Path | Purpose |
|---|---|
| `/uapi/domestic-stock/v1/quotations/inquire-time-itemconclusion` | 종목 체결 시계열(초단위) |
| `/uapi/domestic-stock/v1/quotations/pbar-tratio` | 매물대/거래비중 |
| `/uapi/domestic-stock/v1/quotations/tradprt-byamt` | 체결금액대별 매매비중 |
| `/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice` | RSI/기술지표용 OHLCV 원천 |

## Notes

- VKOSPI code can differ by environment. Use `VKOSPI_INDEX_CODE` env var if auto-detection fails.
- RSI is not a direct KIS endpoint; compute it from `inquire-daily-itemchartprice` closes (`stck_clpr`).
