# 장중 펄스

현재 구현 계약은 [NOW_DATA_CONTRACT.md](NOW_DATA_CONTRACT.md)입니다. 기존 설계는 [초기 설계 보관본](archive/INTRADAY_PULSE_INITIAL_DESIGN.md)에 보존하며, 구현 지침으로 사용하지 않습니다.

```bash
make now
make now-debug
go run ./cmd/agent report intraday-pulse --json --no-save
```

`now`는 당일 비교 이력을 읽고 적립합니다. `now-debug`는 이력을 읽거나 보고서를 적립하지 않으며, 인증·원천 데이터 캐시는 갱신할 수 있습니다. `--lookback` 플래그는 없으며 비교 창은 2시간입니다.

- 현재값과 기준점의 실제 관측 날짜·시각이 필요합니다. 조회 시작 시각을 관측 시각으로 대체하지 않습니다.
- 1h/2h 기준점은 목표 시각 이전 5분 이내여야 합니다. 짧은 시계열의 첫 값을 대신 사용하지 않습니다.
- 누적 수급 차이를 실제 시간 검증 없이 최근 1h/2h 수급으로 표시하지 않습니다.
- 선물·현물 산술 차이는 시각·계약 정합성 검증 전 `RAW_SPREAD / ALIGNMENT_UNVERIFIED`입니다.
- 공식 안전장치 발동·해제 기록과 조건 관측을 구분합니다. 가격만으로 발동·투매·바닥을 추론하지 않습니다.
- 시장 달력의 개장 여부와 정규 시간표에 따른 세션 추정을 구분합니다. 특별 거래시간은 별도 확인이 필요합니다.
- 이력 부족, 출처 시각 미확인, 미검증 모델은 `null / NOT_EVALUATED`로 유지합니다. 종합은 `COMPOSITE_NOT_CIRCULABLE`입니다.

LLM 연결은 [시장 MCP](MARKET_MCP.md), 검증은 [테스트 가이드](TESTING.md)를 참고하세요.
