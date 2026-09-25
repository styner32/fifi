## 2025-02-28 - [DoS Prevention] Add timeouts to HTTP Server in Go
**Vulnerability:** The default `http.Server` configured implicitly via `gin.Engine.Run()` has no `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`. This allows malicious clients to perform Slowloris Denial of Service (DoS) attacks by sending requests very slowly and exhausting server resources.
**Learning:** It is a common misconfiguration in Go HTTP applications to use default server configurations. This leaves the application highly vulnerable to resource exhaustion.
**Prevention:** Always instantiate a custom `&http.Server{}` and configure `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` explicitly before calling `ListenAndServe`.

## 2025-03-09 - [Wildcard Injection in ILIKE Clauses]
**Vulnerability:** Unescaped user inputs were being directly used inside ILIKE clauses in PostgreSQL queries (e.g., `ILIKE "%" + search + "%"`). This allows an attacker to inject wildcards (`%`, `_`), which can cause a Denial of Service (DoS) by forcing the database to perform full table scans instead of utilizing indexes.
**Learning:** The issue existed because input was blindly concatenated without escaping specific SQL wildcard characters. While GORM parameterizes queries to prevent SQL injection, it does not automatically escape wildcards within LIKE/ILIKE patterns.
**Prevention:** Always sanitize/escape wildcard characters (`\`, `%`, `_`) in user-provided input before incorporating it into a LIKE/ILIKE clause.

## 2026-09-25 - [Insecure File Permissions & Octal Syntax]
**Vulnerability:** Permissive file and directory permissions (`0644`, `0755`) were being used, increasing the risk of unauthorized access or data exposure, especially in sensitive contexts (auth tokens and data storage). Additionally, older syntax (`0644`) for octals can lead to unintentional decimal fallback and parsing confusion in newer Go versions.
**Learning:** Hardcoded secrets and overly permissive paths can introduce security risks, particularly when permissions allow reading/writing by arbitrary users. Older syntax like `0644` vs `0o644` often creates inconsistencies.
**Prevention:** Always use restrictive file creation permissions (`0o600` for files and `0o700` or `0o750` for directories). Additionally, use the modern Go `0o` prefix for octal literals instead of the older `0` prefix syntax to ensure strict interpretation as octal permissions.
