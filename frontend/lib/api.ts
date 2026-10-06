import axios, { type AxiosError, type AxiosResponse } from 'axios';
import { getLocale, t, type MessageKey } from '@/lib/i18n';
import { useAuthStore } from '@/lib/store';
import type {
  ActionLog,
  AddDeviceRequest,
  ApiErrorBody,
  AuthResponse,
  Bootstrap,
  CoopDetail,
  CoopView,
  CreateCoopRequest,
  DayView,
  Device,
  DeviceAction,
  DeviceStatus,
  LoginRequest,
  MessageResponse,
  NotificationSettings,
  OmletDeviceView,
  PreviewDay,
  RegisterRequest,
  Rule,
  SystemInfo,
  TestNotificationRequest,
  UpdateCoopRequest,
  UpdateDeviceRequest,
  User,
} from '@/types';

// Même origine que le frontend : Next.js relaie /api/v2/* vers le conteneur de l'API.
const api = axios.create({
  baseURL: '/api/v2',
  headers: { 'Content-Type': 'application/json' },
  timeout: 30_000,
});

// Routes publiques : un 401 y signifie « identifiants refusés », pas « session expirée ».
const PUBLIC_AUTH_PATHS = [
  '/auth/login',
  '/auth/register',
  '/auth/verify-email',
  '/auth/resend-verification',
];

const isPublicAuthRequest = (url?: string) =>
  !!url && PUBLIC_AUTH_PATHS.some((path) => url.startsWith(path));

// Jeton JWT, et langue de l'interface : le serveur répond ses messages d'erreur dans la même langue.
api.interceptors.request.use((config) => {
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    config.headers['Accept-Language'] = getLocale();
  }
  return config;
});

// 401 → jeton effacé et retour à /login
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (
      typeof window !== 'undefined' &&
      error.response?.status === 401 &&
      !isPublicAuthRequest(error.config?.url)
    ) {
      useAuthStore.getState().logout(); // efface aussi localStorage 'token'
      if (window.location.pathname !== '/login') {
        window.location.replace('/login');
        // La page change : on laisse la promesse en attente pour éviter des toasts d'erreur inutiles.
        return new Promise<never>(() => {});
      }
    }
    return Promise.reject(error);
  }
);

// ---------------------------------------------------------------------------
// Erreurs
// ---------------------------------------------------------------------------

/** Erreur applicative dont le message (déjà traduit) peut être affiché tel quel. */
export class AppError extends Error {
  constructor(message: string, public readonly status?: number) {
    super(message);
    this.name = 'AppError';
    Object.setPrototypeOf(this, AppError.prototype); // instanceof fiable même si compilé en ES5
  }
}

// Le serveur répond dans la langue d'Accept-Language, sauf quelques messages restés en anglais
// (routes d'authentification et notifications) : traduits ici, dans les deux langues.
const KNOWN_MESSAGES = new Map<string, MessageKey>([
  ['Invalid credentials', 'errors.invalidCredentials'],
  ['User already exists', 'errors.userExists'],
  ['Invalid or expired verification token', 'errors.invalidVerificationToken'],
  ['Token is required', 'errors.verificationTokenRequired'],
  ['Email is already verified', 'errors.emailAlreadyVerified'],
  ['Failed to send verification email', 'errors.verificationEmailFailed'],
  ['Failed to send test notification', 'errors.testNotificationFailed'],
  ['Telegram credentials not configured', 'errors.telegramNotConfigured'],
  ['Notification settings not configured', 'errors.notificationsNotConfigured'],
  ['Failed to update settings', 'errors.settingsSaveFailed'],
  ['Failed to get settings', 'errors.settingsReadFailed'],
  ['Invalid or expired token', 'errors.sessionExpired'],
  ['Authorization header required', 'errors.sessionExpired'],
  ['Rate limit exceeded', 'errors.tooManyAttempts'],
]);

/**
 * Message à afficher (toast, encadré) pour une erreur d'appel à l'API, dans la langue de l'interface.
 * `fallback` : clé du message quand l'erreur n'en apporte pas.
 */
export function getErrorMessage(error: unknown, fallback: MessageKey = 'errors.generic'): string {
  if (axios.isAxiosError<ApiErrorBody>(error)) {
    const body = error.response?.data;
    if (body && typeof body === 'object' && typeof body.error === 'string' && body.error.trim()) {
      const known = KNOWN_MESSAGES.get(body.error);
      const message = known ? t(known) : body.error;
      return body.details ? `${message} (${body.details})` : message;
    }
    if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
      return t('errors.timeout');
    }
    if (!error.response) return t('errors.unreachable');
    const status = error.response.status;
    if (status === 429) return t('errors.tooManyAttempts');
    if (status >= 500) return t('errors.server', { status });
    return t(fallback);
  }
  if (error instanceof AppError) return error.message;
  return t(fallback);
}

/** Code HTTP d'une erreur d'appel à l'API (undefined si pas de réponse). */
export function getErrorStatus(error: unknown): number | undefined {
  if (axios.isAxiosError(error)) return error.response?.status;
  if (error instanceof AppError) return error.status;
  return undefined;
}

/** Vrai si la requête a été annulée (AbortController). */
export const isCancelled = (error: unknown) => axios.isCancel(error);

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

const data = <T>(request: Promise<AxiosResponse<T>>): Promise<T> => request.then((r) => r.data);

// Appareil → poulailler, appris à chaque lecture d'un poulailler (il n'y a pas de GET /devices/:id).
const deviceCoopIds = new Map<string, string>();

const remember = (detail: CoopDetail): CoopDetail => {
  detail.devices.forEach((device) => deviceCoopIds.set(device.id, detail.coop.id));
  return detail;
};

export const authApi = {
  login: (body: LoginRequest) => data(api.post<AuthResponse>('/auth/login', body)),
  /** 201 si le compte est créé ; 403 si les inscriptions sont fermées. */
  register: async (body: RegisterRequest): Promise<void> => {
    await api.post('/auth/register', body);
  },
  me: () => data(api.get<User>('/auth/me')),
  verifyEmail: (token: string) =>
    data(api.get<MessageResponse>('/auth/verify-email', { params: { token } })),
  resendVerification: (email: string) =>
    data(api.post<MessageResponse>('/auth/resend-verification', { email })),
};

export const systemApi = {
  get: () => data(api.get<SystemInfo>('/system')),
  /** Premier démarrage ? (sans connexion) */
  bootstrap: () => data(api.get<Bootstrap>('/system/bootstrap')),
};

export const coopsApi = {
  list: () => data(api.get<CoopView[]>('/coops')),
  create: (body: CreateCoopRequest) => data(api.post<CoopDetail>('/coops', body)).then(remember),
  get: (id: string) => data(api.get<CoopDetail>(`/coops/${id}`)).then(remember),
  update: (id: string, body: UpdateCoopRequest) =>
    data(api.put<CoopDetail>(`/coops/${id}`, body)).then(remember),
  /** day au format AAAA-MM-JJ (jour local du poulailler). */
  plan: (id: string, day: string) =>
    data(api.get<DayView>(`/coops/${id}/plan`, { params: { day } })),
  omletDevices: (id: string) => data(api.get<OmletDeviceView[]>(`/coops/${id}/omlet-devices`)),
  addDevice: (id: string, body: AddDeviceRequest) =>
    data(api.post<Device>(`/coops/${id}/devices`, body)).then((device) => {
      deviceCoopIds.set(device.id, device.coopId);
      return device;
    }),
};

export const devicesApi = {
  update: (id: string, body: UpdateDeviceRequest) => data(api.put<Device>(`/devices/${id}`, body)),
  /** 204 ; 409 si un autre appareil s'y réfère. */
  remove: async (id: string): Promise<void> => {
    await api.delete(`/devices/${id}`);
    deviceCoopIds.delete(id);
  },
  status: (id: string) => data(api.get<DeviceStatus>(`/devices/${id}/status`)),
  action: (id: string, action: DeviceAction) =>
    data(api.post<MessageResponse>(`/devices/${id}/actions/${action}`)),
  /** Plus récent d'abord. */
  logs: (id: string, limit = 50) =>
    data(api.get<ActionLog[]>(`/devices/${id}/logs`, { params: { limit } })),
  getRule: (id: string) => data(api.get<Rule>(`/devices/${id}/rule`)),
  /** 400 avec message si la règle est invalide. */
  putRule: (id: string, rule: Rule) => data(api.put<Rule>(`/devices/${id}/rule`, rule)),
  /** Horaires des 7 prochains jours avec la règle proposée (brouillon). */
  previewRule: (id: string, rule: Rule, signal?: AbortSignal) =>
    data(api.post<PreviewDay[]>(`/devices/${id}/rule/preview`, rule, { signal })),
};

export const settingsApi = {
  getNotifications: () => data(api.get<NotificationSettings>('/settings/notifications')),
  updateNotifications: (body: Partial<NotificationSettings>) =>
    data(api.put<NotificationSettings>('/settings/notifications', body)),
  testNotification: (body: TestNotificationRequest) =>
    data(api.post<MessageResponse>('/settings/notifications/test', body)),
};

/**
 * Retrouve le poulailler (CoopDetail) qui contient un appareil.
 * Le contrat n'a pas de GET /devices/:id : on part du poulailler déjà vu,
 * sinon on parcourt GET /coops puis GET /coops/:id.
 */
export async function findDeviceContext(
  deviceId: string
): Promise<{ coop: CoopDetail; device: Device }> {
  const known = deviceCoopIds.get(deviceId);
  if (known) {
    try {
      const detail = await coopsApi.get(known);
      const device = detail.devices.find((d) => d.id === deviceId);
      if (device) return { coop: detail, device };
    } catch (error) {
      if (getErrorStatus(error) !== 404) throw error;
    }
  }
  const coops = await coopsApi.list();
  for (const coop of coops) {
    if (coop.id === known) continue;
    const detail = await coopsApi.get(coop.id);
    const device = detail.devices.find((d) => d.id === deviceId);
    if (device) return { coop: detail, device };
  }
  throw new AppError(t('errors.deviceNotFound'), 404);
}

export default api;
