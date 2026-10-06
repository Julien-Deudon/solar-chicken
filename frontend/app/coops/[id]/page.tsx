'use client';

import React, { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { Plus, Settings2 } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { DeviceList } from '@/components/coop/DeviceList';
import { Hero } from '@/components/coop/Hero';
import { ShadowBanner } from '@/components/coop/ShadowBanner';
import { SkyBand } from '@/components/coop/SkyBand';
import { DayJournal } from '@/components/coop/Timeline';
import { buttonBase, buttonSizes, buttonVariants } from '@/components/ui/Button';
import { SectionTitle } from '@/components/ui/Card';
import { ErrorState, Notice, PageLoader } from '@/components/ui/States';
import { coopsApi, getErrorMessage, getErrorStatus } from '@/lib/api';
import { useLiveDeviceState, useNow, usePolling } from '@/lib/hooks';
import { intlLocale, useI18n } from '@/lib/i18n';
import { capitalize, formatDayLong } from '@/lib/time';
import type { CoopDetail, Device } from '@/types';

const byPosition = (a: Device, b: Device) => a.position - b.position || a.name.localeCompare(b.name, intlLocale());

function CoopScreen() {
  const { t } = useI18n();
  const params = useParams<{ id: string }>();
  const coopId = params.id;
  const now = useNow(30_000);
  const [detail, setDetail] = useState<CoopDetail | null>(null);
  const [error, setError] = useState<{ message: string; status?: number } | null>(null);

  const load = useCallback(async () => {
    try {
      setDetail(await coopsApi.get(coopId));
      setError(null);
    } catch (e) {
      setError({ message: getErrorMessage(e, 'common.coopLoadFailed'), status: getErrorStatus(e) });
    }
  }, [coopId]);

  useEffect(() => {
    load();
  }, [load]);
  usePolling(load, 60_000);

  const mainId = detail?.devices.find((d) => d.role === 'main_door')?.id;
  const { live: mainLive, refresh: refreshMain } = useLiveDeviceState(mainId);

  if (!detail) {
    if (error) {
      return (
        <ErrorState
          title={error.status === 404 ? t('common.coopNotFound') : undefined}
          message={error.message}
          onRetry={error.status === 404 ? undefined : load}
        >
          <Link href="/dashboard" className={`${buttonBase} ${buttonVariants.outline} ${buttonSizes.sm}`}>
            {t('common.myCoops')}
          </Link>
        </ErrorState>
      );
    }
    return <PageLoader label={t('coopPage.loading')} />;
  }

  const { coop, devices, today, tomorrow, mode, states } = detail;
  const tz = coop.timezone;
  const sorted = [...devices].sort(byPosition);
  const main = sorted.find((d) => d.role === 'main_door');
  // Une seule source pour la porte principale : l'état en direct dès qu'il est lu.
  const mergedStates = main && mainLive ? { ...states, [main.id]: mainLive } : states;

  return (
    <div className="space-y-6">
      <header className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="truncate text-[22px] font-bold leading-tight">{coop.name}</h1>
          <p className="text-[15px] text-muted">{capitalize(formatDayLong(today.day))}</p>
        </div>
        <Link
          href={`/coops/${coop.id}/settings`}
          aria-label={t('coopSettings.title')}
          className="-mr-2 inline-flex h-11 w-11 items-center justify-center rounded-xl text-muted hover:bg-ink/5 hover:text-ink"
        >
          <Settings2 className="h-[22px] w-[22px]" aria-hidden="true" />
        </Link>
      </header>

      {mode === 'shadow' && <ShadowBanner />}
      {error && <Notice tone="error">{t('coopPage.refreshFailed', { message: error.message })}</Notice>}
      {!coop.hasApiKey && (
        <Notice tone="warning" title={t('coopPage.missingKeyTitle')}>
          {t('coopPage.missingKeyText')}{' '}
          <Link href={`/coops/${coop.id}/settings`} className="font-bold underline">
            {t('coopPage.addKey')}
          </Link>
        </Notice>
      )}

      <div className="space-y-8 lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,400px)] lg:items-start lg:gap-10 lg:space-y-0">
        <div className="space-y-8">
          <div className="space-y-5">
            <Hero
              main={main}
              state={main ? mergedStates?.[main.id] : null}
              days={[today, tomorrow]}
              timeZone={tz}
              now={now}
              mode={mode}
              onRefreshState={refreshMain}
              onChanged={load}
            />
            <SkyBand day={today} devices={devices} timeZone={tz} now={now} />
          </div>

          <section aria-labelledby="devices-title">
            <SectionTitle
              id="devices-title"
              action={
                <Link
                  href={`/coops/${coop.id}/devices/add`}
                  className="-mr-1 inline-flex min-h-[40px] items-center gap-1 rounded-xl px-2 text-[15px] font-bold text-muted hover:bg-ink/5 hover:text-ink"
                >
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  {t('common.add')}
                </Link>
              }
            >
              {t('coopPage.devices')}
            </SectionTitle>
            {sorted.length ? (
              <DeviceList devices={sorted} states={mergedStates} days={[today, tomorrow]} timeZone={tz} now={now} />
            ) : (
              <div className="rounded-sheet bg-surface p-5">
                <p className="text-[17px] font-bold">{t('coopPage.noDevices')}</p>
                <p className="mt-1 text-[15px] text-muted">{t('coopPage.noDevicesHint')}</p>
                <Link href={`/coops/${coop.id}/devices/add`} className={`${buttonBase} ${buttonVariants.primary} ${buttonSizes.md} mt-4`}>
                  {t('common.addDevice')}
                </Link>
              </div>
            )}
          </section>
        </div>

        <div className="lg:sticky lg:top-24">
          <DayJournal today={today} tomorrow={tomorrow} devices={devices} timeZone={tz} now={now} />
        </div>
      </div>
    </div>
  );
}

export default function CoopPage() {
  return (
    <AppShell wide>
      <CoopScreen />
    </AppShell>
  );
}
