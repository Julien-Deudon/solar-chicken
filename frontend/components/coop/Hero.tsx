'use client';

import React, { useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { Button } from '@/components/ui/Button';
import { devicesApi, getErrorMessage } from '@/lib/api';
import { heroFor } from '@/lib/coop';
import { useI18n } from '@/lib/i18n';
import type { DayView, Device, DeviceStateView, Mode } from '@/types';

interface HeroProps {
  main?: Device;
  /** État de la porte principale (en direct si disponible, sinon celui mis en cache). */
  state?: DeviceStateView | null;
  days: DayView[];
  timeZone: string;
  now: number;
  mode: Mode;
  /** Relire l'état en direct (après une commande). */
  onRefreshState: () => void;
  onChanged: () => void;
}

/** Une phrase pour répondre à « la porte est-elle fermée ? », et l'action utile s'il y en a une. */
export const Hero: React.FC<HeroProps> = ({ main, state, days, timeZone, now, mode, onRefreshState, onChanged }) => {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const timers = useRef<number[]>([]);

  useEffect(() => {
    const pending = timers.current;
    return () => pending.forEach((id) => window.clearTimeout(id));
  }, []);

  const model = heroFor(main, state, days, timeZone, now);

  const act = async () => {
    if (!main || !model.action) return;
    const opening = model.action.kind === 'open';
    const warning = mode === 'shadow' ? `\n\n${t('hero.shadowWarning')}` : '';
    if (!window.confirm(t(opening ? 'hero.confirmOpen' : 'hero.confirmClose') + warning)) return;
    setBusy(true);
    try {
      await devicesApi.action(main.id, model.action.kind);
      toast.success(t(opening ? 'hero.openRequested' : 'hero.closeRequested'));
      [4_000, 15_000, 35_000].forEach((ms) => timers.current.push(window.setTimeout(onRefreshState, ms)));
      onChanged();
    } catch (error) {
      toast.error(getErrorMessage(error, 'common.commandRefused'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-live="polite" className="pt-1">
      <p
        className={`font-display text-[40px] font-extrabold leading-[1.02] tracking-[-0.02em] sm:text-[52px] ${
          model.tone === 'alert' ? 'text-alert' : ''
        }`}
      >
        {model.title}
      </p>
      <p className="tnum mt-2 max-w-[32ch] text-lg leading-snug text-muted">{model.detail}</p>
      {model.action && main && (
        <Button
          variant={model.tone === 'alert' ? 'primary' : 'outline'}
          size="sm"
          onClick={act}
          isLoading={busy}
          className="mt-4"
        >
          {model.action.label}
        </Button>
      )}
    </section>
  );
};
