// Logique d'affichage propre au poulailler : accords, état d'une porte, prochaines actions, phrase d'accueil.
// Les phrases sont dans locales/ ; l'accord en genre ({ f, m }) ne sert qu'au français.
import { t } from '@/lib/i18n';
import { faultLabel } from '@/lib/labels';
import { dayKey, formatDateShort, formatTime, parseDate } from '@/lib/time';
import type { DayView, Device, DeviceStateView, PlannedEvent } from '@/types';

export type Gender = 'f' | 'm';

/** « Le pondoir » est masculin ; la porte et la mangeoire sont féminines. */
export const genderOf = (d: Pick<Device, 'role'>): Gender => (d.role === 'nest_box' ? 'm' : 'f');

/** Sujet d'une phrase : « La porte », « Le pondoir », « La mangeoire », ou le nom d'une autre porte. */
export const subjectOf = (d: Pick<Device, 'role' | 'name'>): string => t(`device.subject.${d.role}`, { name: d.name });

/** Complément : « de la porte », « du pondoir »… (en anglais : "Door", "Nest box"…). */
export const ofDevice = (d: Pick<Device, 'role' | 'name'>): string => t(`device.of.${d.role}`, { name: d.name });

/** Complément d'objet : « la porte », « le pondoir »… (« Ouvrir la porte maintenant ? »). */
export const objectOf = (d: Pick<Device, 'role' | 'name'>): string =>
  t(`device.object.${d.role}`, { name: d.name, nameLower: d.name.toLowerCase() });

export type DoorPhase = 'open' | 'closed' | 'opening' | 'closing' | 'stopped' | 'fault' | 'unknown';

export function doorPhase(door?: string | null, fault?: string | null): DoorPhase {
  if (faultLabel(fault)) return 'fault';
  switch ((door ?? '').toLowerCase()) {
    case 'open':
      return 'open';
    case 'closed':
      return 'closed';
    case 'opening':
    case 'openpending':
      return 'opening';
    case 'closing':
    case 'closepending':
      return 'closing';
    case 'stopped':
    case 'stopping':
      return 'stopped';
    case 'faulted':
      return 'fault';
    default:
      return 'unknown';
  }
}

export const phaseOfState = (s?: DeviceStateView | null): DoorPhase => doorPhase(s?.door, s?.fault);

/** Mot court accordé, pour une liste : « Fermée », « Ouvert »… */
export const phaseShort = (phase: DoorPhase, g: Gender): string => t(`coop.phase.${phase}`, { gender: g });

/** Phrase d'état : « La porte est fermée. », « Le pondoir s'ouvre. », « Le pondoir est bloqué. »… */
export const phaseSentence = (d: Pick<Device, 'role' | 'name'>, phase: DoorPhase): string =>
  t(`coop.sentence.${phase}`, { subject: subjectOf(d), gender: genderOf(d) });

/** Teinte du point d'état. */
export const phaseTone = (phase: DoorPhase): 'sun' | 'ink' | 'alert' | 'muted' =>
  phase === 'open' || phase === 'opening' ? 'sun' : phase === 'closed' || phase === 'closing' ? 'ink' : phase === 'unknown' ? 'muted' : 'alert';

const ONE_DAY = 24 * 60 * 60 * 1000;

/** Prochaine ouverture ou fermeture encore à venir d'un appareil (aujourd'hui puis demain). */
export function nextEvent(deviceId: string, action: 'open' | 'close', days: DayView[], now: number): PlannedEvent | undefined {
  return days
    .flatMap((d) => d.events)
    .filter((e) => e.deviceId === deviceId && e.action === action && e.status === 'pending' && Date.parse(e.dueAt) >= now - 60_000)
    .sort((a, b) => Date.parse(a.dueAt) - Date.parse(b.dueAt))[0];
}

/** « à 07:46 », « demain à 07:46 », « le jeu. 8 oct. à 07:46 ». */
export function whenText(e: Pick<PlannedEvent, 'dueAt'>, tz: string, now: number): string {
  const day = dayKey(e.dueAt, tz);
  const time = formatTime(e.dueAt, tz);
  if (day === dayKey(new Date(now), tz)) return t('time.at', { time });
  if (day === dayKey(new Date(now + ONE_DAY), tz)) return t('time.tomorrowAt', { time });
  return t('time.onDateAt', { date: formatDateShort(e.dueAt, tz), time });
}

/** Échec le plus récent du jour pour un appareil (porte non confirmée, défaut). */
export function lastFailure(deviceId: string, day: DayView): PlannedEvent | undefined {
  return day.events
    .filter((e) => e.deviceId === deviceId && e.status === 'failed' && (e.action === 'open' || e.action === 'close'))
    .sort((a, b) => Date.parse(b.dueAt) - Date.parse(a.dueAt))[0];
}

/** Minutes écoulées depuis minuit, heure locale du poulailler. */
export function minutesOfDay(value: string | Date, tz: string): number {
  const d = parseDate(value);
  if (!d) return NaN;
  const parts = new Intl.DateTimeFormat('fr-FR', { timeZone: tz, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(d);
  const get = (t: string) => Number(parts.find((p) => p.type === t)?.value ?? 0);
  return get('hour') * 60 + get('minute');
}

export interface HeroModel {
  tone: 'calm' | 'alert' | 'unknown';
  title: string;
  detail: string;
  action?: { kind: 'open' | 'close'; label: string };
}

/** Phrase d'accueil : l'état de la porte principale et ce qui va se passer ensuite. */
export function heroFor(main: Device | undefined, state: DeviceStateView | null | undefined, days: DayView[], tz: string, now: number): HeroModel {
  if (!main) {
    return { tone: 'unknown', title: t('hero.noMainTitle'), detail: t('hero.noMainDetail') };
  }
  const phase = phaseOfState(state);
  const g = genderOf(main);
  const subject = subjectOf(main);
  const nextOpen = nextEvent(main.id, 'open', days, now);
  const nextClose = nextEvent(main.id, 'close', days, now);
  // Ce que la porte devrait être maintenant : fermée si la prochaine action est une ouverture.
  const shouldBe: 'open' | 'close' =
    nextOpen && (!nextClose || Date.parse(nextOpen.dueAt) < Date.parse(nextClose.dueAt)) ? 'close' : 'open';
  const openDoor = { kind: 'open' as const, label: t('hero.openDoor') };
  const closeDoor = { kind: 'close' as const, label: t('hero.closeDoor') };
  const fix = shouldBe === 'close' ? closeDoor : openDoor;

  const failure = days[0] ? lastFailure(main.id, days[0]) : undefined;
  if (failure && now - Date.parse(failure.dueAt) < 12 * 60 * 60 * 1000) {
    return {
      tone: 'alert',
      title: t(failure.action === 'close' ? 'hero.closeFailed' : 'hero.openFailed', { time: formatTime(failure.dueAt, tz) }),
      detail: failure.note || failure.lastError || t('hero.checkOnSite'),
      action: failure.action === 'close' ? closeDoor : openDoor,
    };
  }
  if (phase === 'fault') {
    return {
      tone: 'alert',
      title: phaseSentence(main, 'fault'),
      detail: t('hero.faultDetail', { fault: faultLabel(state?.fault) ?? t('hero.faultReported') }),
      action: fix,
    };
  }
  if (!main.enabled) {
    return {
      tone: 'unknown',
      title: phase === 'unknown' ? t('hero.notAutomated', { subject, gender: g }) : phaseSentence(main, phase),
      detail: t('hero.pausedDetail'),
    };
  }
  switch (phase) {
    case 'closed':
      return {
        tone: 'calm',
        title: phaseSentence(main, 'closed'),
        detail: nextOpen ? t('coop.opensAt', { when: whenText(nextOpen, tz, now) }) : t('hero.noOpening'),
        action: { kind: 'open', label: t('hero.openNow') },
      };
    case 'open':
      return {
        tone: 'calm',
        title: phaseSentence(main, 'open'),
        detail: nextClose ? t('coop.closesAt', { when: whenText(nextClose, tz, now) }) : t('hero.noClosing'),
        action: { kind: 'close', label: t('hero.closeNow') },
      };
    case 'opening':
    case 'closing':
      return { tone: 'calm', title: phaseSentence(main, phase), detail: t('hero.moving') };
    case 'stopped':
      return { tone: 'alert', title: t('hero.stoppedHalfway', { subject, gender: g }), detail: t('hero.stoppedDetail'), action: fix };
    default:
      return {
        tone: 'unknown',
        title: t('hero.unknownTitle'),
        detail: state?.error ? t('hero.omletDown') : t('hero.notReadYet'),
      };
  }
}

/** État en direct (GET /devices/:id/status) ramené au format des états mis en cache. */
export function stateFromStatus(s: import('@/types').DeviceStatus): DeviceStateView {
  const part = s.door ?? s.feeder;
  return {
    door: part?.state,
    fault: part?.fault,
    light: s.light?.state,
    battery: s.batteryLevel,
    power: s.powerSource,
    connected: s.connected,
    lastOpen: part?.lastOpenTime,
    lastClose: part?.lastCloseTime,
    feedLevel: s.feeder?.feedLevel,
    asleep: s.asleep,
    nextWake: s.nextWake,
    overdue: s.overdue,
    fetchedAt: new Date().toISOString(),
  };
}
