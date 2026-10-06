// Affichage des dates et heures dans le fuseau du poulailler (Intl, aucune dépendance),
// dans la langue de l'interface ('fr-FR' ou 'en-GB', horloge sur 24 h).
// L'API renvoie des instants ISO 8601 (UTC) et des jours locaux "AAAA-MM-JJ".
import { intlLocale, t } from '@/lib/i18n';

const formatters = new Map<string, Intl.DateTimeFormat>();

function formatter(timeZone: string | undefined, options: Intl.DateTimeFormatOptions, locale = intlLocale()) {
  const key = `${locale}|${timeZone ?? ''}|${JSON.stringify(options)}`;
  let f = formatters.get(key);
  if (!f) {
    try {
      f = new Intl.DateTimeFormat(locale, { ...options, timeZone });
    } catch {
      // Fuseau inconnu du navigateur : on retombe sur le fuseau de l'appareil.
      f = new Intl.DateTimeFormat(locale, options);
    }
    formatters.set(key, f);
  }
  return f;
}

// "AAAA-MM-JJ HH:MM[:SS]" sans fuseau : Chrome le lit en heure locale, Safari le refuse.
const NAIVE_DATE_TIME = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?)$/;

/**
 * Lit une date ISO 8601. Une date sans fuseau est lue en UTC (comme toutes les dates de l'API),
 * de la même façon sur tous les navigateurs. null si vide ou invalide.
 */
export function parseDate(value: string | Date | null | undefined): Date | null {
  if (!value) return null;
  let date: Date;
  if (value instanceof Date) {
    date = value;
  } else {
    const naive = NAIVE_DATE_TIME.exec(value.trim());
    date = new Date(naive ? `${naive[1]}T${naive[2]}Z` : value);
  }
  if (Number.isNaN(date.getTime())) return null;
  // Date « zéro » de Go (0001-01-01T00:00:00Z) = jamais
  if (date.getUTCFullYear() <= 1) return null;
  return date;
}

/** "07:42" dans le fuseau donné, "—" si la date est absente. */
export function formatTime(value: string | Date | null | undefined, timeZone?: string): string {
  const date = parseDate(value);
  if (!date) return '—';
  return formatter(timeZone, { hour: '2-digit', minute: '2-digit' }).format(date);
}

/** Jour local "AAAA-MM-JJ" d'un instant dans le fuseau donné (clé technique, indépendante de la langue). */
export function dayKey(value: string | Date, timeZone?: string): string {
  const date = parseDate(value);
  if (!date) return '';
  const parts = formatter(timeZone, { year: 'numeric', month: '2-digit', day: '2-digit' }, 'fr-FR').formatToParts(date);
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? '';
  return `${get('year')}-${get('month')}-${get('day')}`;
}

// Un jour "AAAA-MM-JJ" est une date civile : on la place à midi UTC et on la formate en UTC.
function civilDay(day: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
  if (!m) return null;
  return new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 12));
}

/** "mardi 6 octobre" / "Tuesday 6 October" pour un jour "AAAA-MM-JJ". */
export function formatDayLong(day: string): string {
  const date = civilDay(day);
  if (!date) return day;
  return formatter('UTC', { weekday: 'long', day: 'numeric', month: 'long' }).format(date);
}

/** "mar. 6 oct." / "Tue, 6 Oct" pour un jour "AAAA-MM-JJ". */
export function formatDayShort(day: string): string {
  const date = civilDay(day);
  if (!date) return day;
  return formatter('UTC', { weekday: 'short', day: 'numeric', month: 'short' }).format(date);
}

/** "mar. 6 oct." pour un instant, dans le fuseau donné. */
export function formatDateShort(value: string | Date | null | undefined, timeZone?: string): string {
  const date = parseDate(value);
  if (!date) return '—';
  return formatter(timeZone, { weekday: 'short', day: 'numeric', month: 'short' }).format(date);
}

/** "mar. 6 oct. à 07:42" dans le fuseau donné. */
export function formatDateTime(value: string | Date | null | undefined, timeZone?: string): string {
  const date = parseDate(value);
  if (!date) return '—';
  return t('time.dateAt', { date: formatDateShort(date, timeZone), time: formatTime(date, timeZone) });
}

/** "aujourd'hui à 07:42", "hier à 19:10", "demain à 07:40" ou "lun. 5 oct. à 07:42". */
export function formatWhen(
  value: string | Date | null | undefined,
  timeZone?: string,
  now: number = Date.now()
): string {
  const date = parseDate(value);
  if (!date) return '—';
  const day = dayKey(date, timeZone);
  const time = formatTime(date, timeZone);
  const oneDay = 24 * 60 * 60 * 1000;
  if (day === dayKey(new Date(now), timeZone)) return t('time.todayAt', { time });
  if (day === dayKey(new Date(now - oneDay), timeZone)) return t('time.yesterdayAt', { time });
  if (day === dayKey(new Date(now + oneDay), timeZone)) return t('time.tomorrowAt', { time });
  return t('time.dateAt', { date: formatDateShort(date, timeZone), time });
}

/** Durée lisible : "45 min", "1 h 05", "2 h" (en anglais "1 h 5 min"). */
export function formatDuration(minutes: number): string {
  const total = Math.round(Math.abs(minutes));
  if (total < 60) return t('time.minutes', { m: total });
  const h = Math.floor(total / 60);
  const m = total % 60;
  return m ? t('time.hoursMinutes', { h, m, mm: String(m).padStart(2, '0') }) : t('time.hours', { h });
}

/** Relatif : "dans 2 h", "dans 12 min", "il y a 1 h 05", "maintenant". */
export function formatRelative(value: string | Date | null | undefined, now: number = Date.now()): string {
  const date = parseDate(value);
  if (!date) return '';
  const diff = Math.round((date.getTime() - now) / 60_000);
  const abs = Math.abs(diff);
  if (abs < 1) return t('time.now');
  const days = Math.round(abs / (24 * 60));
  const duration = abs < 24 * 60 ? formatDuration(abs) : t('time.days', { count: days });
  return diff > 0 ? t('time.in', { duration }) : t('time.ago', { duration });
}

/** "07:30" + 10 → "07:40" (sur 24 h). */
export function addMinutesToHHMM(hhmm: string, minutes: number): string {
  const m = /^(\d{1,2}):(\d{2})/.exec(hhmm);
  if (!m) return hhmm;
  const total = (((Number(m[1]) * 60 + Number(m[2]) + minutes) % 1440) + 1440) % 1440;
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}

/** Vrai si le navigateur connaît ce fuseau IANA (ex. "Europe/Paris"). */
export function isValidTimeZone(timeZone: string): boolean {
  if (!timeZone.trim()) return false;
  try {
    new Intl.DateTimeFormat('fr-FR', { timeZone });
    return true;
  } catch {
    return false;
  }
}

/** Première lettre en majuscule ("mardi 6 octobre" → "Mardi 6 octobre"). */
export function capitalize(text: string): string {
  return text ? text.charAt(0).toUpperCase() + text.slice(1) : text;
}
