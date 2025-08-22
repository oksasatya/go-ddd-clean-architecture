# Go DDD Boilerplate (Gin, Postgres, Redis, GCS, JWT)

Production-ready Go boilerplate with DDD/Clean Architecture and pragmatic defaults.

Stack
- Gin HTTP framework
- PostgreSQL (pgx) + golang-migrate (runs at startup)
- Redis (go-redis) for sessions/caching + rate limiting
- Google Cloud Storage (optional)
- JWT (access/refresh cookies)
- Logrus, CORS, Makefile helpers

API base path: /api

Architecture & folders
```
cmd/
  main.go                 # app entrypoint, DI, migrations, graceful shutdown
  seed/main.go            # simple seeder (demo user)
config/
  config.go               # env config, DSN helpers, CORS origins
internal/
  domain/
    entity/
      user_entity.go
    repository/
      user_repository.go
  application/
    user_service.go       # business logic/use-cases
  infrastructure/
    postgres/
      pool.go
      user_repository_impl.go
  interface/
    http/
      user_handler.go
    middleware/
      jwt.go
      rate_limit.go
      request_id.go
  router/
    init.go               # wire modules with container singletons
    module.go             # Registry module interface
    registry.go           # groups /api and mounts modules
    modules/
      user/
        module.go         # public + protected routes (JWT + rate limits)
db/
  migrations/
    000001_init_users.sql
  query/
    users.sql             # optional sqlc
pkg/
  helpers/
    gcs.go, jwt.go, logger.go, password.go, redis.go, response.go
Makefile
sqlc.yaml
```

Environment variables (.env)
```
APP_NAME=go-ddd-boilerplate
APP_ENV=development
PORT=8080
GIN_MODE=release
COOKIE_DOMAIN=localhost
COOKIE_SECURE=false

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=appdb
DB_SSLMODE=disable
DB_MAX_CONNS=10
DB_MIN_CONNS=2
DB_MAX_CONN_LIFETIME=1h

REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

GCS_BUCKET=
GCS_CREDENTIALS_JSON=

JWT_ACCESS_SECRET=devaccesssecret
JWT_REFRESH_SECRET=devrefreshsecret
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

MIGRATIONS_DIR=db/migrations
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

Run locally
- Prereq: Go 1.22+ (or latest), Postgres, Redis, golang-migrate CLI
- Install modules: make tidy
- Run migrations: make migrate-up
- Seed demo user: make seed (email: admin@example.com, password: password123)
- Start API: make run (listens on :$PORT)

API overview
- POST /api/login (rate-limited 5/min per IP+path)
- POST /api/refresh (rate-limited 20/min per IP+path)
- POST /api/logout (JWT required; protected group limited 120/min per IP)
- GET  /api/profile (JWT)
- PUT  /api/profile (JWT)

Notes
- JWT tokens are httpOnly cookies: access_token, refresh_token.
- Responses include a request_id and timestamp. RequestID middleware sets request_id.
- Redis must be available for rate limiting. On Redis errors, middleware fails open.

SQLC (optional)
- Define queries in db/query/*.sql
- Generate with make sqlc-generate

Migrations
- Up: make migrate-up
- Down (1): make migrate-down
- Drop all: make migrate-drop

Docker (local)
- Build: docker build --platform linux/amd64 -t boilerplate-go-pgsql:latest .
- Run: docker run --rm -p 8080:8080 --env-file .env boilerplate-go-pgsql:latest

Deploy to Railway
- Create a new Railway project.
- Add services: PostgreSQL and Redis plugins. Copy their connection details.
- Deploy from GitHub using the Dockerfile in this repo.
- Set environment variables in Railway:
  - PORT: 8080 (Railway also injects PORT; app reads it)
  - DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME from the Postgres plugin
  - DB_SSLMODE=require (Railway Postgres enforces TLS)
  - REDIS_ADDR as host:port from the Redis plugin, REDIS_PASSWORD if provided, REDIS_DB=0
  - JWT_ACCESS_SECRET, JWT_REFRESH_SECRET (generate strong secrets)
  - CORS_ALLOWED_ORIGINS to your frontend URL (e.g., https://your-app.vercel.app)
  - COOKIE_DOMAIN to your domain; set COOKIE_SECURE=true for HTTPS
  - MIGRATIONS_DIR=db/migrations (default)
- Redeploy; the app runs migrations at startup and serves on /api.

Troubleshooting
- 429 Too Many Requests: hit rate limits; check Retry-After header.
- Invalid tokens: verify JWT secrets match across deployments.
- SSL errors to Postgres on Railway: ensure DB_SSLMODE=require.
