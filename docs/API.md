# Solar Chicken API

Base path: `/api/v2`, served on the same origin as the web app (Next.js forwards it to the `api` container).
Authentication: `Authorization: Bearer <jwt>` on every route except `/auth/*` and `/system/bootstrap`.
Errors: `{"error": "message"}`, in the language of the `Accept-Language` header (French when it is
missing or starts with `fr`, English otherwise). A 401 means the token is invalid or expired.
Dates are ISO 8601 in UTC; display them in the coop's time zone (`coop.timezone`).

## Types

```ts
type Mode = 'shadow' | 'live';
type DeviceRole = 'main_door' | 'nest_box' | 'door' | 'feeder';
type Strategy = 'command' | 'onboard' | 'monitor';
type Anchor = 'sunrise' | 'sunset' | 'fixed' | 'device_open' | 'device_close';
type EventAction = 'open' | 'close' | 'light_before_open' | 'light_after_open' | 'light_before_close' | 'light_after_close';
type EventStatus = 'pending' | 'sent' | 'confirmed' | 'failed' | 'skipped' | 'shadow';
type HHMM = string; // "08:00"

interface CoopView { id: string; name: string; omletGroupId: string; latitude: number; longitude: number;
  timezone: string; hasApiKey: boolean; deviceCount: number; language: 'fr' | 'en'; }

interface Rule { id?: string; deviceId?: string;
  openAnchor: Anchor; openOffsetMinutes: number; openFixedTime: HHMM | null; openRefDeviceId: string | null;
  openNotBefore: HHMM | null; openNotAfter: HHMM | null;
  closeAnchor: Anchor; closeOffsetMinutes: number; closeFixedTime: HHMM | null; closeRefDeviceId: string | null;
  closeNotBefore: HHMM | null; closeNotAfter: HHMM | null;
  lightBeforeOpenMinutes: number; lightBeforeCloseMinutes: number;
  enableLightMorning: boolean; enableLightEvening: boolean; lightOffDelayMinutes: number; }

interface Device { id: string; coopId: string; omletDeviceId: string; deviceType: 'Autodoor' | 'Feeder' | string;
  role: DeviceRole; name: string; strategy: Strategy; hasLight: boolean; enabled: boolean; position: number;
  onboardSyncedDay?: string; onboardOpenTime?: HHMM; onboardCloseTime?: HHMM; onboardSyncedAt?: string; onboardSyncError?: string;
  createdAt: string; updatedAt: string; rule?: Rule | null; }

interface PlannedEvent { id: string; coopId: string; deviceId: string; day: string; action: EventAction; dueAt: string;
  status: EventStatus; strategy: Strategy; attempts: number; nextCheckAt?: string; sentAt?: string; doneAt?: string;
  lastError?: string; note?: string; }

interface DayView { day: string; sunrise: string; sunset: string; events: PlannedEvent[]; errors: Record<string, string>; }

interface DeviceStateView { door?: string; fault?: string; light?: 'on' | 'off'; battery: number; power?: string; connected: boolean;
  lastOpen?: string; lastClose?: string; feedLevel?: number; fetchedAt?: string; error?: string; }

// states: last known state of each device (key = device id), refreshed by the server every 5 minutes.
// Reading it never calls Omlet.
interface CoopDetail { coop: CoopView; devices: Device[]; today: DayView; tomorrow: DayView; mode: Mode;
  states: Record<string, DeviceStateView>; }

interface OmletDeviceView { deviceId: string; name: string; deviceType: string; groupId: string; sameGroup: boolean;
  alreadyAdded: boolean; powerSource: string; hasLight: boolean; doorState?: string; }

interface DeviceStatus { name: string; deviceType: string; groupId: string;
  door: { state: string; lastOpenTime: string; lastCloseTime: string; fault: string; lightLevel: number } | null;
  light: { state: 'on' | 'off' } | null;
  feeder: { state: string; lastOpenTime: string; lastCloseTime: string; fault: string; feedLevel: number } | null;
  batteryLevel: number; powerSource: 'external' | 'battery' | string; firmware: string; wifiStrength: number; connected: boolean; }

interface ActionLog { id: string; deviceId: string; actionType: string; status: 'success' | 'error'; errorMessage?: string;
  triggeredBy: 'auto' | 'manual' | 'catchup' | 'fallback' | 'sync'; userId?: string; note?: string; executedAt: string; }

interface PreviewDay { day: string; open?: string; close?: string; events: { action: EventAction; dueAt: string }[]; error?: string; }

interface SystemInfo { mode: Mode; version: string; now: string; lastTick: string; lastPlan: string; startedAt: string; }
interface Bootstrap { needsSetup: boolean; registrationOpen: boolean; version: string; }
```

`User` and `NotificationSettings` are described in [`frontend/types/index.ts`](../frontend/types/index.ts).

## Routes

| Method | Route | Body | Response |
|---|---|---|---|
| GET | `/system/bootstrap` | | `Bootstrap` (no authentication) |
| POST | `/auth/register` | `{email, password, firstName, lastName}` | 201; **403** when sign-ups are closed (the first account can always be created) |
| POST | `/auth/login` | `{email, password}` | `{token, user}` |
| GET | `/auth/me` | | `User` |
| GET | `/auth/verify-email?token=` | | `{message}` |
| POST | `/auth/resend-verification` | `{email}` | `{message}` |
| GET | `/system` | | `SystemInfo` |
| GET | `/coops` | | `CoopView[]` |
| POST | `/coops` | `{name, latitude, longitude, timezone, omletApiKey}` | 201 `CoopView`; the key is checked against Omlet first |
| GET | `/coops/:id` | | `CoopDetail` |
| PUT | `/coops/:id` | `{name?, latitude?, longitude?, timezone?, omletApiKey?, language?}` | `CoopDetail` |
| GET | `/coops/:id/plan?day=YYYY-MM-DD` | | `DayView` |
| GET | `/coops/:id/omlet-devices` | | `OmletDeviceView[]` (devices of the Omlet account) |
| POST | `/coops/:id/devices` | `{omletDeviceId, role, name?, strategy?}` | 201 `Device`; 409 if already added or a second main door |
| PUT | `/devices/:id` | `{name?, role?, strategy?, enabled?, position?}` | `Device` |
| DELETE | `/devices/:id` | | 204; 409 if another device's rule refers to it |
| GET | `/devices/:id/status` | | `DeviceStatus` (live from Omlet) |
| POST | `/devices/:id/actions/:action` | action ∈ `open, close, stop, light_on, light_off` | `{message}` |
| GET | `/devices/:id/logs?limit=50` | | `ActionLog[]` (newest first) |
| GET | `/devices/:id/rule` | | `Rule` |
| PUT | `/devices/:id/rule` | `Rule` | `Rule`; 400 with a message if invalid |
| POST | `/devices/:id/rule/preview` | `Rule` (draft) | `PreviewDay[]` (next 7 days) |
| GET/PUT | `/settings/notifications` | | `NotificationSettings` |
| POST | `/settings/notifications/test` | `{telegramBotToken?, telegramChatId?}` | `{message}` |

Outside `/api/v2`:

- `GET /health`: 200 when the database answers and the scheduler loop is alive, 503 otherwise.
- `GET /ha/state`: read-only summary for Home Assistant, with `Authorization: Bearer <HA_TOKEN>` (see the README).

## How the doors are driven

- Strategies:
  - `onboard` (default for doors): every night the server writes the day's times into the door's control unit
    (Omlet "time" mode). The door runs on its own even if the server or the internet is down; the server checks
    afterwards and sends the command itself if the door did not move.
  - `command`: the server sends open/close at the exact time, checks, retries and alerts.
  - `monitor` (default for feeders): status only.
- Anchors: `sunrise`/`sunset` plus an offset in minutes (negative = before), `fixed` (time of day),
  `device_open`/`device_close` (relative to another door of the same coop). Default nest box rule: opens with
  the main door (`device_open` + 0) and closes 2 h before it (`device_close` − 120); every rule can be edited.
- Optional bounds `NotBefore`/`NotAfter` (e.g. never before 08:00).
- Light: only for devices with `hasLight` and the `command` strategy.
- Statuses: `pending` upcoming, `sent` sent and being checked, `confirmed` done and checked, `failed` failed
  (alert sent), `skipped` not done (see `note`), `shadow` simulated.
- `mode = 'shadow'` (`EXECUTION_MODE=shadow`): the server computes and checks everything but sends nothing.
