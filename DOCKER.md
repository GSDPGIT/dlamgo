# Docker Quick Start

This repo now includes a Dockerized path for the Go + SQLite backend rewrite.

## Files

- `docker-compose.yml`
- `.env.example`
- `go-panel-backend/Dockerfile`
- `vite-frontend/Dockerfile`

## Start

```bash
cp .env.example .env
docker compose up --build -d
```

Frontend defaults to `http://127.0.0.1:6366`

Backend defaults to `http://127.0.0.1:6365`

## Notes

- SQLite data is stored in the named volume `sqlite_data`.
- The frontend proxies `/api/v1`, `/flow/*`, and `/system-info` to the `backend` service.
- Set `ADMIN_PASSWORD` in `.env` before first boot if you do not want a generated password.
- This Go rewrite was prepared for Docker delivery, but you should still run a local build/test pass before production use.
