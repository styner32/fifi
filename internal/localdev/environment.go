package localdev

import (
	"errors"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

var localDefaults = map[string]string{
	"DATABASE_URL":      "postgresql://sunjinlee@localhost:5433/dart?sslmode=disable",
	"TEST_DATABASE_URL": "postgresql://sunjinlee@localhost:5433/dart_test?sslmode=disable",
	"REDIS_URL":         "redis://127.0.0.1:6379/0",
	"TEST_REDIS_URL":    "redis://127.0.0.1:6379/15",
}

// LoadEnvironment reads dotenv without changing the current process environment.
// Parser errors are redacted because godotenv errors can contain input values.
func LoadEnvironment(process []string, filename string, test bool) ([]string, error) {
	dotenv, err := godotenv.Read(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.New("cannot read or parse .env")
	}
	return ExportEnvironment(process, dotenv, test)
}

// ExportEnvironment merges process > dotenv > local defaults, including explicit
// empty values. Test mode routes only through the independently configured test URLs.
func ExportEnvironment(process []string, dotenv map[string]string, test bool) ([]string, error) {
	merged := make(map[string]string, len(dotenv)+len(process)+len(localDefaults))
	for key, value := range dotenv {
		merged[key] = value
	}
	for _, entry := range process {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, errors.New("invalid process environment entry")
		}
		merged[key] = value
	}
	for key, value := range localDefaults {
		if _, exists := merged[key]; !exists {
			merged[key] = value
		}
	}
	if test {
		if !validTestDatabaseURL(merged["TEST_DATABASE_URL"]) {
			return nil, errors.New("TEST_DATABASE_URL must name a nonempty PostgreSQL *_test database in the URL path without database query overrides")
		}
		merged["DATABASE_URL"] = merged["TEST_DATABASE_URL"]
		merged["REDIS_URL"] = merged["TEST_REDIS_URL"]
	}
	keys := make([]string, 0, len(merged))
	for key, value := range merged {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return nil, errors.New("invalid environment entry")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+merged[key])
	}
	return environment, nil
}

func validTestDatabaseURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Opaque != "" {
		return false
	}
	database := strings.TrimPrefix(u.Path, "/")
	if database == "" || !strings.HasSuffix(database, "_test") || strings.ContainsAny(database, "/\x00") {
		return false
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	_, dbnameOverride := query["dbname"]
	_, databaseOverride := query["database"]
	return !dbnameOverride && !databaseOverride
}
