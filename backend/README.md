# Backend

Go backend for the monitoring console.

## Run

```bash
cd backend
go run ./cmd/healthmon
```

## Environment

- `CONFIG_PATH`: path to the JSON config file, default `config/default.json`
- `STATE_PATH`: path to the persisted state file, default `data/state.json`
- `MONGODB_URI`: optional MongoDB connection string for mirrored persistence
- `MONGODB_DATABASE`: MongoDB database name, default `healthmon`
- `MONGODB_COLLECTION_PREFIX`: collection prefix, default `healthmon`

MongoDB is best-effort only. The backend keeps running with the local file store if MongoDB is unavailable.

## API

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/checks`
- `POST /api/v1/checks`
- `PUT /api/v1/checks/{id}`
- `DELETE /api/v1/checks/{id}`
- `POST /api/v1/runs`
- `GET /api/v1/summary`
- `GET /api/v1/results?checkId=&days=7`
- Dashboard aliases:
  - `GET /api/v1/dashboard/checks`
  - `GET /api/v1/dashboard/summary`
  - `GET /api/v1/dashboard/results`
