// Captures d'écran du README (docs/screenshots/{home,rule}-{fr,en}.png) avec Chrome sans interface.
//   1. MOCK_SCENE=demo MOCK_NOW=16:20 bash dev/start-dev.sh   (dans un autre terminal)
//   2. node dev/screenshots.mjs
// Aucune dépendance : Chrome est piloté par le protocole DevTools (Node 22+).
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const BASE = process.env.BASE_URL || 'http://localhost:3000';
const NOW = process.env.MOCK_NOW || '16:20'; // même heure que le mock
const CHROME = process.env.CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const OUT = join(dirname(fileURLToPath(import.meta.url)), '../../docs/screenshots');
const TOKEN = 'mock-token';
const COOP = '11111111-1111-4111-8111-111111111111';
const MAIN = '22222222-2222-4222-8222-222222222222';
const NEST = '33333333-3333-4333-8333-333333333333';
const FEEDER = '44444444-4444-4444-8444-444444444444';

const LANGS = {
  fr: { coop: 'Poulailler du jardin', names: { [MAIN]: 'Porte principale', [NEST]: 'Pondoir', [FEEDER]: 'Mangeoire' } },
  en: { coop: 'Garden coop', names: { [MAIN]: 'Main door', [NEST]: 'Nest box', [FEEDER]: 'Feeder' } },
};
// ready : texte attendu une fois la page chargée. fit : la hauteur d'écran est ajustée pour que la barre
// d'onglets commence juste sous la liste des appareils (aucun texte coupé) ; la capture suivante la garde.
const SHOTS = [
  { name: 'home', path: `/coops/${COOP}`, ready: (l) => LANGS[l].names[FEEDER], fit: true },
  { name: 'rule', path: `/devices/${NEST}/settings`, ready: (l) => LANGS[l].names[MAIN] },
];
const WIDTH = 390;
const HEIGHT = 844;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Instant « aujourd'hui à HH:MM, heure de Paris ». */
function parisEpoch(hhmm) {
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Paris', year: 'numeric', month: '2-digit', day: '2-digit', timeZoneName: 'longOffset' })
      .formatToParts(new Date())
      .map((p) => [p.type, p.value]),
  );
  const offset = parts.timeZoneName.replace('GMT', '') || '+00:00';
  return Date.parse(`${parts.year}-${parts.month}-${parts.day}T${hhmm}:00${offset}`);
}

async function api(method, path, body, lang) {
  const res = await fetch(`${BASE}/api/v2${path}`, {
    method,
    headers: { Authorization: `Bearer ${TOKEN}`, 'Content-Type': 'application/json', 'Accept-Language': lang },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`${method} ${path} : ${res.status} ${await res.text()}`);
  return res.json().catch(() => null);
}

async function launchChrome() {
  if (!existsSync(CHROME)) throw new Error(`Chrome introuvable (${CHROME}) : définir CHROME=/chemin/vers/chrome`);
  const profile = mkdtempSync(join(tmpdir(), 'solar-chicken-shots-'));
  const proc = spawn(CHROME, [
    '--headless=new', '--remote-debugging-port=0', `--user-data-dir=${profile}`, '--no-first-run',
    '--no-default-browser-check', '--hide-scrollbars', '--force-color-profile=srgb', 'about:blank',
  ], { stdio: 'ignore' });
  const portFile = join(profile, 'DevToolsActivePort');
  for (let i = 0; i < 100 && !existsSync(portFile); i++) await sleep(100);
  const port = readFileSync(portFile, 'utf8').split('\n')[0];
  const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
  const page = targets.find((t) => t.type === 'page');
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });
  let seq = 0;
  const pending = new Map();
  const listeners = new Map();
  ws.onmessage = ({ data }) => {
    const msg = JSON.parse(data);
    if (msg.id && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      msg.error ? reject(new Error(`${msg.error.message}`)) : resolve(msg.result);
    } else if (msg.method && listeners.has(msg.method)) {
      listeners.get(msg.method).splice(0).forEach((fn) => fn(msg.params));
    }
  };
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++seq;
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, method, params }));
  });
  const once = (method) => new Promise((resolve) => {
    if (!listeners.has(method)) listeners.set(method, []);
    listeners.get(method).push(resolve);
  });
  const close = () => { ws.close(); proc.kill(); setTimeout(() => rmSync(profile, { recursive: true, force: true }), 500); };
  return { send, once, close };
}

async function main() {
  const shift = parisEpoch(NOW) - Date.now();
  const chrome = await launchChrome();
  const { send, once } = chrome;
  try {
    await send('Page.enable');
    await send('Runtime.enable');
    const viewport = (height) => send('Emulation.setDeviceMetricsOverride', { width: WIDTH, height, deviceScaleFactor: 2, mobile: true });
    await viewport(HEIGHT);
    await send('Emulation.setEmulatedMedia', { features: [
      { name: 'prefers-color-scheme', value: 'light' },
      { name: 'prefers-reduced-motion', value: 'reduce' },
    ] });
    await send('Emulation.setTimezoneOverride', { timezoneId: 'Europe/Paris' });
    // Horloge de la page alignée sur celle du mock ; indicateurs de développement de Next.js masqués.
    await send('Page.addScriptToEvaluateOnNewDocument', { source: `(() => {
      const R = Date, SHIFT = ${shift};
      class D extends R { constructor(...a) { if (a.length) super(...a); else super(R.now() + SHIFT); } static now() { return R.now() + SHIFT; } }
      globalThis.Date = D;
      addEventListener('DOMContentLoaded', () => {
        const s = document.createElement('style');
        s.textContent = 'nextjs-portal { display: none !important; }';
        document.head.appendChild(s);
      });
    })();` });

    const go = async (path) => {
      const loaded = once('Page.loadEventFired');
      await send('Page.navigate', { url: BASE + path });
      await loaded;
    };
    const evaluate = async (expression) =>
      (await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })).result.value;

    await go('/login');
    mkdirSync(OUT, { recursive: true });
    for (const [lang, data] of Object.entries(LANGS)) {
      await api('PUT', `/coops/${COOP}`, { name: data.coop, language: lang }, lang);
      for (const [id, name] of Object.entries(data.names)) await api('PUT', `/devices/${id}`, { name }, lang);
      const user = { id: 'u1', email: 'demo@example.com', firstName: 'Demo', lastName: '', createdAt: '2025-01-01T00:00:00Z', updatedAt: '2025-01-01T00:00:00Z' };
      await evaluate(`localStorage.setItem('token', '${TOKEN}');
        localStorage.setItem('auth-storage', ${JSON.stringify(JSON.stringify({ state: { user, token: TOKEN }, version: 0 }))});
        localStorage.setItem('locale', '${lang}'); true`);
      await viewport(HEIGHT);

      for (const shot of SHOTS) {
        await go(shot.path);
        const expected = JSON.stringify(shot.ready(lang));
        const ok = await evaluate(`new Promise((resolve) => {
          const t0 = performance.now();
          const tick = () => document.body.innerText.includes(${expected}) ? resolve(true)
            : performance.now() - t0 > 30000 ? resolve(false) : setTimeout(tick, 200);
          tick();
        })`);
        if (!ok) throw new Error(`${shot.path} (${lang}) : « ${shot.ready(lang)} » jamais affiché`);
        await evaluate('document.fonts.ready.then(() => true)');
        if (shot.fit) {
          const height = await evaluate(`(() => {
            const list = document.querySelector('a[href^="/devices/"]')?.closest('ul');
            const tabs = document.querySelector('nav.fixed');
            return list && tabs ? Math.ceil(list.getBoundingClientRect().bottom + window.scrollY + 24 + tabs.offsetHeight) : 0;
          })()`);
          if (height < 600 || height > 1000) throw new Error(`hauteur d'écran inattendue (${height}) : la mise en page a changé ?`);
          await viewport(height);
        }
        await sleep(800);
        const { data: png } = await send('Page.captureScreenshot', { format: 'png' });
        const file = join(OUT, `${shot.name}-${lang}.png`);
        writeFileSync(file, Buffer.from(png, 'base64'));
        console.log(`✓ ${file}`);
      }
    }
    // Remet le mock dans son état français d'origine.
    await api('PUT', `/coops/${COOP}`, { name: LANGS.fr.coop, language: 'fr' }, 'fr');
    for (const [id, name] of Object.entries(LANGS.fr.names)) await api('PUT', `/devices/${id}`, { name }, 'fr');
  } finally {
    chrome.close();
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
