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

## 2025-03-09 - [Insecure TLS] Replace InsecureSkipVerify with Embedded Certificates
**Vulnerability:** External API clients (like KOFIA FreeSIS) were configured with `InsecureSkipVerify: true` in their `tls.Config`, allowing connections to potentially malicious servers by bypassing certificate validation (gosec G402).
**Learning:** This is often done as a quick fix when the server uses a certificate that is not trusted by the default system pool. However, removing the flag blindly leads to TLS handshake failures.
**Prevention:** Instead of skipping verification, download the required certificate (e.g., via openssl), embed it using `//go:embed`, and explicitly load it into a custom `x509.CertPool` assigned to `RootCAs` in the `tls.Config`.
