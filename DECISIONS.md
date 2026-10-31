# sindri — Architecture Decisions

This document records decisions that are **policy**, not implementation. If you are
about to change one of these, understand why it exists first. Each decision notes
the trade-off that was accepted, so future-you can decide whether the trade-off
still applies.

Format: `ADR-NNN — <title>`. Append new decisions; do not rewrite history.

---

## ADR-001 — Single currency per order; no FX

**Decision.** An order has exactly one `currency_code`, inherited from the product
(or set by the invoice creator). The buyer must already hold that currency in their
wallet. No conversion, no cross-currency matching.

**Why.** Cross-currency settlement needs a rate source, a settlement-time pin, a
fee model for FX, and reconciliation. For an MVP, it's out of scope. A buyer who
holds EUR but buys a USD product gets `ErrInsufficientFunds` — that is the intended
behavior, not a bug.

**Trade-off.** Friction for users who hold multiple currencies and want to buy
across them. Acceptable. Revisit if user research shows it's a real blocker.

**Where.** `internal/order/service.go` → `CreateOrder`, `PayInvoice`.

---

## ADR-002 — Escrow timing

**Decision.** Funds are held the moment an order is created (`escrow_hold`).
- Buyer confirms → funds release to seller, minus commission.
- Buyer never confirms → auto-release after `platform_settings.escrow_auto_release_days`
  (default 3 days), executed by the worker.
- Buyer disputes → auto-release is frozen; an admin resolves (release or refund).

**Why.** Buyer protection without a manual admin step for happy paths.

**Trade-off.** Sellers wait up to 3 days if the buyer goes silent. That's the cost
of buyer protection. Auto-release exists to bound it.

**Where.** `internal/order/service.go` → `CreateOrder`, `release`, `Refund`,
`Dispute`. `cmd/worker` runs `FindStaleHeldOrders` on a 5-minute ticker (plus once
on startup).

---

## ADR-003 — Withdrawals require admin review; no direct debit path

**Decision.** The only withdrawal flow is:
1. User hits `POST /withdrawals` → funds move `available → held`, a
   `withdrawal_requests` row is created with `status='pending'`.
2. Admin approves → `held → gone`, ledger row written.
3. Admin rejects → `held → available`, ledger row written.

There is **no** public endpoint to debit a wallet directly. `POST /wallet/withdraw`
was removed intentionally (see git history around the security phase).

**Why.** Direct debit is a money-laundering vector and a fraud vector. Admin review
is the cheapest gate that catches obvious abuse. `wallet.Service.Withdraw` still
exists for internal/admin use, but is not routed.

**Trade-off.** Withdrawals are not instant; they wait for admin. Acceptable for
MVP scale. If you scale to thousands of users, replace manual review with automated
KYC + bank webhook, but keep the state machine.

**Where.** `internal/withdrawal/service.go`. `internal/wallet/service.go` — see the
doc comment on `Withdraw`.

---

## ADR-004 — Auth: JWT in HttpOnly cookie, DB-checked on every request

**Decision.**
- Login returns a JWT; the JWT also goes into an `HttpOnly`, `Secure` (in prod),
  `SameSite=Strict` cookie.
- `RequireAuth` extracts the token from cookie (fallback: `Authorization: Bearer`),
  verifies the signature, then **queries the DB** for `is_active` and `is_admin`.
- The DB result overrides the token's `is_admin` claim.
- TTL is 24h. No refresh token yet.

**Why.**
- Cookie prevents XSS from stealing the token.
- The DB check closes the "disabled user keeps access for 24h" hole. Without it,
  a demoted admin or a disabled user had access for the full TTL. With it, `disable`
  takes effect on the user's very next request.
- DB `is_admin` override prevents "stale admin claim" privilege escalation.

**Trade-off.** One indexed DB query per authenticated request. On pgx + a pool,
~0.2ms. Acceptable. If it becomes a bottleneck, cache user state with a short TTL
(e.g. 30s) — but only if measured.

**Where.** `internal/auth/middleware.go` (`RequireAuth`),
`internal/auth/dbchecker.go` (`DBUserChecker`).

---

## ADR-005 — WebSocket auth: cookie only, no query-string token

**Decision.** WS connections authenticate via the `sindri_token` cookie (or
`Authorization: Bearer` header for non-browser clients). The `?token=` fallback
was removed. Origin allowlist is enforced (`APP_ALLOWED_ORIGINS`).

**Why.**
- Query-string tokens leak via access logs, proxy logs, browser history, and
  Referer headers. Non-starter.
- Combined with cookie auth, an unrestricted origin was a CSWSH (Cross-Site
  WebSocket Hijacking) vector: `evil.example` could open a WS with the victim's
  cookie and read/send messages as them. The allowlist closes this.

**Trade-off.** Non-browser clients that want WS must send an `Authorization:
Bearer` header — standard for tools like `websocat`. Acceptable.

**Where.** `internal/httpapi/chat.go` → `newWSUpgrader`, `chatWSHandler`.
`internal/config/config.go` → `ParseOrigins`. **No default origins in code.**

---

## ADR-006 — Passwords: Argon2id, with bcrypt migration path

**Decision.**
- New registrations hash with Argon2id via `github.com/thomas666-beast/cryptkit/password`
  (PHC format: `$cryptkit$argon2id$v=19$m=65536,t=3,p=4$...`).
- Existing bcrypt hashes (`$2a$`/`$2b$`/`$2y$`) are still verified.
- On successful login, if the stored hash is legacy bcrypt or uses weaker
  params than current defaults, it is rehashed to Argon2id and written back.
- Password byte length capped at 72 during the migration window — mirroring
  bcrypt's silent truncation at 72 bytes. Once all hashes are Argon2id, this cap
  can be removed (Argon2id has no length limit).

**Why.**
- bcrypt at cost 10 is below OWASP 2026 recommendations (cost ≥ 12).
- Argon2id is the current recommendation. `cryptkit` implements it correctly with
  Wycheproof-verified AEADs alongside.
- The 72-byte cap prevents the "two passwords authenticate the same account" gap
  where a >72-byte password is silently truncated.

**Trade-off.**
- Login latency increases to ~200–300ms (Argon2's cost). This is intentional —
  it makes brute-force expensive.
- `cryptkit` is a self-authored, unaudited library. Known risk, accepted for this
  project. Anyone auditing sindri must include cryptkit in scope.

**Where.** `internal/user/service.go` → `Register`, `Authenticate`.

---

## ADR-007 — Chat encryption: server-side AES-256-GCM, not E2EE

**Decision.** Chat messages are encrypted at rest with AES-256-GCM using a key from
`CHAT_ENCRYPTION_KEY`. The server can decrypt any message. This is **not**
end-to-end encryption and is never described as such.

**Why.**
- Moderation and abuse reporting require the server to read messages.
- True E2EE (Signal protocol, or per-conversation key exchange) is a much larger
  design change and incompatible with server-side search or moderation.
- Encryption at rest protects against DB dumps, not against the operator.

**Trade-off.** Users who want E2EE need to use a different tool. Be explicit about
this in user-facing copy. Never imply E2EE.

**Where.** `internal/crypto/aes.go`. `internal/chat/service.go` → `SendMessage`,
`ListMessages`. **Migration to `cryptkit/cipher` is out of scope until we have a
reason and a re-encryption plan.**

---

## ADR-008 — Trust model: single-instance deployment

**Decision.** The current backend assumes **one API process** and **one worker
process** per environment.

**Why.**
- `httprate.LimitByIP` uses in-memory counters. Multi-instance → limits are
  per-instance, not global. Cheaper to accept this than to run Redis.
- The WebSocket hub (`internal/chat.Hub`) is in-memory. Multi-instance → a message
  sent on instance A doesn't reach clients connected to instance B. Would need
  Redis pub/sub or a message bus.
- The auto-release worker isn't distributed. If two workers run, `FindStaleHeldOrders`
  could double-process. `GetOrderForUpdate` provides some safety, but see the note
  in `release()`.

**Trade-off.** Can't horizontally scale today. That's fine for MVP scale (< ~1000
concurrent users). When needed, the migration path is documented in code comments.

**Where.** Everywhere `httprate` or `chat.Hub` is used, plus `cmd/worker`.

---

## ADR-009 — Nothing security- or deployment-relevant is hardcoded

**Decision.** Any value that is a *deployment decision* lives in config
(`.env`, environment variables) or in the database (`platform_settings`). Not in
code.

Non-negotiable env vars (fails to boot if missing):
- `DATABASE_URL`
- `APP_ALLOWED_ORIGINS` — required even in dev. No hidden defaults.
- `JWT_SECRET` — required in production; dev fallback is logged as a warning.
- `CHAT_ENCRYPTION_KEY` — required in production; dev fallback is deterministic.

Values in `platform_settings`:
- `commission_percent`
- `escrow_auto_release_days`
- `kyc_required_for_deposit`
- `kyc_required_for_withdraw` — reserved; not enforced yet (see ADR-016)

**Why.** If changing a value is a *decision*, it goes in config. If changing it is
a *bug*, it goes in code. A JWT secret is a decision. A status enum (`held`,
`disputed`) is a bug if you change it.

**Where.** `internal/config/config.go`. `internal/settings/service.go`. The
`platform_settings` migration in `migrations/001_init.sql`.

**Exception.** Test fixtures (e.g. `internal/httpapi/testhelpers_test.go`) are
allowed to hardcode values, because they're not read by production code.

---

## ADR-010 — Money is `BIGINT` in minor units, everywhere

**Decision.** All monetary amounts in the codebase and database are `BIGINT`
integers in the currency's minor units (cents for USD, satoshis for crypto,
whole yen for JPY). The `currencies.decimal_digits` column determines the
scaling. Never store floats; never store the currency's major unit.

**Why.** Floating-point money is a known bug class. Integers with a known scale
are exact and easy to audit.

**Trade-off.** Every rendering path must call `fmtAmount(currencies, cents, code)`.
Every input path must call `parseAmount(str, digitsFor(...))`. Both are in
`web/src/lib/money.ts` on the frontend. On the backend, `BigInt` types are
serialized as JSON numbers — JavaScript's `Number` can safely represent values
up to 2^53, which at 2 decimal places is ~$90 trillion. Sufficient for this app.

**Where.** Everywhere. Never introduce a `float64` field for money.

---

## ADR-011 — API versioning: `/api/v1/`

**Decision.** All API routes live under `/api/v1/`. Breaking changes bump to
`/api/v2/`. Additive changes stay in v1.

**Why.** Allows the frontend and mobile clients to evolve independently of
backend changes. Bumping is cheap; retrofitting versioning is not.

**Trade-off.** None today. It's just a directory.

**Where.** `internal/httpapi/router.go`.

---

## ADR-012 — Search: `pg_trgm` + `ILIKE`, not full-text

**Decision.** Product and user search uses PostgreSQL `pg_trgm` GIN indexes with
`ILIKE '%q%'` queries. No `tsvector` / `websearch_to_tsquery` yet.

**Why.** `pg_trgm` handles substring matches (what users expect from a marketplace
search bar) with an index that works out of the box. Full-text search requires
language-specific stemming and tokenization — overkill until we have enough
content to need relevance ranking.

**Trade-off.** `pg_trgm` GIN indexes take significant disk space (10–20% of the
indexed column size). Acceptable at MVP scale. When search volume justifies it,
migrate to `tsvector` with `websearch_to_tsquery` for ranking + `pg_trgm` for
fuzzy fallback.

**Where.** `migrations/001_init.sql` → `*_trgm_idx` indexes.
`db/queries/product_search.sql`, `db/queries/user_search.sql`.

---

## ADR-013 — No end-to-end tests, only integration tests

**Decision.** We do not write browser-based E2E tests. We write **integration
tests** that use `httptest.Server` against a real database. Unit tests for pure
logic.

**Why.** E2E tests are brittle, slow, and expensive to maintain. Integration tests
that hit the real HTTP handler + real DB catch 90% of the same bugs at 10% of the
cost. If we need browser testing later, add Playwright for the specific flows
(payment, chat) that integration tests can't reach.

**Trade-off.** Some frontend-only bugs (Vue reactivity, CSS, routing) go untested.
Acceptable for now.

**Where.** `internal/httpapi/*_test.go` — use `newTestRouter(t)`.

---

## ADR-014 — Test hygiene: DB cleanup before any assert

**Decision.** In every DB-backed test, register `t.Cleanup` **immediately after**
a created resource's ID is available — before any `t.Fatal`-able assertion.
Otherwise a failing early assertion orphans DB rows, which pollutes subsequent
runs.

**Why.** Discovered during the security phase: a 30% failure rate in the suite
left the test DB full of orphaned users. Once cleanup ordering was fixed, `make
test` became reliably repeatable.

**Trade-off.** Slightly more verbose tests. Worth it.

**Where.** Every `*_test.go` in `internal/httpapi`, `internal/order`,
`internal/wallet`, `internal/review`, `internal/withdrawal`, `internal/chat`,
`internal/admin`, `internal/user`, `internal/category`, `internal/product`.

---

## ADR-015 — Frontend: no hardcoded deployment values

**Status.** Enforced.

**Decision.** Same rule as ADR-009, applied to the Vue app:
- API base URL and WS URL come from runtime state (window.location) or from
  `import.meta.env.VITE_*`. Never hardcoded literals like `http://localhost:8080`.
- The Vite dev proxy target comes from `VITE_API_URL` (default `http://localhost:8080`),
  not a magic string.
- The brand name comes from `VITE_APP_BRAND_NAME`, read via `useBrand()` — not
  from i18n, not from any hardcoded string (see ADR-016).
- Any value that becomes a "deployment decision" must be auditable with
  `grep -rn "http://\|https://\|localhost\|127.0.0.1" web/src/`.

**Why.** Same principle as ADR-009. Frontend code is easy to ship with hardcoded
hosts, and they break in production in non-obvious ways (CORS, cookies, WS).

**Trade-off.** Slightly more setup for fresh clones (an `.env` file). Acceptable.

**Where.** `vite.config.ts` (`VITE_API_URL`), `web/src/config/app.ts`,
`web/src/composables/useBrand.ts` (`VITE_APP_BRAND_NAME`), `web/src/api/*.ts`,
`web/src/lib/*.ts`.

---

## ADR-016 — KYC integration is incomplete; not enforced as a gate yet

**Decision.** KYC is **not** fully integrated. The data model (`users.kyc_status`,
`users.kyc_override`) and the admin override path (`POST /admin/users/{id}/kyc`)
exist, and `kyc_required_for_deposit` is enforced in `wallet.depositHandler`.
However, there is **no** user-initiated submission flow, **no** document storage,
and **no** review pipeline.

`platform_settings.kyc_required_for_withdraw` is reserved but **not** read by any
code path today. Do not assume it is enforced.

**Why.** Building the full KYC pipeline (document upload, review queue, audit log,
vendor integration) is a project on its own. The current partial state was shipped
deliberately so the admin override exists when needed, without pretending the flow
is complete.

**Trade-off.** A user can hold an account with `kyc_status = 'not_required'`
indefinitely. If `kyc_required_for_deposit` is turned on, deposits require a
`verified` status — but the user has no way to *become* verified without an admin.
That's acceptable while the setting is off (dev) and should be turned on only after
the submission flow lands.

**Where.** `internal/admin/service.go` → `SetKYC`. `internal/wallet/...` (deposit
handler). `migrations/001_init.sql` → `users.kyc_*` columns.

**Out of scope for now.** Document upload, review queue, KYC vendor integration,
audit trail. Backlog entry: "KYC flow — user submission + admin review + audit log".

---

## ADR-017 — Brand name and mark are runtime config, not code or i18n

**Decision.**
- The brand name (currently `sindri`) lives in `VITE_APP_BRAND_NAME` and is read
  via `useBrand()`. It is **not** in translation files, and **not** hardcoded in
  components.
- The brand mark ships as two SVG files:
  - `web/public/logo.svg` — transparent, uses `currentColor`, inlined in
    components so it inherits `--accent` and hover state from CSS.
  - `web/public/logo-badge.svg` — same glyph with a rounded background, used
    only for the favicon / PWA manifest / social previews (browsers don't apply
    CSS there).
- The accent color lives in CSS variables (`--accent`, `--accent-hover`).
  Changing those recolors the inline mark automatically.

**Why.**
- The brand is a **proper noun** — it does not localize. Storing it under
  `auth.title` (the earlier mistake) was semantically wrong and made rebranding
  require touching every locale file.
- Rebranding is a deployment decision, not a translation edit. One env var
  change should be sufficient.
- The mark must survive a rename without redesign. The `>_` glyph reads as
  "shell / coding / terminal" and is name-agnostic, so a rename to a
  coding/free-software name (`Sindri`, `pragma`, `stdlib`) keeps the mark
  intact.

**Trade-off.** The brand is not editable via a CMS. Acceptable — rebrands happen
at most once a year.

**Where.** `web/src/config/app.ts`, `web/src/composables/useBrand.ts`,
`web/.env.example`, `web/public/logo.svg`, `web/public/logo-badge.svg`.
`BRAND.md` at project root documents the contract.

---

## Changelog

- **2026-10-29** — Initial version. ADR-001 through ADR-015 captured after the
  security-hardening phase (A3, B1–B4, Argon2 migration, C1–C4).
- **2026-10-31** — Corrections and additions.
  - ADR-002: corrected worker ticker description (5 min, not 60s).
  - ADR-009: noted `kyc_required_for_withdraw` as reserved / not enforced.
  - ADR-015: status changed from "to be enforced" to "Enforced"; added
    `VITE_APP_BRAND_NAME` reference.
  - Added ADR-016 (KYC integration incomplete).
  - Added ADR-017 (brand name and mark are runtime config).
