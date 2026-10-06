'use client';

import React, { useEffect, useState } from 'react';
import { minutesOfDay } from '@/lib/coop';
import { useI18n } from '@/lib/i18n';
import { dayKey, formatDayLong, formatTime } from '@/lib/time';
import type { DayView, Device } from '@/types';

const SKY = { night: '#1E2B4A', blue: '#4A5D8F', dawn: '#F5B935', day: '#C9DDEC', noon: '#E3EEF6' };
const pct = (minutes: number) => Math.min(100, Math.max(0, (minutes / 1440) * 100));

interface SkyBandProps {
  day: DayView;
  devices: Device[];
  timeZone: string;
  now: number;
}

/**
 * La journée du poulailler : un ciel calé sur le vrai lever et le vrai coucher du soleil,
 * avec les ouvertures (▲) et fermetures (▼) de chaque porte et l'heure qu'il est.
 */
export const SkyBand: React.FC<SkyBandProps> = ({ day, devices, timeZone, now }) => {
  const { t } = useI18n();
  const [arrived, setArrived] = useState(false);
  useEffect(() => {
    const id = window.requestAnimationFrame(() => setArrived(true));
    return () => window.cancelAnimationFrame(id);
  }, []);

  const sunrise = pct(minutesOfDay(day.sunrise, timeZone));
  const sunset = pct(minutesOfDay(day.sunset, timeZone));
  const isToday = day.day === dayKey(new Date(now), timeZone);
  const nowPos = isToday ? pct(minutesOfDay(new Date(now), timeZone)) : null;
  const isDay = nowPos !== null && nowPos >= sunrise && nowPos <= sunset;

  const stops = [
    `${SKY.night} 0%`,
    `${SKY.night} ${Math.max(0, sunrise - 7)}%`,
    `${SKY.blue} ${Math.max(0, sunrise - 3)}%`,
    `${SKY.dawn} ${sunrise}%`,
    `#F3D98F ${sunrise + 2}%`,
    `${SKY.day} ${sunrise + 6}%`,
    `${SKY.noon} ${(sunrise + sunset) / 2}%`,
    `${SKY.day} ${sunset - 6}%`,
    `#F3D98F ${sunset - 2}%`,
    `${SKY.dawn} ${sunset}%`,
    `${SKY.blue} ${Math.min(100, sunset + 3)}%`,
    `${SKY.night} ${Math.min(100, sunset + 7)}%`,
    `${SKY.night} 100%`,
  ];

  const byId = new Map(devices.map((d) => [d.id, d]));
  const markers = day.events
    .filter((e) => (e.action === 'open' || e.action === 'close') && byId.get(e.deviceId)?.role !== 'feeder')
    .map((e) => {
      const device = byId.get(e.deviceId);
      const open = e.action === 'open';
      const time = formatTime(e.dueAt, timeZone);
      const nest = device?.role === 'nest_box';
      return {
        key: `${e.deviceId}:${e.action}`,
        x: pct(minutesOfDay(e.dueAt, timeZone)),
        open,
        main: device?.role === 'main_door',
        past: Date.parse(e.dueAt) < now,
        failed: e.status === 'failed',
        time,
        label: t(nest ? (open ? 'skyBand.nestOpens' : 'skyBand.nestCloses') : open ? 'skyBand.doorOpens' : 'skyBand.doorCloses', { time }),
      };
    });

  const summary = [
    t('skyBand.summary', {
      day: formatDayLong(day.day),
      sunrise: formatTime(day.sunrise, timeZone),
      sunset: formatTime(day.sunset, timeZone),
    }),
    ...markers.map((m) => `${m.label}.`),
    nowPos !== null ? t('skyBand.now', { time: formatTime(new Date(now), timeZone) }) : '',
  ].join(' ');

  return (
    <figure className="select-none">
      <div
        role="img"
        aria-label={summary}
        className="relative h-[104px] overflow-hidden rounded-[20px] ring-1 ring-inset ring-black/5 dark:ring-white/10"
        style={{ background: `linear-gradient(90deg, ${stops.join(', ')})` }}
      >
        {markers.map((m) => (
          <span
            key={m.key}
            className="absolute -translate-x-1/2"
            style={{ left: `${m.x}%`, bottom: m.main ? 14 : 34, opacity: m.past && !m.failed ? 0.55 : 1 }}
            aria-hidden="true"
          >
            <svg width={m.main ? 16 : 12} height={m.main ? 14 : 10} viewBox="0 0 16 14">
              <path
                d={m.open ? 'M8 1 15 13H1z' : 'M8 13 15 1H1z'}
                fill={m.failed ? '#C02D26' : m.main ? '#FFFFFF' : SKY.dawn}
                stroke={SKY.night}
                strokeWidth="1.5"
                strokeLinejoin="round"
              />
            </svg>
          </span>
        ))}

        {markers
          .filter((m) => m.main)
          .map((m) => (
            <span
              key={`${m.key}-t`}
              className="tnum absolute top-2.5 -translate-x-1/2 rounded-md bg-white/85 px-1.5 py-0.5 font-display text-[13px] font-bold text-sky-night"
              style={{ left: `${Math.min(92, Math.max(8, m.x))}%` }}
              aria-hidden="true"
            >
              {m.time}
            </span>
          ))}

        {nowPos !== null && (
          <span
            className="absolute inset-y-0 w-0 transition-[left] duration-[1400ms] ease-out"
            style={{ left: `${arrived ? nowPos : 0}%` }}
            aria-hidden="true"
          >
            <span className="absolute inset-y-0 -left-px w-[2px] bg-white/80" />
            <span
              className={`absolute -left-[9px] bottom-[42px] h-[18px] w-[18px] rounded-full ring-2 ring-white ${
                isDay ? 'bg-sky-dawn shadow-[0_0_14px_4px_rgba(245,185,53,0.55)]' : 'bg-white shadow-[inset_-5px_-2px_0_0_#C9D3E4]'
              }`}
            />
          </span>
        )}
      </div>

      <div className="relative mt-1.5 h-5 text-[13px] text-muted" aria-hidden="true">
        <span className="tnum absolute -translate-x-1/2 whitespace-nowrap" style={{ left: `${Math.max(9, sunrise)}%` }}>
          {t('skyBand.sunrise', { time: formatTime(day.sunrise, timeZone) })}
        </span>
        <span className="tnum absolute -translate-x-1/2 whitespace-nowrap" style={{ left: `${Math.min(88, sunset)}%` }}>
          {t('skyBand.sunset', { time: formatTime(day.sunset, timeZone) })}
        </span>
      </div>
    </figure>
  );
};
