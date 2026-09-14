## 2025-02-28 - [DoS Prevention] Add timeouts to HTTP Server in Go
**Vulnerability:** The default `http.Server` configured implicitly via `gin.Engine.Run()` has no `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`. This allows malicious clients to perform Slowloris Denial of Service (DoS) attacks by sending requests very slowly and exhausting server resources.

**Learning:** It is a common misconfiguration in Go HTTP applications to use default server configurations. This leaves the application highly vulnerable to resource exhaustion.

**Prevention:** Always instantiate a custom `&http.Server{}` and configure `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` explicitly before calling `ListenAndServe`.

## 2025-02-28 - [High] SQL Wildcard Injection via ILIKE Query Parameters
**Vulnerability:** Found unescaped user inputs directly interpolated into `%` surrounded strings for `ILIKE` clauses in the database layer. Even when using an ORM like GORM, standard parameter bindings do not escape `%` or `_` inside LIKE patterns.
**Learning:** Supplying patterns such as `%_%_%` via user input triggers expensive full-table scans with heavily fragmented results, causing a potential Denial of Service (DoS) risk.
**Prevention:** Always escape `\`, `%`, and `_` with a backward slash when using them as user input embedded into `LIKE`/`ILIKE` queries.
