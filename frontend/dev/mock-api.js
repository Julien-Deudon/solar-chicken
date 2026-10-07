// Mock local de l'API Solar Chicken (contrat docs/API.md) pour tester le frontend.
// MOCK_FRESH=1 : premier démarrage (aucun compte, aucun poulailler) pour parcourir
// inscription → assistant de création du poulailler → ajout des appareils.
// MOCK_SCENE=demo : journée sans incident en mode réel (captures d'écran du README).
// MOCK_NOW=16:20 : horloge du mock décalée à cette heure de Paris, aujourd'hui.
// Langues, comme le vrai serveur : messages d'erreur selon Accept-Language (français si absent
// ou « fr… », anglais sinon) ; notes du planning et du journal selon la langue du poulailler.
const http = require('http');

const PORT = Number(process.env.MOCK_PORT || 8787);
const FRESH = process.env.MOCK_FRESH === '1';
const DEMO = process.env.MOCK_SCENE === 'demo';
const TZ = 'Europe/Paris';
const COOP_ID = '11111111-1111-4111-8111-111111111111';
const MAIN = '22222222-2222-4222-8222-222222222222';
const NEST = '33333333-3333-4333-8333-333333333333';
const FEEDER = '44444444-4444-4444-8444-444444444444';

const user = { id: 'u1', email: 'test@example.com', firstName: 'Test', lastName: 'User', createdAt: '2025-01-01T00:00:00Z', updatedAt: '2025-01-01T00:00:00Z' };
// Compte créé par /auth/register en mode premier démarrage (connexion avec son mot de passe ou « secret »).
let registered = null;
let mode = DEMO ? 'live' : 'shadow';
const NOW_SHIFT = process.env.MOCK_NOW ? Date.parse(at(dayKey(new Date()), process.env.MOCK_NOW)) - Date.now() : 0;
/** Instant courant du mock (décalé par MOCK_NOW). */
const now = () => Date.now() + NOW_SHIFT;

let coop = FRESH
  ? null
  : { id: COOP_ID, name: 'Poulailler du jardin', omletGroupId: 'grp-1', latitude: 48.8566, longitude: 2.3522, timezone: TZ, hasApiKey: true, deviceCount: 3, language: 'fr' };

/** Langue de la requête, comme le serveur : français si Accept-Language est absent ou commence par « fr ». */
const langOf = (req) => {
  const al = String(req.headers['accept-language'] || '').trim().toLowerCase();
  return al === '' || al.startsWith('fr') ? 'fr' : 'en';
};
/** Message d'erreur dans la langue de la requête. */
const tr = (req, fr, en) => (langOf(req) === 'en' ? en : fr);
/** Texte produit par le moteur (notes, erreurs du planning) : langue du poulailler. */
const tc = (fr, en) => (coop && coop.language === 'en' ? en : fr);

function dayKey(d) {
  const p = new Intl.DateTimeFormat('en-CA', { timeZone: TZ, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(d);
  const g = (t) => p.find((x) => x.type === t).value;
  return `${g('year')}-${g('month')}-${g('day')}`;
}
// Instant UTC pour "jour local + HH:MM" (Paris = UTC+2 en octobre)
function at(day, hhmm, offsetHours = 2) {
  const [h, m] = hhmm.split(':').map(Number);
  const [y, mo, d] = day.split('-').map(Number);
  return new Date(Date.UTC(y, mo - 1, d, h - offsetHours, m)).toISOString();
}

/** Règle proposée pour un pondoir, comme le serveur : avec la porte principale, fermeture 2 h avant elle. */
const nestDefaultRule = (id, mainId) => mainId
  ? { ...defaultRule(id), openAnchor: 'device_open', openRefDeviceId: mainId, openOffsetMinutes: 0, closeAnchor: 'device_close', closeRefDeviceId: mainId, closeOffsetMinutes: -120 }
  : { ...defaultRule(id), closeOffsetMinutes: -100 };
const defaultRule = (id) => ({ id: 'r-' + id, deviceId: id, openAnchor: 'sunrise', openOffsetMinutes: -10, openFixedTime: null, openRefDeviceId: null, openNotBefore: null, openNotAfter: null, closeAnchor: 'sunset', closeOffsetMinutes: 20, closeFixedTime: null, closeRefDeviceId: null, closeNotBefore: null, closeNotAfter: null, lightBeforeOpenMinutes: 0, lightBeforeCloseMinutes: 0, enableLightMorning: false, enableLightEvening: false, lightOffDelayMinutes: 0 });

let devices = FRESH ? [] : [
  {
    id: MAIN, coopId: COOP_ID, omletDeviceId: 'omlet-main', deviceType: 'Autodoor', role: 'main_door', name: 'Porte principale',
    strategy: 'command', hasLight: true, enabled: true, position: 0, createdAt: '2025-01-01T00:00:00Z', updatedAt: '2025-01-01T00:00:00Z',
    rule: { id: 'r1', deviceId: MAIN, openAnchor: 'sunrise', openOffsetMinutes: -10, openFixedTime: null, openRefDeviceId: null, openNotBefore: null, openNotAfter: null,
      closeAnchor: 'sunset', closeOffsetMinutes: 20, closeFixedTime: null, closeRefDeviceId: null, closeNotBefore: null, closeNotAfter: null,
      lightBeforeOpenMinutes: 2, lightBeforeCloseMinutes: 0, enableLightMorning: true, enableLightEvening: true, lightOffDelayMinutes: 5, createdAt: '2025-01-01T00:00:00Z', updatedAt: '2025-01-01T00:00:00Z' },
  },
  {
    id: NEST, coopId: COOP_ID, omletDeviceId: 'omlet-nest', deviceType: 'Autodoor', role: 'nest_box', name: 'Pondoir',
    strategy: 'onboard', hasLight: false, enabled: true, position: 1, createdAt: '2025-01-01T00:00:00Z', updatedAt: '2025-01-01T00:00:00Z',
    ...(DEMO
      ? { onboardSyncedDay: dayKey(new Date(now())), onboardOpenTime: '08:13', onboardCloseTime: '18:12', onboardSyncedAt: at(dayKey(new Date(now())), '00:05') }
      : { onboardSyncedDay: dayKey(new Date(now() - 86400000)), onboardOpenTime: '08:30', onboardCloseTime: '18:14', onboardSyncedAt: new Date(now() - 20 * 3600000).toISOString() }),
    rule: { id: 'r2', deviceId: NEST, openAnchor: 'device_open', openOffsetMinutes: 30, openFixedTime: null, openRefDeviceId: MAIN, openNotBefore: '08:00', openNotAfter: null,
      closeAnchor: 'sunset', closeOffsetMinutes: -60, closeFixedTime: null, closeRefDeviceId: null, closeNotBefore: null, closeNotAfter: null,
      lightBeforeOpenMinutes: 0, lightBeforeCloseMinutes: 0, enableLightMorning: false, enableLightEvening: false, lightOffDelayMinutes: 0 },
  },
  {
    id: FEEDER, coopId: COOP_ID, omletDeviceId: 'omlet-feeder', deviceType: 'Feeder', role: 'feeder', name: 'Mangeoire',
    strategy: 'monitor', hasLight: false, enabled: true, position: 2, createdAt: '2025-01-01T00:00:00Z', updatedAt: '2025-01-01T00:00:00Z', rule: null,
  },
];

function dayView(offsetDays) {
  const day = dayKey(new Date(now() + offsetDays * 86400000));
  const zero = '00000000-0000-0000-0000-000000000000';
  const ev = (id, deviceId, action, hhmm, status, extra = {}) => ({ id: offsetDays ? zero : id, coopId: COOP_ID, deviceId, day, action, dueAt: at(day, hhmm), status, strategy: 'command', attempts: status === 'pending' ? 0 : 1, ...extra });
  const t0 = new Date(now());
  const nowHM = `${String((t0.getUTCHours() + 2) % 24).padStart(2, '0')}:${String(t0.getUTCMinutes()).padStart(2, '0')}`;
  const later = (h) => { const t = new Date(now() + h * 3600000); return `${String((t.getUTCHours() + 2) % 24).padStart(2, '0')}:${String(t.getUTCMinutes()).padStart(2, '0')}`; };
  const events = DEMO
    ? (offsetDays === 0
      ? [
          ev('e1', MAIN, 'light_before_open', '07:41', 'confirmed', { doneAt: at(day, '07:41') }),
          ev('e2', MAIN, 'open', '07:43', 'confirmed', { doneAt: at(day, '07:43') }),
          ev('e3', MAIN, 'light_after_open', '07:48', 'confirmed', { doneAt: at(day, '07:48') }),
          ev('e4', NEST, 'open', '08:13', 'confirmed', { strategy: 'onboard', doneAt: at(day, '08:14') }),
          ev('e5', NEST, 'close', '18:12', 'pending', { strategy: 'onboard' }),
          ev('e6', MAIN, 'close', '19:32', 'pending'),
          ev('e7', MAIN, 'light_after_close', '19:37', 'pending'),
        ]
      : [
          ev('x', MAIN, 'light_before_open', '07:43', 'pending'),
          ev('x', MAIN, 'open', '07:45', 'pending'),
          ev('x', NEST, 'open', '08:15', 'pending', { strategy: 'onboard' }),
          ev('x', NEST, 'close', '18:10', 'pending', { strategy: 'onboard' }),
          ev('x', MAIN, 'close', '19:30', 'pending'),
        ])
    : offsetDays === 0
    ? [
        ev('e1', MAIN, 'light_before_open', '07:40', 'shadow'),
        ev('e2', MAIN, 'open', '07:42', 'shadow', { note: tc('aurait envoyé « ouverture »', 'would have sent the opening') }),
        ev('e3', MAIN, 'light_after_open', '07:47', 'skipped', { note: tc('lumière abandonnée : trop tard', 'light skipped: too late') }),
        ev('e4', NEST, 'open', '08:30', 'confirmed', { strategy: 'onboard', doneAt: at(day, '08:31') }),
        ev('e5', NEST, 'close', '18:14', 'failed', { strategy: 'onboard', lastError: tc('porte toujours ouverte après 3 vérifications', 'door still open after 3 checks') }),
        ev('e6', MAIN, 'close', nowHM < '23:00' ? later(1.5) : '23:59', 'pending'),
        ev('e7', MAIN, 'light_after_close', nowHM < '23:00' ? later(1.6) : '23:59', 'pending'),
      ]
    : [
        ev('x', MAIN, 'light_before_open', '07:41', 'pending'),
        ev('x', MAIN, 'open', '07:43', 'pending'),
        ev('x', NEST, 'open', '08:30', 'pending'),
        ev('x', NEST, 'close', '18:12', 'pending'),
        ev('x', MAIN, 'close', '19:32', 'pending'),
      ];
  const errors = !DEMO && offsetDays === 1 && devices.some((d) => d.id === NEST)
    ? { [NEST]: tc("fermeture (07:00) avant l'ouverture (08:30) : aucune action prévue ce jour", 'closing (07:00) before opening (08:30): nothing scheduled that day') }
    : {};
  return { day, sunrise: at(day, '07:53'), sunset: at(day, '19:12'), events: events.filter((e) => devices.some((d) => d.id === e.deviceId)), errors };
}

function detail() {
  const fetchedAt = new Date(now() - 2 * 60000).toISOString();
  const today = dayKey(new Date(now()));
  const yesterday = dayKey(new Date(now() - 86400000));
  const states = DEMO ? {
    [MAIN]: { door: 'open', fault: 'none', light: 'off', battery: 100, power: 'external', connected: true, lastOpen: at(today, '07:43'), lastClose: at(yesterday, '19:34'), fetchedAt },
    [NEST]: { door: 'open', fault: 'none', battery: 87, power: 'battery', connected: true, lastOpen: at(today, '08:14'), lastClose: at(yesterday, '18:14'), fetchedAt },
    [FEEDER]: { door: 'open', fault: 'none', battery: 76, power: 'battery', connected: true, feedLevel: 64, lastOpen: at(today, '07:50'), lastClose: at(yesterday, '19:21'), fetchedAt },
  } : {
    [MAIN]: { door: 'closed', fault: 'none', light: 'off', battery: 100, power: 'external', connected: true, lastOpen: '2026-10-06T07:42:13+02:00', lastClose: '2026-10-06T19:32:40+02:00', fetchedAt },
    [NEST]: { door: 'closed', fault: 'blocked', battery: 87, power: 'battery', connected: true, lastOpen: '2026-10-06T08:16:02+02:00', lastClose: '2026-10-06T18:12:00+02:00', fetchedAt },
    [FEEDER]: { door: 'closed', fault: 'none', battery: 15, power: 'battery', connected: false, feedLevel: 64, lastOpen: '2026-10-06T07:49:58+02:00', lastClose: '2026-10-06T19:21:16+02:00', fetchedAt },
  };
  return { coop: { ...coop, deviceCount: devices.length }, devices, today: dayView(0), tomorrow: dayView(1), mode, states };
}

let notif = { id: 'n1', userId: 'u1', telegramAccounts: [{ id: 'a1', name: 'Mon Telegram', botToken: '123:abc', chatId: '42' }], notifyDailySchedule: true, notifyOnOpen: true, notifyOnClose: false, notifyOnLight: true, notifyOnError: true };

const logs = () => [
  { id: 'l1', deviceId: MAIN, actionType: 'open', status: 'success', triggeredBy: 'manual', userId: 'u1', executedAt: new Date(now() - 3600000).toISOString() },
  { id: 'l2', deviceId: MAIN, actionType: 'on', status: 'error', errorMessage: 'Omlet: 503 Service Unavailable', triggeredBy: 'auto', executedAt: new Date(now() - 5 * 3600000).toISOString() },
  { id: 'l3', deviceId: NEST, actionType: 'configuration', status: 'success', triggeredBy: 'sync', note: tc('horaires 08:30 / 18:14', 'times 08:30 / 18:14'), executedAt: new Date(now() - 20 * 3600000).toISOString() },
  { id: 'l4', deviceId: NEST, actionType: 'open', status: 'success', triggeredBy: 'fallback', note: tc("le boîtier n'a pas exécuté son horaire", 'the unit did not run its schedule'), executedAt: new Date(now() - 30 * 3600000).toISOString() },
  { id: 'l5', deviceId: MAIN, actionType: 'close', status: 'success', triggeredBy: 'catchup', note: tc('rattrapage', 'catch-up'), executedAt: new Date(now() - 50 * 3600000).toISOString() },
];

function preview(req, rule, devId) {
  const hm = (s) => s && /^\d{2}:\d{2}$/.test(s);
  for (const k of ['open', 'close']) {
    const label = k === 'open' ? tr(req, 'Ouverture', 'Opening') : tr(req, 'Fermeture', 'Closing');
    if (rule[`${k}Anchor`] === 'fixed' && !rule[`${k}FixedTime`]) return [400, { error: tr(req, `${label} : heure fixe manquante`, `${label}: fixed time missing`) }];
    if (rule[`${k}Anchor`]?.startsWith('device_') && !rule[`${k}RefDeviceId`]) return [400, { error: tr(req, `${label} : appareil de référence manquant`, `${label}: reference device missing`) }];
    if (Math.abs(rule[`${k}OffsetMinutes`]) > 720) return [400, { error: tr(req, `${label} : décalage hors limites (±720 min)`, `${label}: offset out of range (±720 min)`) }];
    const nb = rule[`${k}NotBefore`], na = rule[`${k}NotAfter`];
    if (nb === '' || na === '') return [400, { error: tr(req, 'Règle illisible : heure invalide ""', 'Unreadable rule: invalid time ""') }];
    if (hm(nb) && hm(na) && nb > na) return [400, { error: tr(req, `${label} : « pas avant » est après « pas après »`, `${label}: “not before” is after “not after”`) }];
  }
  const out = [];
  for (let i = 0; i < 7; i++) {
    const day = dayKey(new Date(now() + i * 86400000));
    const base = (anchor, off, fixed) => {
      let hhmm = anchor === 'sunrise' ? '07:53' : anchor === 'sunset' ? '19:12' : anchor === 'fixed' ? fixed : anchor === 'device_open' ? '07:43' : '19:32';
      const [h, m] = hhmm.split(':').map(Number);
      const t = h * 60 + m + off + i * (anchor === 'sunset' ? -2 : anchor === 'sunrise' ? 2 : 0);
      return `${String(Math.floor(t / 60)).padStart(2, '0')}:${String(t % 60).padStart(2, '0')}`;
    };
    let o = base(rule.openAnchor, rule.openOffsetMinutes, rule.openFixedTime);
    let c = base(rule.closeAnchor, rule.closeOffsetMinutes, rule.closeFixedTime);
    if (rule.openNotBefore && o < rule.openNotBefore) o = rule.openNotBefore;
    if (rule.openNotAfter && o > rule.openNotAfter) o = rule.openNotAfter;
    if (rule.closeNotBefore && c < rule.closeNotBefore) c = rule.closeNotBefore;
    if (rule.closeNotAfter && c > rule.closeNotAfter) c = rule.closeNotAfter;
    const events = [];
    const dev = devices.find((d) => d.id === devId);
    const error = c <= o ? tc(`fermeture (${c}) avant l'ouverture (${o}) : aucune action prévue ce jour`, `closing (${c}) before opening (${o}): nothing scheduled that day`) : undefined;
    if (!error && dev.enabled && dev.strategy !== 'monitor') {
      const light = dev.hasLight && dev.strategy === 'command';
      const add = (a, hhmm, delta) => { const [h, m] = hhmm.split(':').map(Number); const t = h * 60 + m + delta; events.push({ action: a, dueAt: at(day, `${String(Math.floor(t / 60)).padStart(2, '0')}:${String(t % 60).padStart(2, '0')}`) }); };
      if (light && rule.enableLightMorning && rule.lightBeforeOpenMinutes > 0) add('light_before_open', o, -rule.lightBeforeOpenMinutes);
      add('open', o, 0);
      if (light && rule.enableLightMorning && rule.lightOffDelayMinutes > 0) add('light_after_open', o, rule.lightOffDelayMinutes);
      if (light && rule.enableLightEvening && rule.lightBeforeCloseMinutes > 0) add('light_before_close', c, -rule.lightBeforeCloseMinutes);
      add('close', c, 0);
      if (light && rule.enableLightEvening && rule.lightOffDelayMinutes > 0) add('light_after_close', c, rule.lightOffDelayMinutes);
    }
    out.push({ day, open: at(day, o), close: at(day, c), events, ...(error ? { error } : {}) });
  }
  return [200, out];
}

/** Champs du poulailler (PUT et POST /coops), validés comme le serveur ; renvoie un message d'erreur ou null. */
function applyCoop(req, target, body, creating) {
  if (body.name !== undefined || creating) {
    const name = String(body.name || '').trim();
    if (!name) return tr(req, 'Le nom ne peut pas être vide', 'The name cannot be empty');
    target.name = name;
  }
  if (body.latitude !== undefined) {
    if (typeof body.latitude !== 'number' || body.latitude < -90 || body.latitude > 90) return tr(req, 'Latitude invalide', 'Invalid latitude');
    target.latitude = body.latitude;
  }
  if (body.longitude !== undefined) {
    if (typeof body.longitude !== 'number' || body.longitude < -180 || body.longitude > 180) return tr(req, 'Longitude invalide', 'Invalid longitude');
    target.longitude = body.longitude;
  }
  if (body.timezone !== undefined) {
    try {
      new Intl.DateTimeFormat('en', { timeZone: body.timezone });
    } catch {
      return tr(req, 'Fuseau horaire inconnu', 'Unknown time zone');
    }
    target.timezone = body.timezone;
  }
  if (body.language !== undefined) {
    if (body.language !== 'fr' && body.language !== 'en') return tr(req, 'Langue non prise en charge (fr ou en)', 'Unsupported language (fr or en)');
    target.language = body.language;
  }
  if (body.omletApiKey) {
    if (body.omletApiKey === 'bad') return tr(req, 'Clé API Omlet refusée : 401 Unauthorized', 'Omlet API key rejected: 401 Unauthorized');
    target.hasApiKey = true;
  }
  return null;
}

function send(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(body === undefined ? '' : JSON.stringify(body));
}

http.createServer((req, res) => {
  let raw = '';
  req.on('data', (c) => (raw += c));
  req.on('end', () => {
    const url = new URL(req.url, 'http://x');
    const p = url.pathname.replace(/^\/api\/v2/, '');
    const body = raw ? JSON.parse(raw) : {};
    const auth = req.headers.authorization === 'Bearer mock-token';
    console.log(req.method, url.pathname + url.search, `[${langOf(req)}]`, raw ? raw.slice(0, 300) : '');
    let m;
    if (req.method === 'GET' && p === '/system/bootstrap') {
      const needsSetup = FRESH && !registered;
      return send(res, 200, { needsSetup, registrationOpen: needsSetup, version: 'v2.0.0-mock' });
    }
    // Routes d'authentification : messages restés en anglais côté serveur (traduits par le frontend).
    if (req.method === 'POST' && p === '/auth/login') {
      const ok = body.password === 'secret' || (registered && body.email === registered.email && body.password === registered.password);
      return ok ? send(res, 200, { token: 'mock-token', user: registered ? registered.user : user }) : send(res, 401, { error: 'Invalid credentials' });
    }
    if (req.method === 'POST' && p === '/auth/register') {
      if (!FRESH || registered) return send(res, 403, { error: tr(req, 'Les inscriptions sont fermées sur ce serveur', 'Sign-ups are closed on this server') });
      registered = { email: body.email, password: body.password, user: { ...user, email: body.email, firstName: body.firstName, lastName: body.lastName } };
      return send(res, 201, { message: 'User registered successfully' });
    }
    if (req.method === 'GET' && p === '/auth/verify-email') return url.searchParams.get('token') === 'good' ? send(res, 200, { message: 'Email verified successfully! You can now log in.' }) : send(res, 404, { error: 'Invalid or expired verification token' });
    if (req.method === 'POST' && p === '/auth/resend-verification') return send(res, 200, { message: 'If the email exists, a verification link has been sent' });
    if (!auth) return send(res, 401, { error: 'Invalid or expired token' });
    if (p === '/auth/me') return send(res, 200, registered ? registered.user : user);
    if (p === '/system') return send(res, 200, { mode, version: 'v2.0.0-mock', now: new Date(now()).toISOString(), lastTick: new Date(now() - 20000).toISOString(), lastPlan: new Date(now() - 6 * 3600000).toISOString(), startedAt: new Date(now() - 86400000).toISOString() });
    if (req.method === 'GET' && p === '/coops') return send(res, 200, coop ? [{ ...coop, deviceCount: devices.length }] : []);
    if (req.method === 'POST' && p === '/coops') {
      if (body.latitude === undefined || body.longitude === undefined || !body.timezone || !String(body.omletApiKey || '').trim()) {
        return send(res, 400, { error: tr(req, 'Nom, position, fuseau horaire et clé API Omlet sont obligatoires', 'Name, location, time zone and Omlet API key are required') });
      }
      const created = { id: COOP_ID, omletGroupId: 'grp-1', hasApiKey: false, deviceCount: 0, language: langOf(req) };
      const error = applyCoop(req, created, body, true);
      if (error) return send(res, 400, { error });
      coop = created;
      devices = [];
      return send(res, 201, detail());
    }
    if ((m = p.match(/^\/coops\/([^/]+)(\/.*)?$/)) && (!coop || m[1] !== COOP_ID)) {
      return send(res, 404, { error: tr(req, 'Poulailler introuvable', 'Coop not found') });
    }
    if ((m = p.match(/^\/coops\/([^/]+)$/))) {
      if (req.method === 'PUT') {
        const draft = { ...coop };
        const error = applyCoop(req, draft, body, false);
        if (error) return send(res, 400, { error });
        coop = draft;
      }
      return send(res, 200, detail());
    }
    if ((m = p.match(/^\/coops\/([^/]+)\/plan$/))) return send(res, 200, dayView(0));
    if ((m = p.match(/^\/coops\/([^/]+)\/omlet-devices$/))) {
      return send(res, 200, [
        { deviceId: 'omlet-main', name: 'Autodoor 1', deviceType: 'Autodoor', groupId: 'grp-1', sameGroup: true, alreadyAdded: devices.some((d) => d.omletDeviceId === 'omlet-main'), powerSource: 'external', hasLight: true, doorState: 'open' },
        { deviceId: 'omlet-nest', name: 'Autodoor 2', deviceType: 'Autodoor', groupId: 'grp-1', sameGroup: true, alreadyAdded: devices.some((d) => d.omletDeviceId === 'omlet-nest'), powerSource: 'battery', hasLight: false, doorState: 'closed' },
        { deviceId: 'omlet-new', name: 'Porte du parc', deviceType: 'Autodoor', groupId: 'grp-2', sameGroup: false, alreadyAdded: devices.some((d) => d.omletDeviceId === 'omlet-new'), powerSource: 'battery', hasLight: false, doorState: 'closed' },
        { deviceId: 'omlet-feeder2', name: 'Mangeoire 2', deviceType: 'Feeder', groupId: 'grp-1', sameGroup: true, alreadyAdded: devices.some((d) => d.omletDeviceId === 'omlet-feeder2'), powerSource: 'battery', hasLight: false },
      ]);
    }
    if (req.method === 'POST' && (m = p.match(/^\/coops\/([^/]+)\/devices$/))) {
      if (devices.some((d) => d.omletDeviceId === body.omletDeviceId)) return send(res, 409, { error: tr(req, 'Cet appareil est déjà ajouté', 'This device is already added') });
      const main = devices.find((d) => d.role === 'main_door');
      if (body.role === 'main_door' && main) return send(res, 409, { error: tr(req, `Ce poulailler a déjà une porte principale (${main.name})`, `This coop already has a main door (${main.name})`) });
      const id = `55555555-5555-4555-8555-${String(now()).slice(-12)}`;
      const isFeeder = body.role === 'feeder';
      const dev = { id, coopId: COOP_ID, omletDeviceId: body.omletDeviceId, deviceType: isFeeder ? 'Feeder' : 'Autodoor', role: body.role, name: body.name || 'Nouveau', strategy: body.strategy || 'command', hasLight: body.omletDeviceId === 'omlet-main', enabled: true, position: devices.length, createdAt: new Date(now()).toISOString(), updatedAt: new Date(now()).toISOString(),
        rule: isFeeder ? null : body.role === 'nest_box' ? nestDefaultRule(id, main && main.id) : defaultRule(id) };
      devices.push(dev);
      return send(res, 201, dev);
    }
    if ((m = p.match(/^\/devices\/([^/]+)(\/.*)?$/))) {
      const dev = devices.find((d) => d.id === m[1]);
      if (!dev) return send(res, 404, { error: tr(req, 'Appareil introuvable', 'Device not found') });
      const sub = m[2] || '';
      if (sub === '' && req.method === 'PUT') {
        const main = devices.find((d) => d.role === 'main_door' && d.id !== dev.id);
        if (body.role === 'main_door' && main) return send(res, 409, { error: tr(req, `Ce poulailler a déjà une porte principale (${main.name})`, `This coop already has a main door (${main.name})`) });
        Object.assign(dev, Object.fromEntries(Object.entries(body).filter(([k]) => ['name', 'role', 'strategy', 'enabled', 'position'].includes(k))));
        return send(res, 200, dev);
      }
      if (sub === '' && req.method === 'DELETE') {
        const refs = devices.find((d) => d.id !== dev.id && d.rule && (d.rule.openRefDeviceId === dev.id || d.rule.closeRefDeviceId === dev.id));
        if (refs) return send(res, 409, { error: tr(req, `« ${refs.name} » utilise cet appareil comme référence : modifie d'abord sa règle`, `“${refs.name}” uses this device as a reference: change its rule first`) });
        devices = devices.filter((d) => d.id !== dev.id);
        res.writeHead(204); return res.end();
      }
      if (sub === '/status') {
        if (dev.role === 'feeder') return send(res, 200, { name: dev.name, deviceType: 'Feeder', groupId: 'grp-1', door: null, light: null, feeder: { state: 'closed', lastOpenTime: '2026-10-06 06:00:00', lastCloseTime: '', fault: 'none', feedLevel: 64 }, batteryLevel: 15, powerSource: 'battery', firmware: '1.0.2', wifiStrength: -78, connected: false });
        const doorState = DEMO || dev.id === MAIN ? 'open' : dev.id === NEST ? 'closepending' : 'closed';
        return send(res, 200, { name: dev.name, deviceType: 'Autodoor', groupId: 'grp-1', door: { state: doorState, lastOpenTime: new Date(now() - 9 * 3600000).toISOString(), lastCloseTime: new Date(now() - 22 * 3600000).toISOString(), fault: !DEMO && dev.id === NEST ? 'blocked' : 'none', lightLevel: 30 }, light: dev.hasLight ? { state: 'off' } : null, feeder: null, batteryLevel: 87, powerSource: dev.id === MAIN ? 'external' : 'battery', firmware: '1.4.0', wifiStrength: -61, connected: true });
      }
      if (sub.startsWith('/actions/')) return sub.endsWith('/stop') && dev.id === NEST ? send(res, 502, { error: tr(req, 'Action refusée par Omlet : device offline', 'Omlet refused the command: device offline') }) : send(res, 200, { message: 'Action envoyée' });
      if (sub === '/logs') return send(res, 200, logs().filter((l) => l.deviceId === dev.id).slice(0, Number(url.searchParams.get('limit') || 50)));
      if (sub === '/rule' && req.method === 'GET') return dev.rule ? send(res, 200, dev.rule) : send(res, 404, { error: tr(req, "Cet appareil n'a pas de règle", 'This device has no rule') });
      if (sub === '/rule' && req.method === 'PUT') {
        const [st, out] = preview(req, body, dev.id);
        if (st !== 200) return send(res, st, out);
        dev.rule = { ...body, id: dev.rule?.id || 'r-new', deviceId: dev.id };
        return send(res, 200, dev.rule);
      }
      if (sub === '/rule/preview') { const [st, out] = preview(req, body, dev.id); return send(res, st, out); }
    }
    // Routes des notifications : messages restés en anglais côté serveur (traduits par le frontend).
    if (p === '/settings/notifications' && req.method === 'GET') return send(res, 200, notif);
    if (p === '/settings/notifications' && req.method === 'PUT') { notif = { ...notif, ...body }; return send(res, 200, notif); }
    if (p === '/settings/notifications/test') return body.telegramChatId === 'bad' ? send(res, 500, { error: 'Failed to send test notification', details: 'Bad Request: chat not found' }) : send(res, 200, { message: 'Test notification sent successfully' });
    send(res, 404, { error: tr(req, 'Route inconnue (mock)', 'Unknown route (mock)') });
  });
}).listen(PORT, '127.0.0.1', () => console.log(`mock API on :${PORT}${FRESH ? ' (premier démarrage)' : ''}`));
