// Outils autour des règles d'ouverture / fermeture.
import type { Anchor, Device, HHMM, Rule } from '@/types';
import { t } from '@/lib/i18n';
import { addMinutesToHHMM, formatDuration } from '@/lib/time';

export type MomentKind = 'open' | 'close';

/** Une ouverture ou une fermeture : repère + décalage, bornes facultatives. */
export interface Moment {
  anchor: Anchor;
  offsetMinutes: number;
  fixedTime: HHMM | null;
  refDeviceId: string | null;
  notBefore: HHMM | null;
  notAfter: HHMM | null;
}

export const OFFSET_LIMIT = 720; // ±12 h, comme le serveur
export const LIGHT_LIMIT = 120; // 0 à 120 min, comme le serveur

/** Règle proposée pour une porte sans règle (identique au serveur). */
export const DEFAULT_RULE: Rule = {
  openAnchor: 'sunrise',
  openOffsetMinutes: -10,
  openFixedTime: null,
  openRefDeviceId: null,
  openNotBefore: null,
  openNotAfter: null,
  closeAnchor: 'sunset',
  closeOffsetMinutes: 20,
  closeFixedTime: null,
  closeRefDeviceId: null,
  closeNotBefore: null,
  closeNotAfter: null,
  lightBeforeOpenMinutes: 0,
  lightBeforeCloseMinutes: 0,
  enableLightMorning: false,
  enableLightEvening: false,
  lightOffDelayMinutes: 0,
};

export const isDeviceAnchor = (anchor: Anchor) => anchor === 'device_open' || anchor === 'device_close';

/** Porte : seule une porte sert de repère à la règle d'un autre appareil. */
export const isDoor = (device: Pick<Device, 'role'>) => device.role !== 'feeder';

/**
 * Règle proposée à un appareil qui n'en a pas encore, comme le serveur : le pondoir s'ouvre avec la porte
 * principale et se ferme 2 h avant elle, la mangeoire s'ouvre avec elle et se ferme au coucher du soleil.
 */
export function defaultRuleFor(device: Pick<Device, 'role'>, mainDoorId: string | null): Rule {
  const withMain = (patch: Partial<Rule>): Rule =>
    mainDoorId ? { ...DEFAULT_RULE, openAnchor: 'device_open', openOffsetMinutes: 0, openRefDeviceId: mainDoorId, ...patch } : DEFAULT_RULE;
  if (device.role === 'nest_box') {
    return mainDoorId
      ? withMain({ closeAnchor: 'device_close', closeOffsetMinutes: -120, closeRefDeviceId: mainDoorId })
      : { ...DEFAULT_RULE, closeOffsetMinutes: -100 };
  }
  if (device.role === 'feeder') {
    return mainDoorId ? withMain({ closeOffsetMinutes: 0 }) : { ...DEFAULT_RULE, closeOffsetMinutes: 0 };
  }
  return DEFAULT_RULE;
}

export function getMoment(rule: Rule, kind: MomentKind): Moment {
  return kind === 'open'
    ? {
        anchor: rule.openAnchor,
        offsetMinutes: rule.openOffsetMinutes,
        fixedTime: rule.openFixedTime,
        refDeviceId: rule.openRefDeviceId,
        notBefore: rule.openNotBefore,
        notAfter: rule.openNotAfter,
      }
    : {
        anchor: rule.closeAnchor,
        offsetMinutes: rule.closeOffsetMinutes,
        fixedTime: rule.closeFixedTime,
        refDeviceId: rule.closeRefDeviceId,
        notBefore: rule.closeNotBefore,
        notAfter: rule.closeNotAfter,
      };
}

export function withMoment(rule: Rule, kind: MomentKind, moment: Moment): Rule {
  return kind === 'open'
    ? {
        ...rule,
        openAnchor: moment.anchor,
        openOffsetMinutes: moment.offsetMinutes,
        openFixedTime: moment.fixedTime,
        openRefDeviceId: moment.refDeviceId,
        openNotBefore: moment.notBefore,
        openNotAfter: moment.notAfter,
      }
    : {
        ...rule,
        closeAnchor: moment.anchor,
        closeOffsetMinutes: moment.offsetMinutes,
        closeFixedTime: moment.fixedTime,
        closeRefDeviceId: moment.refDeviceId,
        closeNotBefore: moment.notBefore,
        closeNotAfter: moment.notAfter,
      };
}

const int = (value: number) => (Number.isFinite(value) ? Math.round(value) : 0);

function cleanMoment(moment: Moment): Moment {
  return {
    anchor: moment.anchor,
    offsetMinutes: int(moment.offsetMinutes),
    fixedTime: moment.anchor === 'fixed' ? moment.fixedTime || null : null,
    refDeviceId: isDeviceAnchor(moment.anchor) ? moment.refDeviceId || null : null,
    notBefore: moment.notBefore || null,
    notAfter: moment.notAfter || null,
  };
}

/**
 * Corps envoyé à l'API : uniquement les champs du contrat, heure fixe / porte de référence
 * remises à null quand le repère ne les utilise pas, "" → null (le serveur refuse "").
 */
export function toRulePayload(rule: Rule): Rule {
  const base: Rule = {
    ...DEFAULT_RULE,
    lightBeforeOpenMinutes: int(rule.lightBeforeOpenMinutes),
    lightBeforeCloseMinutes: int(rule.lightBeforeCloseMinutes),
    enableLightMorning: !!rule.enableLightMorning,
    enableLightEvening: !!rule.enableLightEvening,
    lightOffDelayMinutes: int(rule.lightOffDelayMinutes),
  };
  const withOpen = withMoment(base, 'open', cleanMoment(getMoment(rule, 'open')));
  return withMoment(withOpen, 'close', cleanMoment(getMoment(rule, 'close')));
}

export function rulesEqual(a: Rule, b: Rule): boolean {
  return JSON.stringify(toRulePayload(a)) === JSON.stringify(toRulePayload(b));
}

/**
 * Phrase décrivant un moment, ex. « 10 min avant le lever du soleil, jamais avant 08:00 »
 * ("30 min after the main door opens, never before 08:00"). `ref` : la porte de référence.
 */
export function describeMoment(moment: Moment, ref?: Pick<Device, 'name' | 'role'>): string {
  const { anchor, offsetMinutes: offset } = moment;
  const refText = ref
    ? t(ref.role === 'main_door' ? 'rule.refMainDoor' : 'rule.refNamed', { name: ref.name })
    : t('rule.refNone');
  let text: string;

  if (anchor === 'fixed') {
    if (!moment.fixedTime) {
      text = t('rule.fixedMissing');
    } else if (offset === 0) {
      text = t('rule.fixedAt', { time: moment.fixedTime });
    } else {
      text = t('rule.fixedOffset', {
        time: addMinutesToHHMM(moment.fixedTime, offset),
        fixed: moment.fixedTime,
        sign: offset > 0 ? '+' : '−',
        duration: formatDuration(offset),
      });
    }
  } else if (offset === 0) {
    text = t(`rule.at.${anchor}`, { ref: refText });
  } else {
    const base = t(`rule.base.${anchor}`, { ref: refText });
    text = t(offset < 0 ? 'rule.before' : 'rule.after', { duration: formatDuration(offset), base });
  }

  if (moment.notBefore) text += t('rule.notBefore', { time: moment.notBefore });
  if (moment.notAfter) text += t('rule.notAfter', { time: moment.notAfter });
  return text;
}
