## 2024-03-24 - [Wildcard Injection in GORM ILIKE Queries]
**Vulnerability:** User input used in GORM `ILIKE` clauses was not escaped for wildcard characters (`%`, `_`) and the escape character (`\`), allowing users to perform wildcard injections.
**Learning:** GORM parameterized queries (`?`) only protect against SQL injection (executing arbitrary SQL), but they do not automatically escape wildcard characters within `LIKE`/`ILIKE` clauses. An attacker can craft queries with many `%` characters to cause a Denial of Service (DoS) due to expensive pattern matching.
**Prevention:** Always manually escape user input before embedding it in a `LIKE` or `ILIKE` clause string, especially for `%`, `_`, and `\`. E.g., `strings.ReplaceAll(input, "%", "\\%")`.
