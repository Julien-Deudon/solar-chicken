// Libellés des valeurs de l'API v2, dans la langue de l'interface (textes : locales/).
import { hasMessage, t, type MessageKey } from '@/lib/i18n';
import type { Anchor, DeviceRole, EventAction, EventStatus, Strategy } from '@/types';

// Clé vérifiée à la compilation pour chaque valeur connue du type ; une valeur inattendue
// venue du serveur s'affiche telle quelle.
const label = (key: MessageKey, raw: string) => (hasMessage(key) ? t(key) : raw);

export const actionLabel = (action: EventAction) => label(`labels.action.${action}`, action);

export const statusLabel = (status: EventStatus) => label(`labels.status.${status}`, status);

export const roleLabel = (role: DeviceRole) => label(`labels.role.${role}`, role);

export const strategyLabel = (strategy: Strategy) => label(`labels.strategy.${strategy}`, strategy);

/** Explication en une ligne d'une stratégie. */
export const strategyHelp = (strategy: Strategy) => label(`labels.strategyHelp.${strategy}`, '');

export const anchorLabel = (anchor: Anchor) => label(`labels.anchor.${anchor}`, anchor);

export const ANCHORS: Anchor[] = ['sunrise', 'sunset', 'fixed', 'device_open', 'device_close'];

/** Stratégie conseillée pour un rôle (identique au serveur). */
export function defaultStrategy(role: DeviceRole): Strategy {
  if (role === 'nest_box' || role === 'feeder') return 'onboard';
  return 'command';
}

/** Explication de la stratégie choisie, avec la recommandation si elle diffère. */
export function strategyHint(strategy: Strategy, role: DeviceRole): string {
  const advised = defaultStrategy(role);
  const help = strategyHelp(strategy);
  return strategy === advised
    ? t('labels.strategyAdvised', { help })
    : t('labels.strategyAdvice', { help, advised: strategyLabel(advised) });
}

/** Libellé d'une valeur brute du serveur ; la valeur elle-même si elle est inconnue. */
function known(group: string, value: string): string | null {
  const key = `labels.${group}.${value}`;
  return hasMessage(key) ? t(key) : null;
}

export function doorStateLabel(state?: string | null): string {
  if (!state) return t('labels.doorState.unknown');
  return known('doorState', state.toLowerCase()) ?? state;
}

/** Libellé du défaut, ou null s'il n'y en a pas ("", "none"). */
export function faultLabel(fault?: string | null): string | null {
  if (!fault || fault.toLowerCase() === 'none') return null;
  return known('fault', fault.toLowerCase()) ?? fault;
}

/** Libellé d'une entrée du journal (actionType = commande Omlet envoyée). */
export function logActionLabel(actionType: string): string {
  return known('logAction', actionType) ?? actionType;
}

export function triggerLabel(trigger: string): string {
  return known('trigger', trigger) ?? trigger;
}

export function deviceTypeLabel(deviceType: string): string {
  if (deviceType === 'Autodoor') return t('labels.deviceType.autodoor');
  if (deviceType === 'Feeder') return t('labels.deviceType.feeder');
  return deviceType || t('labels.deviceType.other');
}

export function powerSourceLabel(powerSource?: string | null): string {
  if (powerSource === 'external') return t('labels.power.external');
  if (powerSource === 'battery') return t('labels.power.battery');
  return powerSource || t('labels.power.unknown');
}
