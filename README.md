# Employee Portal

A web application where an employee views and maintains their own profile: avatar,
contact details, and skills — with an auditable history of every change.

- **Frontend** — React 18, TypeScript, Vite, Tailwind CSS, Vitest
- **Backend** — Go 1.26, chi, PostgreSQL 16, `database/sql`
- **Infrastructure** — Docker Compose (db + api + web behind nginx)

Documentation (ภาษาไทย): [เอกสารข้อกำหนดซอฟต์แวร์](docs/SOFTWARE_SPEC.md) ·
[เอกสารออกแบบระบบ](docs/DESIGN_DOC.md)

---

## Quick start

```bash
docker compose up --build
```

Then open **http://localhost:8080**.

| Account | Email | Password | Role |
|---|---|---|---|
| Employee | `somchai@example.com` | `Password123!` | employee — lands on `/` |
| Admin | `admin@example.com` | `Admin123!` | admin — lands on `/admin` |

> These accounts exist for local development only. They are never created when
> `APP_ENV=production`, regardless of `SEED_DEMO_USERS`.

The API applies its migrations and seeds the skill catalog and demo users on first start. Admins can add accounts from the Users page.

### Running without Docker

```bash
docker compose up -d db

cd backend
DATABASE_URL="postgres://portal:portal@localhost:5432/portal?sslmode=disable" go run ./cmd/server

cd ../frontend
npm install
npm run dev
```

The Vite dev server at http://localhost:5173 proxies `/api` and `/uploads` to the API on
port 8000.

Under `docker compose` the API is **not** published to the host: everything goes through
nginx on :8080. That is deliberate — `TRUST_PROXY_HEADERS` is enabled there, so direct
access would let a client forge `X-Real-IP` and bypass the login rate limit.

---

## Features

| Requirement | Where it lives |
|---|---|
| View own profile | `GET /api/v1/me` → `ProfilePage` |
| Read-only first/last name | Enforced in SQL (`UpdateUser` omits the columns) and rendered as text |
| Avatar upload | `POST /api/v1/me/avatar`, type sniffed from the bytes, 2 MB cap |
| Contact fields edited together, saved once | One form, one Save; the server diffs the submitted values against what it holds and logs only what moved |
| Avatar changed through a modal | Pick, preview, then Save — nothing is uploaded until you confirm |
| Configurable skill catalog | `skills` table (a flat list, no categories), seeded from SQL, managed at `/api/v1/admin/skills` |
| Employees can add their own skill | Search the catalog; if nothing matches, add the typed text as a personal skill that stays off the shared catalog |
| Change history, 7 days by default | `GET /api/v1/me/history`, paged at 5 per page with numbered page controls; the window is set by an admin, not chosen per user |
| Responsive desktop / tablet / mobile | Mobile-first Tailwind: one column, `md:` two columns, `lg:` sidebar + content |
| Basic authentication | bcrypt + JWT, per-IP login rate limit |
| Admin workspace | Left sidebar with Dashboard, Users, Skills, and History retention pages; `/api/v1/admin/*` routes are role-guarded on the server |

## Roles

The two roles have **disjoint** surfaces rather than one being a superset of the other.

| | Employee | Admin |
|---|---|---|
| Route | `/` | `/admin` |
| Own profile, avatar, contact, skills, history | yes | **no** — `/api/v1/me*` returns `403` |
| User accounts: list, add, edit, delete | no | yes |
| Skill catalog: add, rename, deactivate, reactivate | no | yes |
| History retention window | sees it applied | sets it |

An administrator is a configuration operator, not a member of staff: they have no
employee profile at all, and logging in as one returns `"profile": null`. Signing in
sends each role to its own home, and visiting the other role's route redirects rather
than erroring.

### Additional feature

- **Profile completeness meter** — a 0–100 score with a checklist of missing profile details.

---

## Tests

```bash
cd backend && go test ./...
```

```bash
cd frontend && npm test
```

Backend coverage is reported per package with `go test -cover ./internal/...`; the
service, auth, config, validation, storage, and HTTP layers are all above 79%. The HTTP
suite runs against `store.Memory`, an in-memory implementation of the store interface, so
no database is needed.

Frontend: 156 tests across the API client, formatting helpers, `ContactForm`, `AvatarUpload`, `Pagination`, `SkillSelector`, `HistoryList`, `Avatar`,
`CompletenessMeter`, `SkillCatalog` and `RetentionSetting` components, and a routing suite that mounts the whole app and checks
the role guards in both directions.

---

## Layout

```
employee-portal/
├── backend/
│   ├── cmd/server/           entry point and demo seeding
│   ├── internal/
│   │   ├── api/              HTTP handlers, router, middleware
│   │   ├── auth/             bcrypt + JWT
│   │   ├── config/           environment configuration
│   │   ├── models/           domain types
│   │   ├── service/          validation, change detection, history, admin
│   │   ├── storage/          avatar files
│   │   ├── store/            Store interface, Postgres and in-memory implementations
│   │   └── validate/         field rules
│   └── migrations/           numbered SQL, applied at startup
├── frontend/src/
│   ├── api/                  fetch client
│   ├── auth/                 session context
│   ├── components/           UI (employee + admin console, including user CRUD modals)
│   ├── hooks/                server state
│   ├── lib/                  formatting
│   └── pages/                login, profile, admin
├── docs/                     spec and design doc
└── docker-compose.yml
```

---

## Configuration

All backend settings come from the environment; see [.env.example](.env.example) and
section 6 of the design doc. The ones worth knowing:

- `HISTORY_RETENTION_DAYS` (default `7`) — fallback history window used only when the
  database has no readable `history_retention_days` setting. A database setting takes
  precedence; admins can save it via `PUT /api/v1/admin/settings`. The initial migration
  seeds the database setting as `7`, so a fresh database uses that value before env.
  Employees cannot change the window, and the effective value is clamped to 1–365.
- `JWT_SECRET` — the server refuses to start with the built-in default when
  `APP_ENV=production`.
- `TRUST_PROXY_HEADERS` (default `false`) — read the client address from `X-Real-IP` /
  `X-Forwarded-For`. **Required whenever a reverse proxy fronts the API**, otherwise every
  request shares the proxy's address and the login rate limit applies to the whole
  deployment at once. Leave it off when the API is exposed directly, or clients can forge
  the headers. `docker-compose.yml` sets it because nginx fronts the API there.
- `SEED_DEMO_USERS` (default `true`) — the demo accounts below. **Forced off when
  `APP_ENV=production`**, since their passwords are published in this file.

---

## API

The full route table is in section 5 of [the spec](docs/SOFTWARE_SPEC.md).

```bash
TOKEN=$(curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"somchai@example.com","password":"Password123!"}' | jq -r .token)

curl -s localhost:8080/api/v1/me -H "Authorization: Bearer $TOKEN" | jq
curl -s -X PATCH localhost:8080/api/v1/me/contact \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"mobile":"0899999999"}' | jq
curl -s "localhost:8080/api/v1/me/history?page=1&per_page=5" -H "Authorization: Bearer $TOKEN" | jq
```

---

## Known limitations

- The login rate limiter is in-process, so the limit is per API instance.
- No refresh token: a session ends after `JWT_TTL_MINUTES`.
- Avatars live on a local volume; a multi-node deployment needs shared storage behind the
  same `Storage` interface.
- History rows are never pruned — the retention window only controls what is displayed.
- History paging uses OFFSET, which degrades on very deep pages; keyset paging would be
  the next step if the log grows large.
