'use client';

import React from 'react';
import { NumberField } from '@/components/ui/NumberField';
import { Switch } from '@/components/ui/Switch';
import { useI18n } from '@/lib/i18n';
import { LIGHT_LIMIT } from '@/lib/rule';
import type { Rule } from '@/types';

type LightFields = Pick<Rule, 'enableLightMorning' | 'lightBeforeOpenMinutes' | 'enableLightEvening' | 'lightBeforeCloseMinutes' | 'lightOffDelayMinutes'>;

const Minutes: React.FC<{ id: string; label: string; value: number; onChange: (v: number) => void }> = ({ id, label, value, onChange }) => {
  const { t } = useI18n();
  return (
    <div className="flex min-h-[48px] items-center justify-between gap-3 pl-1">
      <label htmlFor={id} className="text-[16px]">
        {label}
      </label>
      <span className="flex items-center gap-2">
        <NumberField id={id} value={value} min={0} max={LIGHT_LIMIT} onValueChange={onChange} className="w-[4.5rem] text-center font-bold" />
        <span className="font-bold">{t('common.minutesUnit')}</span>
      </span>
    </div>
  );
};

/** Lumière du poulailler (seulement si la porte en a une et qu'elle est pilotée par le serveur). */
export const LightSettings: React.FC<{ rule: Rule; onChange: (patch: Partial<LightFields>) => void }> = ({ rule, onChange }) => {
  const { t } = useI18n();
  return (
    <fieldset className="space-y-3">
      <legend className="font-display text-[22px] font-bold">{t('light.title')}</legend>
      <Switch
        id="light-morning"
        label={t('light.morning')}
        description={t('light.morningHint')}
        checked={rule.enableLightMorning}
        onChange={(value) => onChange({ enableLightMorning: value })}
      />
      {rule.enableLightMorning && (
        <Minutes id="light-before-open" label={t('light.beforeOpen')} value={rule.lightBeforeOpenMinutes} onChange={(v) => onChange({ lightBeforeOpenMinutes: v })} />
      )}
      <div className="border-t border-line/60 pt-3">
        <Switch
          id="light-evening"
          label={t('light.evening')}
          description={t('light.eveningHint')}
          checked={rule.enableLightEvening}
          onChange={(value) => onChange({ enableLightEvening: value })}
        />
      </div>
      {rule.enableLightEvening && (
        <Minutes id="light-before-close" label={t('light.beforeClose')} value={rule.lightBeforeCloseMinutes} onChange={(v) => onChange({ lightBeforeCloseMinutes: v })} />
      )}
      {(rule.enableLightMorning || rule.enableLightEvening) && (
        <div className="border-t border-line/60 pt-1">
          <Minutes id="light-off-delay" label={t('light.offAfter')} value={rule.lightOffDelayMinutes} onChange={(v) => onChange({ lightOffDelayMinutes: v })} />
        </div>
      )}
    </fieldset>
  );
};
