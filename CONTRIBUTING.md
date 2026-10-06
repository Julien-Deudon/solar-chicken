# Contributing

Issues and pull requests are welcome — in English or French.

- Backend: Go (`backend/`) — `go test ./...` (set `GOFLAGS=-buildvcs=false` if your git is unusual).
- Frontend: Next.js (`frontend/`) — `npm ci`, then `bash dev/start-dev.sh` runs the app against a mock API
  (log in with any e-mail and the password `secret`, no Omlet account needed).
- Keep the door logic safe: any change to the scheduler or the engine needs a test
  (`backend/internal/planner`, `backend/internal/engine`).
- UI texts live in `frontend/locales/fr.ts` and `frontend/locales/en.ts`.
- README screenshots: `MOCK_SCENE=demo MOCK_NOW=16:20 bash dev/start-dev.sh`, then `node dev/screenshots.mjs`
  in another terminal (needs Google Chrome; set `CHROME=/path/to/chrome` elsewhere than macOS).
