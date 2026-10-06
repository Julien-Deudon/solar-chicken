'use client';

import React from 'react';
import { useI18n } from '@/lib/i18n';
import { dayKey, formatWhen } from '@/lib/time';
import type { Device, Mode } from '@/types';

/** Horaires écrits dans le boîtier de la porte (stratégie « horaires dans le boîtier »). */
export const OnboardInfo: React.FC<{ device: Device; timeZone: string; mode: Mode; now: number }> = ({ device, timeZone, mode, now }) => {
  const { t } = useI18n();
  const synced = Boolean(device.onboardSyncedDay && device.onboardOpenTime && device.onboardCloseTime);
  const stale = synced && device.onboardSyncedDay !== dayKey(new Date(now), timeZone);
  return (
    <div className="text-[15px] leading-snug">
      <p className="font-bold">{t('onboard.title')}</p>
      {synced ? (
        <>
          <p className="tnum mt-0.5 text-[17px]">
            {device.onboardOpenTime} – {device.onboardCloseTime}
          </p>
          <p className="text-muted">
            {device.onboardSyncedAt ? t('onboard.written', { when: formatWhen(device.onboardSyncedAt, timeZone, now) }) : ''}
            {stale ? ` ${t('onboard.stale')}` : ''}
          </p>
        </>
      ) : (
        <p className="text-muted">{mode === 'shadow' ? t('onboard.shadow') : t('onboard.pending')}</p>
      )}
      {device.onboardSyncError && (
        <p className="mt-1 font-bold text-alert">{t('onboard.syncError', { error: device.onboardSyncError })}</p>
      )}
    </div>
  );
};
