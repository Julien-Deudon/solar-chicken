'use client';

import React, { useState } from 'react';
import { ofDevice } from '@/lib/coop';
import { useI18n, type TFunction } from '@/lib/i18n';
import { statusLabel } from '@/lib/labels';
import { formatRelative, formatTime } from '@/lib/time';
import type { DayView, Device, EventAction, EventStatus, PlannedEvent } from '@/types';

/**
 * Clé stable d'un évènement. Les jours pas encore en base sont un aperçu calculé
 * dont les id sont vides : on utilise (appareil, jour, action), unique côté serveur.
 */
export const eventKey = (event: PlannedEvent) => `${event.deviceId}:${event.day}:${event.action}`;

export const sortEvents = (events: PlannedEvent[]) => [...events].sort((a, b) => Date.parse(a.dueAt) - Date.parse(b.dueAt));

const STATUS_CLASS: Record<EventStatus, string> = {
  pending: 'text-muted',
  sent: 'text-ink',
  confirmed: 'text-ok',
  failed: 'text-alert',
  skipped: 'text-muted',
  shadow: 'text-muted',
};

function labelFor(action: EventAction, t: TFunction, device?: Device): string {
  switch (action) {
    case 'open':
      return device ? t('journal.open', { of: ofDevice(device) }) : t('journal.openAlone');
    case 'close':
      return device ? t('journal.close', { of: ofDevice(device) }) : t('journal.closeAlone');
    case 'light_before_open':
    case 'light_before_close':
      return t('journal.lightOn');
    default:
      return t('journal.lightOff');
  }
}

interface DayJournalProps {
  today: DayView;
  tomorrow: DayView;
  devices: Device[];
  timeZone: string;
  now: number;
}

/** Ce qui est prévu et ce qui s'est passé, heure par heure. */
export const DayJournal: React.FC<DayJournalProps> = ({ today, tomorrow, devices, timeZone, now }) => {
  const { t, tRich } = useI18n();
  const [showTomorrow, setShowTomorrow] = useState(false);
  const view = showTomorrow ? tomorrow : today;
  const byId = new Map(devices.map((d) => [d.id, d]));
  const events = sortEvents(view.events);
  const next = sortEvents([...today.events, ...tomorrow.events]).find((e) => e.status === 'pending' && Date.parse(e.dueAt) >= now);
  const errors = Object.entries(view.errors ?? {});

  return (
    <section aria-labelledby="journal-title">
      <div className="mb-2 flex items-end justify-between gap-3 px-1">
        <h2 id="journal-title" className="text-[22px] font-bold leading-tight">
          {t('journal.title')}
        </h2>
        <div className="flex rounded-xl bg-surface p-1 text-[15px] font-bold" role="group" aria-label={t('journal.dayShown')}>
          {[false, true].map((isTomorrow) => (
            <button
              key={String(isTomorrow)}
              type="button"
              onClick={() => setShowTomorrow(isTomorrow)}
              aria-pressed={showTomorrow === isTomorrow}
              className={`min-h-[36px] rounded-lg px-3 ${showTomorrow === isTomorrow ? 'bg-ink text-canvas' : 'text-muted'}`}
            >
              {isTomorrow ? t('common.tomorrow') : t('common.today')}
            </button>
          ))}
        </div>
      </div>

      <div className="rounded-sheet bg-surface px-4 py-1.5">
        {errors.map(([deviceId, message]) => (
          <p key={deviceId} className="my-2.5 rounded-xl bg-sun/20 px-3 py-2 text-[15px]">
            {tRich('journal.deviceError', {
              name: <strong>{byId.get(deviceId)?.name ?? t('journal.deviceFallback')}</strong>,
              message,
            })}
          </p>
        ))}
        {events.length === 0 ? (
          <p className="py-4 text-[15px] text-muted">{t('journal.empty')}</p>
        ) : (
          <ol>
            {events.map((event, i) => {
              const device = byId.get(event.deviceId);
              const isNext = next !== undefined && eventKey(next) === eventKey(event);
              const status = STATUS_CLASS[event.status] ? event.status : 'pending';
              const note = event.status === 'failed' || event.status === 'skipped' || event.status === 'sent' ? event.lastError || event.note : '';
              return (
                <li key={eventKey(event)} className={`relative flex gap-3 py-3 ${i > 0 ? 'border-t border-line/60' : ''}`}>
                  {isNext && <span className="absolute -left-4 bottom-2 top-2 w-1 rounded-r-full bg-sun" aria-hidden="true" />}
                  <span className="tnum w-[3.3rem] shrink-0 font-display text-lg font-bold leading-6">{formatTime(event.dueAt, timeZone)}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-[16px] font-bold leading-6">{labelFor(event.action, t, device)}</span>
                    {note && <span className="block text-sm text-muted">{note}</span>}
                  </span>
                  <span className={`shrink-0 text-[15px] font-bold leading-6 ${isNext ? 'text-ink' : STATUS_CLASS[status]}`}>
                    {isNext ? formatRelative(event.dueAt, now) : statusLabel(status)}
                  </span>
                </li>
              );
            })}
          </ol>
        )}
      </div>
    </section>
  );
};
