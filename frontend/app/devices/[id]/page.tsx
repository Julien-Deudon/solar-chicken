'use client';

import React, { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { Check, RefreshCw, X } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { ManualActions } from '@/components/device/ManualActions';
import { OnboardInfo } from '@/components/device/OnboardInfo';
import { RuleSummary } from '@/components/device/RuleSummary';
import { BackLink } from '@/components/ui/BackLink';
import { SectionTitle } from '@/components/ui/Card';
import { ErrorState, Notice, PageLoader, Spinner } from '@/components/ui/States';
import { devicesApi, getErrorMessage } from '@/lib/api';
import { doorPhase, nextEvent, phaseSentence, stateFromStatus, whenText } from '@/lib/coop';
import { useDeviceContext, useDeviceStatus, useNow, usePolling } from '@/lib/hooks';
import { useI18n, type TFunction } from '@/lib/i18n';
import { faultLabel, isOnBattery, logActionLabel, roleLabel, triggerLabel } from '@/lib/labels';
import { capitalize, dayKey, formatDateShort, formatTime, formatWhen } from '@/lib/time';
import type { ActionLog, DeviceStatus } from '@/types';

const LOG_LIMIT = 60;

function wifiWord(dbm: number, t: TFunction): string {
  if (!dbm) return '—';
  const quality = t(dbm >= -60 ? 'devicePage.wifiGood' : dbm >= -70 ? 'devicePage.wifiFair' : 'devicePage.wifiWeak');
  return t('devicePage.wifiValue', { quality, dbm });
}

/** Lignes « libellé : valeur » de l'état en direct. */
function stateRows(s: DeviceStatus, tz: string, now: number, t: TFunction): { label: string; value: string; alert?: boolean }[] {
  const part = s.door ?? s.feeder;
  const rows: { label: string; value: string; alert?: boolean }[] = [];
  const fault = faultLabel(part?.fault);
  if (fault) rows.push({ label: t('devicePage.rowFault'), value: fault, alert: true });
  if (s.feeder) {
    rows.push({ label: t('devicePage.rowFeed'), value: t('devicePage.percent', { value: s.feeder.feedLevel }), alert: s.feeder.feedLevel < 15 });
  }
  if (s.light) rows.push({ label: t('devicePage.rowLight'), value: t(s.light.state === 'on' ? 'devicePage.lightOn' : 'devicePage.lightOff') });
  rows.push(
    isOnBattery(s.powerSource)
      ? { label: t('devicePage.rowPower'), value: t('devicePage.battery', { level: s.batteryLevel }), alert: s.batteryLevel < 20 }
      : { label: t('devicePage.rowPower'), value: t('devicePage.mains') }
  );
  rows.push({ label: t('devicePage.rowWifi'), value: wifiWord(s.wifiStrength, t) });
  const asleep = !s.connected && (s.asleep ?? isOnBattery(s.powerSource));
  rows.push({
    label: t('devicePage.rowConnection'),
    value: s.overdue
      ? t('devicePage.overdue', { when: formatWhen(s.lastSeen, tz, now) })
      : s.connected
        ? t('devicePage.online')
        : asleep
          ? s.nextWake
            ? t('devicePage.asleepUntil', { when: formatWhen(s.nextWake, tz, now) })
            : t('devicePage.asleep')
          : t('devicePage.offline'),
    alert: !!s.overdue || (!s.connected && !asleep),
  });
  if (part?.lastOpenTime) rows.push({ label: t('devicePage.rowLastOpen'), value: formatWhen(part.lastOpenTime, tz, now) });
  if (part?.lastCloseTime) rows.push({ label: t('devicePage.rowLastClose'), value: formatWhen(part.lastCloseTime, tz, now) });
  if (s.firmware) rows.push({ label: t('devicePage.rowFirmware'), value: s.firmware });
  return rows;
}

function dayTitle(day: string, sample: string, tz: string, now: number, t: TFunction): string {
  if (day === dayKey(new Date(now), tz)) return t('common.today');
  if (day === dayKey(new Date(now - 86_400_000), tz)) return t('common.yesterday');
  return capitalize(formatDateShort(sample, tz));
}

const Journal: React.FC<{ logs: ActionLog[]; tz: string; now: number }> = ({ logs, tz, now }) => {
  const { t } = useI18n();
  const groups: { day: string; logs: ActionLog[] }[] = [];
  for (const log of logs) {
    const day = dayKey(log.executedAt, tz);
    const last = groups[groups.length - 1];
    if (last && last.day === day) last.logs.push(log);
    else groups.push({ day, logs: [log] });
  }
  return (
    <div className="space-y-5">
      {groups.map((g) => (
        <div key={g.day}>
          <h3 className="mb-1 font-sans text-[15px] font-bold text-muted">{dayTitle(g.day, g.logs[0].executedAt, tz, now, t)}</h3>
          <ol>
            {g.logs.map((log, i) => {
              const ok = log.status === 'success';
              return (
                <li key={log.id} className={`flex gap-3 py-2.5 ${i > 0 ? 'border-t border-line/60' : ''}`}>
                  <span className="tnum w-[3.3rem] shrink-0 text-[17px] font-bold leading-6">{formatTime(log.executedAt, tz)}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block font-bold leading-6">{logActionLabel(log.actionType)}</span>
                    <span className="block text-sm text-muted">
                      {triggerLabel(log.triggeredBy)}
                      {log.note && log.note !== 'historique v1' ? `, ${log.note}` : ''}
                    </span>
                    {log.errorMessage && <span className="block break-words text-sm font-bold text-alert">{log.errorMessage}</span>}
                  </span>
                  {ok ? (
                    <Check className="mt-0.5 h-5 w-5 shrink-0 text-ok" aria-label={t('devicePage.succeeded')} />
                  ) : (
                    <X className="mt-0.5 h-5 w-5 shrink-0 text-alert" aria-label={t('devicePage.failed')} />
                  )}
                </li>
              );
            })}
          </ol>
        </div>
      ))}
    </div>
  );
};

function DeviceScreen() {
  const { t } = useI18n();
  const params = useParams<{ id: string }>();
  const deviceId = params.id;
  const now = useNow(30_000);
  const { coop, device, error, reload } = useDeviceContext(deviceId);
  const live = useDeviceStatus(deviceId);
  const [logs, setLogs] = useState<ActionLog[] | null>(null);
  const [logsError, setLogsError] = useState<string | null>(null);

  const loadLogs = useCallback(async () => {
    try {
      setLogs(await devicesApi.logs(deviceId, LOG_LIMIT));
      setLogsError(null);
    } catch (e) {
      setLogsError(getErrorMessage(e, 'devicePage.logLoadFailed'));
    }
  }, [deviceId]);

  useEffect(() => {
    loadLogs();
  }, [loadLogs]);
  usePolling(loadLogs, 60_000);

  if (error && !device) return <ErrorState message={error} onRetry={reload} />;
  if (!device || !coop) return <PageLoader label={t('common.deviceLoading')} />;

  const tz = coop.coop.timezone;
  const state = live.status ? stateFromStatus(live.status) : coop.states?.[device.id];
  const phase = doorPhase(state?.door, state?.fault);
  const days = [coop.today, coop.tomorrow];
  const next = [nextEvent(device.id, 'open', days, now), nextEvent(device.id, 'close', days, now)]
    .filter((e): e is NonNullable<typeof e> => Boolean(e))
    .sort((a, b) => Date.parse(a.dueAt) - Date.parse(b.dueAt))[0];

  const sentence = phaseSentence(device, phase);
  const following = phase === 'fault'
    ? t('devicePage.faultHelp')
    : !device.enabled
    ? t('devicePage.paused')
    : next
      ? t(next.action === 'open' ? 'coop.opensAt' : 'coop.closesAt', { when: whenText(next, tz, now) })
      : '';
  const role = roleLabel(device.role);

  const afterAction = () => {
    live.refresh();
    loadLogs();
  };

  return (
    <div className="space-y-8">
      <div>
        <BackLink href={`/coops/${coop.coop.id}`}>{coop.coop.name}</BackLink>
        <div className="flex items-baseline justify-between gap-3">
          <h1 className="truncate text-[30px] font-extrabold leading-tight">{device.name}</h1>
          <Link href={`/devices/${device.id}/settings`} className="shrink-0 rounded-xl px-2 py-2 text-[15px] font-bold text-muted hover:bg-ink/5 hover:text-ink">
            {t('common.settings')}
          </Link>
        </div>
        {role.toLowerCase() !== device.name.toLowerCase() && <p className="text-[15px] text-muted">{role}</p>}
        <p className={`mt-4 font-display text-[28px] font-bold leading-tight ${phase === 'fault' ? 'text-alert' : ''}`}>{sentence}</p>
        {following && <p className="tnum mt-1 text-lg text-muted">{following}</p>}
        {live.error && !live.status && <p className="mt-2 text-[15px] text-muted">{t('devicePage.liveUnavailable', { error: live.error })}</p>}
      </div>

      <section aria-label={t('devicePage.controls')} className="space-y-2">
        <ManualActions device={device} mode={coop.mode} onDone={afterAction} />
        {coop.mode === 'shadow' && <p className="px-1 text-sm text-muted">{t('devicePage.shadowNote')}</p>}
      </section>

      <section aria-labelledby="rule-title">
        <SectionTitle
          id="rule-title"
          action={
            <Link href={`/devices/${device.id}/settings`} className="rounded-xl px-2 py-1.5 text-[15px] font-bold text-muted hover:bg-ink/5 hover:text-ink">
              {t('common.edit')}
            </Link>
          }
        >
          {t('devicePage.schedule')}
        </SectionTitle>
        <div className="space-y-4 rounded-sheet bg-surface p-4 sm:p-5">
          <RuleSummary device={device} devices={coop.devices} />
          {device.strategy === 'onboard' && (
            <div className="border-t border-line/60 pt-4">
              <OnboardInfo device={device} timeZone={tz} mode={coop.mode} now={now} />
            </div>
          )}
        </div>
      </section>

      <section aria-labelledby="state-title">
        <SectionTitle
          id="state-title"
          action={
            <button
              type="button"
              onClick={live.refresh}
              disabled={live.loading}
              className="inline-flex min-h-[40px] items-center gap-1.5 rounded-xl px-2 text-sm text-muted hover:bg-ink/5 hover:text-ink disabled:opacity-60"
              aria-label={t('devicePage.refresh')}
            >
              {live.updatedAt && <span className="tnum">{t('devicePage.readAt', { time: formatTime(new Date(live.updatedAt), tz) })}</span>}
              <RefreshCw className={`h-4 w-4 ${live.loading ? 'animate-spin' : ''}`} aria-hidden="true" />
            </button>
          }
        >
          {t('devicePage.state')}
        </SectionTitle>
        <div className="rounded-sheet bg-surface px-4 py-1">
          {live.status ? (
            <dl>
              {stateRows(live.status, tz, now, t).map((row, i) => (
                <div key={row.label} className={`flex items-baseline justify-between gap-4 py-3 ${i > 0 ? 'border-t border-line/60' : ''}`}>
                  <dt className="text-[15px] text-muted">{row.label}</dt>
                  <dd className={`tnum text-right text-[16px] font-bold ${row.alert ? 'text-alert' : ''}`}>{row.value}</dd>
                </div>
              ))}
            </dl>
          ) : (
            <p className="flex items-center gap-2 py-4 text-[15px] text-muted">
              {live.loading ? <Spinner className="h-4 w-4" /> : null}
              {live.loading ? t('devicePage.readingState') : live.error ?? t('devicePage.stateUnavailable')}
            </p>
          )}
        </div>
      </section>

      <section aria-labelledby="log-title">
        <SectionTitle id="log-title">{t('devicePage.log')}</SectionTitle>
        <div className="rounded-sheet bg-surface p-4 sm:p-5">
          {logsError && <Notice tone="error">{logsError}</Notice>}
          {!logs && !logsError && (
            <p className="flex items-center gap-2 text-[15px] text-muted">
              <Spinner className="h-4 w-4" /> {t('devicePage.readingLog')}
            </p>
          )}
          {logs && logs.length === 0 && <p className="text-[15px] text-muted">{t('devicePage.noLog')}</p>}
          {logs && logs.length > 0 && <Journal logs={logs} tz={tz} now={now} />}
        </div>
      </section>
    </div>
  );
}

export default function DevicePage() {
  return (
    <AppShell>
      <DeviceScreen />
    </AppShell>
  );
}
