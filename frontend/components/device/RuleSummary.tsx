'use client';

import React from 'react';
import { useI18n } from '@/lib/i18n';
import { describeMoment, getMoment } from '@/lib/rule';
import { formatDuration } from '@/lib/time';
import type { Device } from '@/types';

/** Les horaires d'une porte, dits en phrases. */
export const RuleSummary: React.FC<{ device: Device; devices: Device[] }> = ({ device, devices }) => {
  const { t } = useI18n();
  const rule = device.rule;
  if (!rule) return <p className="text-[16px] text-muted">{t('ruleSummary.none')}</p>;
  const byId = (id: string | null) => devices.find((d) => d.id === id);
  const open = getMoment(rule, 'open');
  const close = getMoment(rule, 'close');
  const lights: string[] = [];
  if (device.hasLight && device.strategy === 'command') {
    if (rule.enableLightMorning && rule.lightBeforeOpenMinutes > 0) {
      lights.push(t('ruleSummary.lightOnBeforeOpen', { duration: formatDuration(rule.lightBeforeOpenMinutes) }));
    }
    if (rule.enableLightEvening && rule.lightBeforeCloseMinutes > 0) {
      lights.push(t('ruleSummary.lightOnBeforeClose', { duration: formatDuration(rule.lightBeforeCloseMinutes) }));
    }
    if (rule.lightOffDelayMinutes > 0) lights.push(t('ruleSummary.lightOffAfter', { duration: formatDuration(rule.lightOffDelayMinutes) }));
  }
  return (
    <div className="space-y-1.5 text-[17px] leading-snug">
      <p>
        <strong>{t('rule.opens')}</strong> {describeMoment(open, byId(open.refDeviceId))}.
      </p>
      <p>
        <strong>{t('rule.closes')}</strong> {describeMoment(close, byId(close.refDeviceId))}.
      </p>
      {lights.length > 0 && <p className="text-muted">{t('ruleSummary.lights', { list: lights.join(', ') })}</p>}
    </div>
  );
};
