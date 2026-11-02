## 2025-02-28 - [DoS Prevention] Add timeouts to HTTP Server in Go
**Vulnerability:** The default `http.Server` configured implicitly via `gin.Engine.Run()` has no `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`. This allows malicious clients to perform Slowloris Denial of Service (DoS) attacks by sending requests very slowly and exhausting server resources.

**Learning:** It is a common misconfiguration in Go HTTP applications to use default server configurations. This leaves the application highly vulnerable to resource exhaustion.

**Prevention:** Always instantiate a custom `&http.Server{}` and configure `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` explicitly before calling `ListenAndServe`.

## 2025-03-09 - [Wildcard Injection in ILIKE Clauses]
**Vulnerability:** Unescaped user inputs were being directly used inside ILIKE clauses in PostgreSQL queries (e.g., `ILIKE "%" + search + "%"`). This allows an attacker to inject wildcards (`%`, `_`), which can cause a Denial of Service (DoS) by forcing the database to perform full table scans instead of utilizing indexes.
**Learning:** The issue existed because input was blindly concatenated without escaping specific SQL wildcard characters. While GORM parameterizes queries to prevent SQL injection, it does not automatically escape wildcards within LIKE/ILIKE patterns.
**Prevention:** Always sanitize/escape wildcard characters (`\`, `%`, `_`) in user-provided input before incorporating it into a LIKE/ILIKE clause.

## 2025-03-09 - [Insecure Permissions] Fix Insecure File and Directory Permissions
**Vulnerability:** Files and directories were being created with overly permissive permissions (e.g., 0755 for directories and 0644 for files), which could expose sensitive data like token caches or downloaded reports to unauthorized users on the system.
**Learning:** It is a common oversight to use default or common permissions without considering the sensitivity of the data being stored.
**Prevention:** Always use strict permissions (e.g., 0o600 for files, 0o700/0o750 for directories) when creating files or directories that store sensitive information, as recommended by gosec.

## 2025-03-09 - [Insecure TLS Configuration] Removing InsecureSkipVerify: true
**Vulnerability:** The codebase was using `InsecureSkipVerify: true` in `tls.Config` for an HTTP client in `internal/external/kofia/client.go` and `cmd/kofia_debug/main.go`. This essentially disables TLS certificate verification, making the application vulnerable to Man-In-The-Middle (MITM) attacks.
**Learning:** This is often done as a quick workaround when dealing with self-signed certificates or certificates issued by authorities not in the system's default trust store. However, it compromises the security of the TLS connection entirely.
**Prevention:** Instead of bypassing verification completely with `InsecureSkipVerify: true`, obtain the required certificate, embed it securely within the application (e.g., using `go:embed`), and explicitly add it to a custom `RootCAs` pool (initialized with `x509.SystemCertPool()`) in the `tls.Config`.
