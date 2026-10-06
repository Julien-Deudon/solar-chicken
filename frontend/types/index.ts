// Types de l'API Solar Chicken (contrat : docs/API.md).
// Toutes les dates sont en ISO 8601 (UTC) ; les heures HH:MM sont locales au poulailler.

// ---------------------------------------------------------------------------
// Authentification (inchangé par rapport à la v1)
// ---------------------------------------------------------------------------

export interface User {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  createdAt: string;
  updatedAt: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  firstName: string;
  lastName: string;
}

export interface MessageResponse {
  message: string;
}

/** Corps des erreurs renvoyées par l'API : {"error": "message en français"}. */
export interface ApiErrorBody {
  error?: string;
  details?: string;
}

// ---------------------------------------------------------------------------
// Notifications Telegram (inchangé par rapport à la v1)
// ---------------------------------------------------------------------------

export interface TelegramAccount {
  id: string;
  name: string;
  botToken: string;
  chatId: string;
}

export interface NotificationSettings {
  id?: string;
  /** Ancien format (un seul compte), gardé pour compatibilité. */
  telegramBotToken?: string;
  /** Ancien format (un seul compte), gardé pour compatibilité. */
  telegramChatId?: string;
  telegramAccounts: TelegramAccount[];
  notifyDailySchedule: boolean;
  notifyOnOpen: boolean;
  notifyOnClose: boolean;
  notifyOnLight: boolean;
  notifyOnError: boolean;
}

export interface TestNotificationRequest {
  telegramBotToken?: string;
  telegramChatId?: string;
}

// ---------------------------------------------------------------------------
// v2 : poulailler, appareils, règles, planning
// ---------------------------------------------------------------------------

export type Mode = 'shadow' | 'live';
export type DeviceRole = 'main_door' | 'nest_box' | 'door' | 'feeder';
export type Strategy = 'command' | 'onboard' | 'monitor';
export type Anchor = 'sunrise' | 'sunset' | 'fixed' | 'device_open' | 'device_close';
export type EventAction =
  | 'open'
  | 'close'
  | 'light_before_open'
  | 'light_after_open'
  | 'light_before_close'
  | 'light_after_close';
export type EventStatus = 'pending' | 'sent' | 'confirmed' | 'failed' | 'skipped' | 'shadow';
/** Heure locale "08:00". */
export type HHMM = string;

export interface CoopView {
  id: string;
  name: string;
  omletGroupId: string;
  latitude: number;
  longitude: number;
  timezone: string;
  hasApiKey: boolean;
  deviceCount: number;
  /** Langue des notifications et des messages du planning. */
  language: 'fr' | 'en';
}

/** GET /system/bootstrap (sans connexion). */
export interface Bootstrap {
  needsSetup: boolean;
  registrationOpen: boolean;
  version: string;
}

export interface Rule {
  id?: string;
  deviceId?: string;
  openAnchor: Anchor;
  openOffsetMinutes: number;
  openFixedTime: HHMM | null;
  openRefDeviceId: string | null;
  openNotBefore: HHMM | null;
  openNotAfter: HHMM | null;
  closeAnchor: Anchor;
  closeOffsetMinutes: number;
  closeFixedTime: HHMM | null;
  closeRefDeviceId: string | null;
  closeNotBefore: HHMM | null;
  closeNotAfter: HHMM | null;
  lightBeforeOpenMinutes: number;
  lightBeforeCloseMinutes: number;
  enableLightMorning: boolean;
  enableLightEvening: boolean;
  lightOffDelayMinutes: number;
}

export interface Device {
  id: string;
  coopId: string;
  omletDeviceId: string;
  deviceType: 'Autodoor' | 'Feeder' | string;
  role: DeviceRole;
  name: string;
  strategy: Strategy;
  hasLight: boolean;
  enabled: boolean;
  position: number;
  onboardSyncedDay?: string;
  onboardOpenTime?: HHMM;
  onboardCloseTime?: HHMM;
  onboardSyncedAt?: string;
  onboardSyncError?: string;
  createdAt: string;
  updatedAt: string;
  rule?: Rule | null;
}

export interface PlannedEvent {
  id: string;
  coopId: string;
  deviceId: string;
  day: string;
  action: EventAction;
  dueAt: string;
  status: EventStatus;
  strategy: Strategy;
  attempts: number;
  nextCheckAt?: string;
  sentAt?: string;
  doneAt?: string;
  lastError?: string;
  note?: string;
}

export interface DayView {
  day: string;
  sunrise: string;
  sunset: string;
  events: PlannedEvent[];
  /** Erreurs de planification, indexées par identifiant d'appareil. */
  errors: Record<string, string>;
}

/** Dernier état connu d'un appareil, relu par le serveur toutes les 5 minutes. */
export interface DeviceStateView {
  door?: string;
  fault?: string;
  light?: 'on' | 'off';
  battery: number;
  power?: string;
  connected: boolean;
  lastOpen?: string;
  lastClose?: string;
  feedLevel?: number;
  fetchedAt?: string;
  error?: string;
}

export interface CoopDetail {
  coop: CoopView;
  devices: Device[];
  today: DayView;
  tomorrow: DayView;
  mode: Mode;
  /** Clé : id de l'appareil. Absent si le serveur n'a pas encore relu l'état. */
  states?: Record<string, DeviceStateView>;
}

export interface OmletDeviceView {
  deviceId: string;
  name: string;
  deviceType: string;
  groupId: string;
  sameGroup: boolean;
  alreadyAdded: boolean;
  powerSource: string;
  hasLight: boolean;
  doorState?: string;
}

export interface DeviceStatus {
  name: string;
  deviceType: string;
  groupId: string;
  door: {
    state: string;
    lastOpenTime: string;
    lastCloseTime: string;
    fault: string;
    lightLevel: number;
  } | null;
  light: { state: 'on' | 'off' } | null;
  feeder: {
    state: string;
    lastOpenTime: string;
    lastCloseTime: string;
    fault: string;
    feedLevel: number;
  } | null;
  batteryLevel: number;
  powerSource: 'external' | 'battery' | string;
  firmware: string;
  wifiStrength: number;
  connected: boolean;
}

export type ActionTrigger = 'auto' | 'manual' | 'catchup' | 'fallback' | 'sync';

export interface ActionLog {
  id: string;
  deviceId: string;
  actionType: string;
  status: 'success' | 'error';
  errorMessage?: string;
  triggeredBy: ActionTrigger;
  userId?: string;
  note?: string;
  executedAt: string;
}

export interface PreviewDay {
  day: string;
  open?: string;
  close?: string;
  events: { action: EventAction; dueAt: string }[];
  error?: string;
}

export interface SystemInfo {
  mode: Mode;
  version: string;
  now: string;
  lastTick: string;
  lastPlan: string;
  startedAt: string;
}

// ---------------------------------------------------------------------------
// Corps de requêtes
// ---------------------------------------------------------------------------

export interface UpdateCoopRequest {
  name?: string;
  latitude?: number;
  longitude?: number;
  timezone?: string;
  /** Écriture seule : la clé n'est jamais renvoyée par l'API. */
  omletApiKey?: string;
  language?: 'fr' | 'en';
}

export interface CreateCoopRequest {
  name: string;
  latitude: number;
  longitude: number;
  timezone: string;
  omletApiKey: string;
  language?: 'fr' | 'en';
}

export interface AddDeviceRequest {
  omletDeviceId: string;
  role: DeviceRole;
  name?: string;
  strategy?: Strategy;
}

export interface UpdateDeviceRequest {
  name?: string;
  role?: DeviceRole;
  strategy?: Strategy;
  enabled?: boolean;
  position?: number;
}

/** Actions manuelles : POST /devices/:id/actions/:action */
export type DeviceAction = 'open' | 'close' | 'stop' | 'light_on' | 'light_off';
