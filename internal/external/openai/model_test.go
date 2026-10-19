package openai_test

import (
	"os"

	"github.com/fifi/internal/external/openai"
	"github.com/openai/openai-go/v3/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenAI 모델 결정", func() {
	BeforeEach(func() {
		prev, had := os.LookupEnv(openai.ModelEnvKey)
		DeferCleanup(func() {
			if had {
				os.Setenv(openai.ModelEnvKey, prev)
			} else {
				os.Unsetenv(openai.ModelEnvKey)
			}
		})
		os.Unsetenv(openai.ModelEnvKey)
	})

	It("기본 모델은 gpt-6-luna다", func() {
		Expect(openai.DefaultModel).To(Equal(shared.ResponsesModel("gpt-6-luna")))
		Expect(openai.ResolveModel()).To(Equal(openai.DefaultModel))
	})

	Context("OPENAI_MODEL이 설정되면", func() {
		BeforeEach(func() { os.Setenv(openai.ModelEnvKey, " gpt-custom ") })

		It("기본 모델 대신 환경변수 값을 사용한다", func() {
			Expect(openai.ResolveModel()).To(Equal(shared.ResponsesModel("gpt-custom")))
		})

		It("빈 명시값은 무시하고 환경변수 값을 사용한다", func() {
			Expect(openai.ResolveModel("", "  ")).To(Equal(shared.ResponsesModel("gpt-custom")))
		})

		It("명시값이 있으면 환경변수보다 우선한다", func() {
			Expect(openai.ResolveModel("gpt-explicit")).To(Equal(shared.ResponsesModel("gpt-explicit")))
		})
	})

	It("FileAnalyzer 생성 시 같은 규칙을 적용한다", func() {
		Expect(openai.NewFileAnalyzer("dummy").GetModel()).To(Equal(openai.DefaultModel))
		Expect(openai.NewFileAnalyzer("dummy", "").GetModel()).To(Equal(openai.DefaultModel))
		Expect(openai.NewFileAnalyzer("dummy", "gpt-explicit").GetModel()).To(Equal(shared.ResponsesModel("gpt-explicit")))
	})
})

