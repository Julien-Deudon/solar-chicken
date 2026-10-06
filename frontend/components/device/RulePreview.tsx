'use client';

import React from 'react';
import { Spinner } from '@/components/ui/States';
import { useI18n } from '@/lib/i18n';
import { capitalize, formatDayShort, formatTime } from '@/lib/time';
import type { PreviewDay } from '@/types';

interface RulePreviewProps {
  days: PreviewDay[] | null;
  /** Message du serveur quand le brouillon est invalide (400). */
  error: string | null;
  loading: boolean;
  timeZone: string;
  /** Explication quand aucune action ne sera envoyée (appareil désactivé, surveillance…). */
  note?: string | null;
}

/** Les 7 prochains jours calculés par le serveur avec la règle en cours d'édition. */
export const RulePreview: React.FC<RulePreviewProps> = ({ days, error, loading, timeZone, note }) => {
  const { t } = useI18n();
  return (
    <section aria-labelledby="preview-title" aria-busy={loading}>
      <div className="mb-1 flex items-center justify-between gap-2">
        <h2 id="preview-title" className="text-[22px] font-bold">
          {t('preview.title')}
        </h2>
        {loading && <Spinner className="h-5 w-5" label={t('preview.computing')} />}
      </div>
      {note && <p className="mb-2 text-[15px] text-muted">{note}</p>}
      {error && (
        <p role="alert" className="my-2 rounded-xl bg-alert/10 px-3 py-2 text-[15px] font-bold text-alert">
          {error}
        </p>
      )}
      {!days && !error && <p className="text-[15px] text-muted">{t('preview.computing')}</p>}
      {days && (
        <ol className={`transition-opacity ${error ? 'opacity-40' : ''}`}>
          {days.map((day, i) => {
            const lights = day.events.filter((e) => e.action.startsWith('light'));
            return (
              <li key={day.day} className={`py-2.5 ${i > 0 ? 'border-t border-line/60' : ''}`}>
                <div className="flex items-baseline justify-between gap-3">
                  <span className="text-[16px] text-muted">{capitalize(formatDayShort(day.day))}</span>
                  <span className="tnum text-lg font-bold">
                    {formatTime(day.open, timeZone)} – {formatTime(day.close, timeZone)}
                  </span>
                </div>
                {lights.length > 0 && (
                  <p className="tnum text-right text-sm text-muted">
                    {t('preview.lights', {
                      list: lights
                        .map((e) => t(e.action.includes('after') ? 'preview.lightOff' : 'preview.lightOn', { time: formatTime(e.dueAt, timeZone) }))
                        .join(', '),
                    })}
                  </p>
                )}
                {day.error && <p className="text-[15px] font-bold text-alert">{day.error}</p>}
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
};
