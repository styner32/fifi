# Go 테스트 작성 및 실행

신규·수정하는 Go 단위·통합·회귀 테스트는 **Ginkgo v2 + Gomega의 BDD 스타일**로 작성한다. 이 규칙은 CLI, 수집기, 외부 API 클라이언트 등 저장소의 Go 코드에 적용한다.

## 작성 규칙

- `Describe`에는 검증 대상, `Context`에는 주어진 조건, `It`에는 관찰 가능한 기대 동작을 적는다. 함수명만 반복하지 말고 실패한 조건과 동작이 테스트 이름에 드러나게 한다.
- 반복 입력은 `DescribeTable` / `Entry`로 나눈다. 정상 입력과 오류 입력은 각각 독립적으로 실행하고 식별할 수 있어야 한다.
- 단언은 `Expect(...).To(...)` 등 Gomega matcher를 사용한다. 오차가 허용되는 수치는 `BeNumerically`, 포인터 값은 `HaveValue`, 결측은 `BeNil`로 명시한다.
- 각 사례의 상태는 `BeforeEach`에서 준비한다. 임시 파일은 `GinkgoT().TempDir()`을 사용하고, 검증 helper에는 `GinkgoHelper()`를 호출해 실패 위치를 명확히 한다.
- 기존 패키지의 `*_suite_test.go`를 재사용한다. 새 패키지에만 스위트 진입점을 추가한다.
- 독립적인 `func TestXxx`, `t.Run`, `t.Fatal` 기반 동작 테스트를 새로 작성하지 않는다. 표준 `testing`의 `TestXxx(t *testing.T)`는 아래 Ginkgo 스위트 진입점에 사용한다.
- 기존 테스트를 수정할 때도 이 스타일로 맞추되, 검증 범위를 축소하거나 실패한 검증을 삭제하지 않는다. 작업과 무관한 기존 테스트까지 일괄 변환할 필요는 없다.

## 스위트 진입점

예: `snapshot_suite_test.go`. 기존 파일이 있으면 새로 만들지 않는다.

```go
package snapshot

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestSnapshot(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Snapshot Suite")
}
```

## BDD 사례

동작 테스트는 별도 `*_test.go`에서 스위트에 등록한다. 다음은 예탁금 증감 검증의 작성 예시다.

```go
var _ = Describe("예탁금 증감 검증", func() {
    Context("현재 잔고와 이전 잔고가 제공되면", func() {
        DescribeTable("보고된 증감을 잔고 차이와 대조한다",
            func(reportedDelta float64, expected QualityStatus) {
                status, _ := ValidateCredit(110, 100, reportedDelta)
                Expect(status).To(Equal(expected))
            },
            Entry("잔고 차이와 일치", 10.0, StatusValid),
            Entry("허용 오차를 초과", 5.0, StatusArithmeticMismatch),
        )
    })

    Context("현재 잔고가 누락되면", func() {
        It("정합성 검증을 완료한 것으로 처리하지 않는다", func() {
            status, _ := ValidateCredit(0, 100, -100)
            Expect(status).To(Equal(StatusInsufficientData))
        })
    })
})
```

실제 사례: [Snapshot 회귀 테스트](../internal/collector/snapshot/integrity_test.go), [KOFIA 단위·캐시 테스트](../internal/external/kofia/unit_regression_test.go).

## 외부 데이터와 재현성

- 단위·회귀 테스트는 고정 응답, 주입한 시계, fake 의존성으로 재현한다. HTTP 응답은 기존 `MockTransport` 또는 `RoundTripper`를 사용해 실제 API 호출 없이 검증한다.
- 실제 API 재조회는 별도 검증으로 구분한다. 관측 시각에 따라 달라지는 값을 고정 회귀 테스트의 정답으로 사용하지 않는다.
- 오류·결측·실제 0, 날짜·시각·단위·모집단 불일치는 독립적인 사례로 검증한다. 출력이 정상처럼 보이는지만 확인하지 말고 JSON의 상태와 결측값도 확인한다.

## 실행

저장소 루트에서 로컬 의존성과 격리된 테스트 DB를 준비한다. `test-db-prepare`는 로컬 Compose PostgreSQL에 `_test` DB를 생성하고 migration을 적용한다. 기존 로컬 앱 DB와 저장 보고서는 유지한다.

```bash
make setup
make services-up
make test-db-prepare
make test
make test-race
make check
```

`make test`와 `make test-integration`은 같은 격리 환경에서 전체 Go 스위트를 실행한다. `make check`에는 문서 링크, 웹 build와 lint가 포함된다. `make test-race`도 프로젝트 Go 스위트 전체를 검사한다. 범위를 줄일 때는 `TEST_PACKAGES='./internal/localdev'`처럼 지정한다. 별도 Ginkgo CLI 설치는 필요하지 않다.

외부 테스트 DB를 사용하면 `TEST_DATABASE_URL`과 `TEST_REDIS_URL`을 명시한다. DB 이름은 `_test`로 끝나야 하며 전체 migration이 적용돼 있어야 한다. Make의 일반 설정은 명시한 환경변수·Make 할당 → `.env` → 로컬 기본값 순서이며, 테스트 설정은 앱의 `DATABASE_URL`·`REDIS_URL`과 분리한다.

수정한 패키지의 고정 fixture 테스트는 직접 실행할 수도 있다.

```bash
# 변경한 패키지의 스위트 실행
go test ./internal/collector/snapshot ./internal/external/kofia

# race 검사와 함께 캐시 없이 실행
go test -race -count=1 ./internal/collector/snapshot ./internal/external/kofia

# Ginkgo의 사례별 실행 결과 보기
go test -v ./internal/collector/snapshot -ginkgo.v

# 격리 설정·DB 검사를 포함한 전체 테스트
make test
```

변경 범위에 맞는 스위트를 실행하고 결과를 확인한다. 외부 서비스가 필요한 통합 테스트는 해당 의존성을 준비한 뒤 실행하며, 실제로 실행하지 않은 검증을 완료했다고 기록하지 않는다.

전체 DB suite는 같은 fixture 테이블을 초기화하므로 `-p 1`로 패키지를 순차 실행한다. 직접 `go test`로 DB suite를 실행할 때도 동일한 격리 환경을 전달해야 한다. 프로세스 환경변수는 `.env.test`보다 우선한다. 로컬 서비스와 명령 목록은 [로컬 개발 가이드](MAINTENANCE.md)를 참고한다.
