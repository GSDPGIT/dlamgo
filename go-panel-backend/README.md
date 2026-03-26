# Go Panel Backend

This directory contains the Go rewrite of the original Java backend.

## Environment

- `APP_ADDR` default `:6365`
- `DATABASE_PATH` default `data/flux-panel.db`
- `JWT_SECRET` optional, auto-generated when omitted
- `ADMIN_USERNAME` default `admin_user`
- `ADMIN_PASSWORD` optional, auto-generated on first boot when omitted
- `CORS_ALLOWED_ORIGINS` optional comma-separated list
- `LOGIN_RATE_LIMIT_PER_MINUTE` default `12`

## Notes

- Storage is SQLite-only in this rewrite.
- The backend seeds public config keys on first boot.
- The initial admin password is generated and printed once when `ADMIN_PASSWORD` is not set.
