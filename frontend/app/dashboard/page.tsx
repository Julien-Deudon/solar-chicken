'use client';

import React, { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ChevronRight, MapPin } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { ErrorState, PageLoader } from '@/components/ui/States';
import { coopsApi, getErrorMessage } from '@/lib/api';
import { useI18n } from '@/lib/i18n';
import type { CoopView } from '@/types';

function CoopList() {
  const { t } = useI18n();
  const router = useRouter();
  const [coops, setCoops] = useState<CoopView[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Aucun poulailler : assistant de démarrage ; un seul : on y va directement.
  const load = useCallback(async () => {
    setError(null);
    try {
      const list = await coopsApi.list();
      if (list.length === 0) {
        router.replace('/setup');
        return;
      }
      if (list.length === 1) {
        router.replace(`/coops/${list[0].id}`);
        return;
      }
      setCoops(list);
    } catch (e) {
      setError(getErrorMessage(e, 'dashboard.loadFailed'));
    }
  }, [router]);

  useEffect(() => {
    load();
  }, [load]);

  if (error) return <ErrorState message={error} onRetry={load} />;
  if (!coops) return <PageLoader label={t('dashboard.loading')} />;

  return (
    <>
      <h1 className="mb-4 text-2xl font-bold text-ink">{t('common.myCoops')}</h1>
      <ul className="space-y-3">
        {coops.map((coop) => (
          <li key={coop.id}>
            <Link
              href={`/coops/${coop.id}`}
              className="flex min-h-[64px] items-center justify-between gap-3 rounded-xl bg-surface p-4 shadow-sm ring-1 ring-line hover:bg-canvas"
            >
              <div className="min-w-0">
                <p className="truncate text-lg font-semibold text-ink">{coop.name}</p>
                <p className="flex items-center gap-1 text-sm text-muted">
                  <MapPin className="h-4 w-4" aria-hidden="true" />
                  {t('dashboard.deviceCount', { count: coop.deviceCount })}
                </p>
              </div>
              <ChevronRight className="h-5 w-5 shrink-0 text-muted/70" aria-hidden="true" />
            </Link>
          </li>
        ))}
      </ul>
    </>
  );
}

export default function DashboardPage() {
  return (
    <AppShell>
      <CoopList />
    </AppShell>
  );
}
