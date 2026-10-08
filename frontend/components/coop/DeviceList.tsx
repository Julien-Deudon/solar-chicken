import React from 'react';
import Link from 'next/link';
import { ChevronRight, DoorClosed, DoorOpen, Egg, Wheat, type LucideIcon } from 'lucide-react';
import { genderOf, nextEvent, phaseOfState, phaseShort, phaseTone, whenText } from '@/lib/coop';
import { useI18n, type TFunction } from '@/lib/i18n';
import { capitalize } from '@/lib/time';
import { faultLabel, isOnBattery } from '@/lib/labels';
import type { DayView, Device, DeviceStateView } from '@/types';

const DOT = { sun: 'bg-sun', ink: 'bg-ink', alert: 'bg-alert', muted: 'bg-muted/60' } as const;

interface DeviceListProps {
  devices: Device[];
  states?: Record<string, DeviceStateView>;
  days: DayView[];
  timeZone: string;
  now: number;
}

function detailFor(device: Device, state: DeviceStateView | undefined, days: DayView[], tz: string, now: number, t: TFunction) {
  const problems: string[] = [];
  const fault = faultLabel(state?.fault);
  if (fault) problems.push(fault);
  if (isOnBattery(state?.power) && state && state.battery > 0 && state.battery < 20) problems.push(t('deviceList.lowBattery', { level: state.battery }));
  if (state?.error) problems.push(t('deviceList.omletDown'));
  if (state?.overdue) problems.push(t('deviceList.overdue'));

  const feed = device.role === 'feeder' && state?.feedLevel !== undefined ? t('deviceList.feed', { level: state.feedLevel }) : null;
  let line: string;
  if (device.role === 'feeder' && device.strategy === 'monitor') {
    line = feed ?? t('deviceList.monitorOnly');
  } else if (!device.enabled) {
    line = t('deviceList.paused');
  } else {
    const next = [nextEvent(device.id, 'open', days, now), nextEvent(device.id, 'close', days, now)]
      .filter((e): e is NonNullable<typeof e> => Boolean(e))
      .sort((a, b) => Date.parse(a.dueAt) - Date.parse(b.dueAt));
    line = next.length
      ? capitalize(next.map((e) => t(e.action === 'open' ? 'deviceList.opens' : 'deviceList.closes', { when: whenText(e, tz, now) })).join(', '))
      : t('deviceList.nothingPlanned');
    if (feed) line += ` · ${feed}`;
  }
  return { line, problems };
}

/** Les appareils du poulailler, une ligne chacun : état, prochaine action, problème éventuel. */
export const DeviceList: React.FC<DeviceListProps> = ({ devices, states, days, timeZone, now }) => {
  const { t } = useI18n();
  return (
    <ul className="overflow-hidden rounded-sheet bg-surface">
      {devices.map((device, i) => {
        const state = states?.[device.id];
        const phase = phaseOfState(state);
        const tone = phaseTone(phase);
        const { line, problems } = detailFor(device, state, days, timeZone, now, t);
        const Icon: LucideIcon =
          device.role === 'feeder' ? Wheat : device.role === 'nest_box' ? Egg : phase === 'open' || phase === 'opening' ? DoorOpen : DoorClosed;
        return (
          <li key={device.id} className={i > 0 ? 'border-t border-line/60' : ''}>
            <Link href={`/devices/${device.id}`} className="flex min-h-[72px] items-center gap-3.5 px-4 py-3 hover:bg-ink/[0.03] active:bg-ink/[0.06]">
              <span
                className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ${
                  problems.length ? 'bg-alert/10 text-alert' : 'bg-canvas text-ink'
                }`}
                aria-hidden="true"
              >
                <Icon className="h-[22px] w-[22px]" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-baseline justify-between gap-3">
                  <span className="truncate text-[17px] font-bold">{device.name}</span>
                  <span className={`flex shrink-0 items-center gap-1.5 text-[15px] font-bold ${tone === 'alert' ? 'text-alert' : ''}`}>
                    <span className={`h-2 w-2 rounded-full ${DOT[tone]}`} aria-hidden="true" />
                    {phaseShort(phase, genderOf(device))}
                  </span>
                </span>
                <span className="tnum block text-[15px] leading-snug text-muted">{line}</span>
                {problems.map((p) => (
                  <span key={p} className="block text-[15px] font-bold leading-snug text-alert">
                    {p}
                  </span>
                ))}
              </span>
              <ChevronRight className="h-5 w-5 shrink-0 text-muted" aria-hidden="true" />
            </Link>
          </li>
        );
      })}
    </ul>
  );
};
