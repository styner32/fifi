# DCF 구현 계약

현재 구현은 `internal/dcf/engine.go`, `advanced.go`, `internal/domesticstock/dcf_inputs.go`를 기준으로 합니다. 필드별 출처는 [DCF 데이터 맵](DCF_DATA_MAP.md)에 정리합니다. [초기 설계](archive/DCF_INITIAL_DESIGN.md)는 역사 문서이며 추가 라이브러리·모델 도입 지침이 아닙니다.

## 입력 신뢰성과 우선순위

- `exact`: 원 API 필드, `derived`: 원 API 필드로 계산, `assumed`: 명시 옵션·환경변수·기본 가정. 파생·가정을 원자료 사실로 승격하지 않습니다.
- 옵션의 명시 값이 먼저 적용되고, 비어 있는 옵션에 환경변수를 채웁니다. 무위험금리·베타·시장 프리미엄·순부채·차입비용의 명시 override가 자동 조회보다 우선합니다.
- 무위험금리: 명시 값 → 지정 국채의 KIS 시세 → `comp-interest`.
- 베타: 명시 값 → 날짜가 일치하는 KIS 주식·지수 일수익률 회귀.
- 시장 프리미엄: 명시 값 → 선택한 공급원 (`damodaran` 기본, `file` 선택 가능) → 공급원 실패 시 가정값.
- 순부채·차입비용은 proxy일 수 있으므로 출력의 출처·상태·가정 설명을 보존합니다.

## 계산과 단위

```text
FCF = EBIT * (1 - EffectiveTax) + DnA - CapEx - ChangeInNWC
Ke = RiskFreeRate + Beta * MarketPremium
WACC = EquityWeight * Ke + DebtWeight * CostOfDebt * (1 - EffectiveTax)
TV = FinalYearFCF * (1 + TerminalGrowth) / (WACC - TerminalGrowth)
EnterpriseValue = sum(PV(FCF)) + PV(TV)
EquityValue = EnterpriseValue - NetDebt
TargetPrice (KRW/share) = EquityValue * 100000000 / SharesOut
```

재무 입력·EV·주주가치의 기본 단위는 억원이며 목표주가는 원/주입니다. 다른 단위를 사용할 때는 `Assumptions.TargetPriceScale`과 `TargetPriceUnit`을 명시합니다. `WACC <= TerminalGrowth`, 잘못된 기간·주식수 등은 기존 엔진의 유효성 검사를 따릅니다.

Reverse DCF와 Monte Carlo는 `advanced.go`에 구현되어 있습니다. 분포·성장률·할인율의 가정에서 나온 시뮬레이션 범위를 검증된 예측 확률로 표현하지 않습니다. JSON 내보내기는 영업일·종목별 파일과 입력 신뢰도 구분을 유지합니다.

회귀 검증은 [Ginkgo/Gomega 테스트 가이드](TESTING.md)를 따릅니다.
