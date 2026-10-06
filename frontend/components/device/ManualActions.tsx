'use client';

import React, { useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { ArrowDownToLine, ArrowUpToLine, Lightbulb, LightbulbOff, Square } from 'lucide-react';
import { devicesApi, getErrorMessage } from '@/lib/api';
import { genderOf, objectOf } from '@/lib/coop';
import { useI18n } from '@/lib/i18n';
import type { Device, DeviceAction, Mode } from '@/types';

// Libellé, confirmation et toast : locales/ (actions.open.label…).
const ICONS: Record<DeviceAction, React.ReactNode> = {
  open: <ArrowUpToLine className="h-5 w-5" aria-hidden="true" />,
  close: <ArrowDownToLine className="h-5 w-5" aria-hidden="true" />,
  stop: <Square className="h-4 w-4" aria-hidden="true" />,
  light_on: <Lightbulb className="h-5 w-5" aria-hidden="true" />,
  light_off: <LightbulbOff className="h-5 w-5" aria-hidden="true" />,
};

interface ManualActionsProps {
  device: Device;
  mode: Mode;
  /** Appelé après une commande acceptée (relire l'état, le journal…). */
  onDone?: () => void;
}

/** Commandes manuelles d'une porte, en gros boutons : ouvrir, fermer, arrêter, et la lumière s'il y en a une. */
export const ManualActions: React.FC<ManualActionsProps> = ({ device, mode, onDone }) => {
  const { t } = useI18n();
  const [running, setRunning] = useState<DeviceAction | null>(null);
  const timers = useRef<number[]>([]);

  useEffect(() => {
    const pending = timers.current;
    return () => pending.forEach((id) => window.clearTimeout(id));
  }, []);

  const run = async (action: DeviceAction) => {
    const warning = mode === 'shadow' ? `\n\n${t('actions.shadowWarning')}` : '';
    if (!window.confirm(t(`actions.${action}.confirm`, { object: objectOf(device) }) + warning)) return;
    setRunning(action);
    try {
      await devicesApi.action(device.id, action);
      toast.success(t(`actions.${action}.done`));
      if (onDone) {
        onDone();
        // La porte met quelques secondes à bouger : on relit l'état un peu plus tard.
        timers.current.push(window.setTimeout(onDone, 5_000), window.setTimeout(onDone, 20_000));
      }
    } catch (error) {
      toast.error(getErrorMessage(error, 'common.commandRefused'));
    } finally {
      setRunning(null);
    }
  };

  const tile = (action: DeviceAction) => (
    <button
      key={action}
      type="button"
      onClick={() => run(action)}
      disabled={running !== null}
      aria-busy={running === action || undefined}
      className="flex min-h-[76px] flex-col items-center justify-center gap-1.5 rounded-2xl bg-surface text-[15px] font-bold text-ink ring-1 ring-inset ring-line/70 hover:bg-canvas active:bg-line/40 disabled:opacity-50"
    >
      {running === action ? (
        <span className="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden="true" />
      ) : (
        ICONS[action]
      )}
      {t(`actions.${action}.label`)}
    </button>
  );

  return (
    <div className="space-y-2.5">
      <div className="grid grid-cols-3 gap-2.5" role="group" aria-label={t('actions.group', { name: device.name, gender: genderOf(device) })}>
        {tile('open')}
        {tile('close')}
        {tile('stop')}
      </div>
      {device.hasLight && (
        <div className="grid grid-cols-2 gap-2.5" role="group" aria-label={t('actions.lightGroup')}>
          {tile('light_on')}
          {tile('light_off')}
        </div>
      )}
    </div>
  );
};
