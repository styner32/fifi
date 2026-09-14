<<<<<<< HEAD
## 2025-02-28 - [DoS Prevention] Add timeouts to HTTP Server in Go
**Vulnerability:** The default `http.Server` configured implicitly via `gin.Engine.Run()` has no `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`. This allows malicious clients to perform Slowloris Denial of Service (DoS) attacks by sending requests very slowly and exhausting server resources.

**Learning:** It is a common misconfiguration in Go HTTP applications to use default server configurations. This leaves the application highly vulnerable to resource exhaustion.

**Prevention:** Always instantiate a custom `&http.Server{}` and configure `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` explicitly before calling `ListenAndServe`.

## 2025-02-28 - [High] SQL Wildcard Injection via ILIKE Query Parameters
**Vulnerability:** Found unescaped user inputs directly interpolated into `%` surrounded strings for `ILIKE` clauses in the database layer. Even when using an ORM like GORM, standard parameter bindings do not escape `%` or `_` inside LIKE patterns.
**Learning:** Supplying patterns such as `%_%_%` via user input triggers expensive full-table scans with heavily fragmented results, causing a potential Denial of Service (DoS) risk.
**Prevention:** Always escape `\`, `%`, and `_` with a backward slash when using them as user input embedded into `LIKE`/`ILIKE` queries.
=======
## 2025-03-09 - [Wildcard Injection in ILIKE Clauses]
**Vulnerability:** Unescaped user inputs were being directly used inside ILIKE clauses in PostgreSQL queries (e.g., `ILIKE "%" + search + "%"`). This allows an attacker to inject wildcards (`%`, `_`), which can cause a Denial of Service (DoS) by forcing the database to perform full table scans instead of utilizing indexes.
**Learning:** The issue existed because input was blindly concatenated without escaping specific SQL wildcard characters. While GORM parameterizes queries to prevent SQL injection, it does not automatically escape wildcards within LIKE/ILIKE patterns.
**Prevention:** Always sanitize/escape wildcard characters (`\`, `%`, `_`) in user-provided input before incorporating it into a LIKE/ILIKE clause.
>>>>>>> 1901c84 (🛡️ Sentinel: [MEDIUM] Fix Wildcard Injection in ILIKE queries)
