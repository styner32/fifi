package localdev

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func environmentMap(environment []string) map[string]string {
	GinkgoHelper()
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		Expect(ok).To(BeTrue())
		values[key] = value
	}
	return values
}

var _ = Describe("Local command environment", func() {
	Context("when process, dotenv, and defaults overlap", func() {
		It("preserves process values including empty strings and does not mutate its inputs", func() {
			dotenv := map[string]string{"DATABASE_URL": "dotenv-app", "REDIS_URL": "dotenv-redis", "TOKEN": "file-token", "ONLY_FILE": "a=b"}
			process := []string{"DATABASE_URL=process-app", "REDIS_URL=", "TOKEN=process-token"}
			environment, err := ExportEnvironment(process, dotenv, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(SatisfyAll(
				HaveKeyWithValue("DATABASE_URL", "process-app"),
				HaveKeyWithValue("REDIS_URL", ""),
				HaveKeyWithValue("TOKEN", "process-token"),
				HaveKeyWithValue("ONLY_FILE", "a=b"),
			))
			Expect(dotenv["DATABASE_URL"]).To(Equal("dotenv-app"))
			Expect(process).To(Equal([]string{"DATABASE_URL=process-app", "REDIS_URL=", "TOKEN=process-token"}))
		})

		It("uses dotenv values including empty values before applying missing defaults", func() {
			environment, err := ExportEnvironment(nil, map[string]string{"DATABASE_URL": "", "REDIS_URL": "redis://file/2"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(SatisfyAll(
				HaveKeyWithValue("DATABASE_URL", ""),
				HaveKeyWithValue("REDIS_URL", "redis://file/2"),
				HaveKeyWithValue("TEST_DATABASE_URL", "postgresql://sunjinlee@localhost:5433/dart_test?sslmode=disable"),
				HaveKeyWithValue("TEST_REDIS_URL", "redis://127.0.0.1:6379/15"),
			))
		})
	})

	Context("when running tests", func() {
		It("routes app variables to explicit test variables using normal precedence", func() {
			environment, err := ExportEnvironment([]string{"DATABASE_URL=postgresql://app/live", "REDIS_URL=redis://app/0", "TEST_DATABASE_URL=postgresql://test/suite_test", "TEST_REDIS_URL=redis://test/14"}, map[string]string{"TEST_DATABASE_URL": "postgresql://file/file_test", "TEST_REDIS_URL": "redis://file/12"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(SatisfyAll(
				HaveKeyWithValue("DATABASE_URL", "postgresql://test/suite_test"),
				HaveKeyWithValue("REDIS_URL", "redis://test/14"),
			))
		})

		It("uses independent local test defaults instead of deriving them from app URLs", func() {
			environment, err := ExportEnvironment([]string{"DATABASE_URL=postgresql://app/private", "REDIS_URL=redis://app/0"}, nil, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(SatisfyAll(
				HaveKeyWithValue("DATABASE_URL", "postgresql://sunjinlee@localhost:5433/dart_test?sslmode=disable"),
				HaveKeyWithValue("REDIS_URL", "redis://127.0.0.1:6379/15"),
			))
		})

		It("does not replace an explicitly empty test database with any fallback", func() {
			_, err := ExportEnvironment([]string{"TEST_DATABASE_URL="}, map[string]string{"TEST_DATABASE_URL": "postgresql://file/file_test"}, true)
			Expect(err).To(HaveOccurred())
		})

		It("preserves an explicitly empty test Redis value", func() {
			environment, err := ExportEnvironment([]string{"TEST_REDIS_URL="}, map[string]string{"TEST_REDIS_URL": "redis://file/12"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(HaveKeyWithValue("REDIS_URL", ""))
		})

		DescribeTable("rejects database URLs that cannot safely identify the test database", func(databaseURL string) {
			_, err := ExportEnvironment([]string{"TEST_DATABASE_URL=" + databaseURL}, nil, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).NotTo(ContainSubstring("private-secret"))
		},
			Entry("a missing database", "postgresql://localhost/"),
			Entry("an app database", "postgresql://user:private-secret@localhost/app"),
			Entry("another protocol", "mysql://localhost/suite_test"),
			Entry("a keyword DSN", "host=localhost dbname=suite_test"),
			Entry("a query dbname override", "postgresql://localhost/suite_test?dbname=app"),
			Entry("a query database override", "postgresql://localhost/suite_test?database=app"),
			Entry("an invalid query", "postgresql://localhost/suite_test?sslmode=%ZZ"),
			Entry("a nested path", "postgresql://localhost/app/suite_test"),
		)

		DescribeTable("accepts PostgreSQL test database URLs", func(databaseURL string) {
			environment, err := ExportEnvironment([]string{"TEST_DATABASE_URL=" + databaseURL}, nil, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(HaveKeyWithValue("DATABASE_URL", databaseURL))
		},
			Entry("postgresql URL", "postgresql://localhost/suite_test?sslmode=disable"),
			Entry("postgres URL", "postgres://localhost/suite_test"),
			Entry("Unix socket URL", "postgresql:///suite_test?host=/tmp"),
		)
	})

	Context("when reading dotenv", func() {
		var filename string
		BeforeEach(func() { filename = filepath.Join(GinkgoT().TempDir(), ".env") })

		It("allows an absent dotenv file and supplies local defaults", func() {
			environment, err := LoadEnvironment(nil, filename, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(HaveKeyWithValue("DATABASE_URL", "postgresql://sunjinlee@localhost:5433/dart?sslmode=disable"))
		})

		It("reads quoted values without executing shell syntax", func() {
			Expect(os.WriteFile(filename, []byte("TOKEN='literal spaces=$(not-a-command)'\n"), 0600)).To(Succeed())
			environment, err := LoadEnvironment(nil, filename, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(environmentMap(environment)).To(HaveKeyWithValue("TOKEN", "literal spaces=$(not-a-command)"))
		})

		It("rejects malformed dotenv without including its contents", func() {
			Expect(os.WriteFile(filename, []byte("TOKEN='private-secret\n"), 0600)).To(Succeed())
			_, err := LoadEnvironment([]string{"TOKEN=process-value"}, filename, false)
			Expect(err).To(MatchError("cannot read or parse .env"))
			Expect(err.Error()).NotTo(ContainSubstring("private-secret"))
		})
	})
})
