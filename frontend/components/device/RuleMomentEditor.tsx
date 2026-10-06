'use client';

import React, { useState } from 'react';
import { Minus, Plus, X } from 'lucide-react';
import { Chips } from '@/components/ui/Chips';
import { inputBaseClass } from '@/components/ui/Input';
import { NumberField } from '@/components/ui/NumberField';
import { useI18n } from '@/lib/i18n';
import { anchorLabel } from '@/lib/labels';
import { OFFSET_LIMIT, describeMoment, isDeviceAnchor, type Moment, type MomentKind } from '@/lib/rule';
import type { Anchor, Device, HHMM } from '@/types';

type Direction = 'before' | 'after';
type AnchorGroup = 'sunrise' | 'sunset' | 'fixed' | 'device';

const groupOf = (a: Anchor): AnchorGroup => (isDeviceAnchor(a) ? 'device' : (a as AnchorGroup));

const Label: React.FC<{ children: React.ReactNode; htmlFor?: string }> = ({ children, htmlFor }) =>
  htmlFor ? (
    <label htmlFor={htmlFor} className="mb-2 block text-[15px] font-bold text-muted">
      {children}
    </label>
  ) : (
    <p className="mb-2 text-[15px] font-bold text-muted">{children}</p>
  );

const Bound: React.FC<{ id: string; label: string; value: HHMM | null; fallback: HHMM; onChange: (v: HHMM | null) => void }> = ({
  id,
  label,
  value,
  fallback,
  onChange,
}) => {
  const { t } = useI18n();
  return (
    <div className="flex min-h-[48px] items-center justify-between gap-3">
      <label htmlFor={id} className="text-[16px] font-bold">
        {label}
      </label>
      {value ? (
        <span className="flex items-center gap-1">
          <input id={id} type="time" value={value} onChange={(e) => onChange(e.target.value || null)} className={`${inputBaseClass} tnum w-[7.5rem]`} />
          <button
            type="button"
            onClick={() => onChange(null)}
            className="inline-flex h-11 w-11 items-center justify-center rounded-xl text-muted hover:bg-ink/5 hover:text-ink"
            aria-label={t('ruleEditor.remove', { label })}
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </button>
        </span>
      ) : (
        <button
          id={id}
          type="button"
          onClick={() => onChange(fallback)}
          className="min-h-[44px] rounded-xl px-3 text-[15px] font-bold text-muted ring-1 ring-inset ring-line hover:bg-ink/5 hover:text-ink"
        >
          {t('common.add')}
        </button>
      )}
    </div>
  );
};

interface RuleMomentEditorProps {
  kind: MomentKind;
  moment: Moment;
  onChange: (moment: Moment) => void;
  /** Autres portes du poulailler (repère « ouverture / fermeture d'une autre porte »). */
  otherDoors: Device[];
}

/** « S'ouvre … » ou « Se ferme … » : le résultat en clair, puis les choix à toucher. */
export const RuleMomentEditor: React.FC<RuleMomentEditorProps> = ({ kind, moment, onChange, otherDoors }) => {
  const { t } = useI18n();
  const id = (suffix: string) => `${kind}-${suffix}`;
  const opening = kind === 'open';

  // Le sens (avant / après) est gardé même quand le décalage vaut 0.
  const [direction, setDirection] = useState<Direction>(moment.offsetMinutes < 0 ? 'before' : 'after');
  const [lastOffset, setLastOffset] = useState(moment.offsetMinutes);
  if (moment.offsetMinutes !== lastOffset) {
    setLastOffset(moment.offsetMinutes);
    if (moment.offsetMinutes !== 0) setDirection(moment.offsetMinutes < 0 ? 'before' : 'after');
  }
  const magnitude = Math.abs(moment.offsetMinutes);
  const update = (patch: Partial<Moment>) => onChange({ ...moment, ...patch });
  const setMagnitude = (n: number) => {
    const m = Math.min(OFFSET_LIMIT, Math.max(0, n));
    update({ offsetMinutes: direction === 'before' ? -m : m });
  };

  const changeGroup = (group: AnchorGroup) => {
    if (group === groupOf(moment.anchor)) return;
    if (group === 'device') {
      const ref = otherDoors.some((d) => d.id === moment.refDeviceId) ? moment.refDeviceId : otherDoors[0]?.id ?? null;
      update({ anchor: kind === 'open' ? 'device_open' : 'device_close', refDeviceId: ref });
    } else if (group === 'fixed') {
      update({ anchor: 'fixed', offsetMinutes: 0, fixedTime: moment.fixedTime ?? (kind === 'open' ? '08:00' : '20:00') });
    } else {
      update({ anchor: group });
    }
  };

  const refDevice = otherDoors.find((d) => d.id === moment.refDeviceId);
  const group = groupOf(moment.anchor);

  return (
    <fieldset>
      <legend className="font-display text-[22px] font-bold">{t(opening ? 'rule.opens' : 'rule.closes')}</legend>
      <p className="tnum mt-0.5 text-[17px] leading-snug" aria-live="polite">
        {describeMoment(moment, refDevice)}
      </p>

      <div className="mt-5 space-y-5">
        <div>
          <Label>{t('ruleEditor.anchor')}</Label>
          <Chips<AnchorGroup>
            label={t(opening ? 'ruleEditor.anchorOfOpen' : 'ruleEditor.anchorOfClose')}
            value={group}
            onChange={changeGroup}
            options={[
              { value: 'sunrise', label: anchorLabel('sunrise') },
              { value: 'sunset', label: anchorLabel('sunset') },
              { value: 'fixed', label: anchorLabel('fixed') },
              { value: 'device', label: t('ruleEditor.otherDoor'), disabled: otherDoors.length === 0 },
            ]}
          />
        </div>

        {group === 'device' && (
          <div className="space-y-3">
            {otherDoors.length > 1 && (
              <Chips<string>
                label={t('ruleEditor.refDoor')}
                value={moment.refDeviceId}
                onChange={(refDeviceId) => update({ refDeviceId })}
                options={otherDoors.map((d) => ({ value: d.id, label: d.name }))}
              />
            )}
            <Chips<Anchor>
              label={t('ruleEditor.refMoment')}
              value={moment.anchor}
              onChange={(anchor) => update({ anchor })}
              options={[
                { value: 'device_open', label: t('ruleEditor.whenOpens') },
                { value: 'device_close', label: t('ruleEditor.whenCloses') },
              ]}
            />
          </div>
        )}

        {group === 'fixed' ? (
          <div>
            <Label htmlFor={id('fixed')}>{t('ruleEditor.time')}</Label>
            <input
              id={id('fixed')}
              type="time"
              value={moment.fixedTime ?? ''}
              onChange={(e) => update({ fixedTime: e.target.value || null })}
              required
              className={`${inputBaseClass} tnum w-[8.5rem] text-lg`}
            />
          </div>
        ) : (
          <div>
            <Label htmlFor={id('offset')}>{t('ruleEditor.offset')}</Label>
            <div className="flex flex-wrap items-center gap-2">
              <span className="flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={() => setMagnitude(magnitude - 5)}
                  className="inline-flex h-12 w-12 items-center justify-center rounded-xl bg-canvas ring-1 ring-inset ring-line hover:bg-line/40"
                  aria-label={t('ruleEditor.less')}
                >
                  <Minus className="h-5 w-5" aria-hidden="true" />
                </button>
                <NumberField
                  id={id('offset')}
                  value={magnitude}
                  min={0}
                  max={OFFSET_LIMIT}
                  onValueChange={setMagnitude}
                  className="w-[4.5rem] text-center text-lg font-bold"
                  aria-label={t(opening ? 'ruleEditor.offsetOfOpen' : 'ruleEditor.offsetOfClose')}
                />
                <button
                  type="button"
                  onClick={() => setMagnitude(magnitude + 5)}
                  className="inline-flex h-12 w-12 items-center justify-center rounded-xl bg-canvas ring-1 ring-inset ring-line hover:bg-line/40"
                  aria-label={t('ruleEditor.more')}
                >
                  <Plus className="h-5 w-5" aria-hidden="true" />
                </button>
                <span className="px-1 text-[16px] font-bold">{t('common.minutesUnit')}</span>
              </span>
              <Chips<Direction>
                label={t('ruleEditor.direction')}
                value={direction}
                onChange={(next) => {
                  setDirection(next);
                  update({ offsetMinutes: next === 'before' ? -magnitude : magnitude });
                }}
                options={[
                  { value: 'before', label: t('ruleEditor.before') },
                  { value: 'after', label: t('ruleEditor.after') },
                ]}
              />
            </div>
          </div>
        )}

        <div className="border-t border-line/60 pt-2">
          <Bound
            id={id('not-before')}
            label={t('ruleEditor.notBefore')}
            value={moment.notBefore}
            fallback={opening ? '07:00' : '17:00'}
            onChange={(notBefore) => update({ notBefore })}
          />
          <Bound
            id={id('not-after')}
            label={t('ruleEditor.notAfter')}
            value={moment.notAfter}
            fallback={opening ? '09:00' : '22:00'}
            onChange={(notAfter) => update({ notAfter })}
          />
        </div>
      </div>
    </fieldset>
  );
};
