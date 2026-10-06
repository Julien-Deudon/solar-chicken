'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { ExternalLink, LocateFixed } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Field, TextInput } from '@/components/ui/Field';
import { coopsApi, getErrorMessage } from '@/lib/api';
import { useI18n } from '@/lib/i18n';
import { isValidTimeZone } from '@/lib/time';

const OMLET_CONSOLE = 'https://smart.omlet.com/developers';

/** Assistant de démarrage : le poulailler (nom, position, fuseau) et la clé de l'API Omlet. */
function Setup() {
  const { locale, t, tRich } = useI18n();
  const router = useRouter();
  const [name, setName] = useState(() => t('setup.defaultName'));
  const [lat, setLat] = useState('');
  const [lon, setLon] = useState('');
  const [tz, setTz] = useState('');
  const [key, setKey] = useState('');
  const [locating, setLocating] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTz(Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Paris');
    coopsApi
      .list()
      .then((list) => {
        if (list.length > 0) router.replace(`/coops/${list[0].id}`);
      })
      .catch(() => undefined);
  }, [router]);

  const locate = () => {
    if (!navigator.geolocation) {
      toast.error(t('setup.noGeolocation'));
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(pos.coords.latitude.toFixed(4));
        setLon(pos.coords.longitude.toFixed(4));
        setLocating(false);
      },
      () => {
        toast.error(t('setup.geolocationFailed'));
        setLocating(false);
      },
      { enableHighAccuracy: false, timeout: 10_000 }
    );
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const latitude = Number(lat.replace(',', '.'));
    const longitude = Number(lon.replace(',', '.'));
    if (!Number.isFinite(latitude) || !Number.isFinite(longitude) || lat === '' || lon === '') {
      toast.error(t('setup.positionRequired'));
      return;
    }
    if (!isValidTimeZone(tz)) {
      toast.error(t('setup.timeZoneUnknown'));
      return;
    }
    setSaving(true);
    try {
      // Langue des notifications : celle de l'interface au départ (modifiable dans les réglages du poulailler).
      const detail = await coopsApi.create({ name: name.trim(), latitude, longitude, timezone: tz, omletApiKey: key.trim(), language: locale });
      toast.success(t('setup.created'));
      router.replace(`/coops/${detail.coop.id}/devices/add`);
    } catch (error) {
      toast.error(getErrorMessage(error, 'setup.createFailed'));
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-8">
      <div>
        <h1 className="text-[34px] font-extrabold leading-tight">{t('setup.title')}</h1>
        <p className="mt-1 text-lg text-muted">{t('setup.subtitle')}</p>
      </div>

      <Card as="section" className="space-y-5">
        <h2 className="text-[22px] font-bold">{t('setup.where')}</h2>
        <Field label={t('common.name')} htmlFor="setup-name">
          <TextInput id="setup-name" value={name} onChange={(e) => setName(e.target.value)} required autoComplete="off" />
        </Field>
        <div>
          <div className="mb-2 flex items-center justify-between gap-3">
            <p className="text-[15px] font-bold">{t('common.position')}</p>
            <Button variant="outline" size="sm" onClick={locate} isLoading={locating}>
              <LocateFixed className="h-4 w-4" aria-hidden="true" />
              {t('common.myPosition')}
            </Button>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field label={t('common.latitude')} htmlFor="setup-lat">
              <TextInput id="setup-lat" inputMode="decimal" value={lat} onChange={(e) => setLat(e.target.value)} placeholder={t('setup.latitudePlaceholder')} required />
            </Field>
            <Field label={t('common.longitude')} htmlFor="setup-lon">
              <TextInput id="setup-lon" inputMode="decimal" value={lon} onChange={(e) => setLon(e.target.value)} placeholder={t('setup.longitudePlaceholder')} required />
            </Field>
          </div>
          <p className="mt-1.5 text-sm text-muted">{t('setup.positionHint')}</p>
        </div>
        <Field label={t('common.timeZone')} htmlFor="setup-tz" hint={t('setup.timeZoneHint')}>
          <TextInput id="setup-tz" value={tz} onChange={(e) => setTz(e.target.value)} required autoComplete="off" />
        </Field>
      </Card>

      <Card as="section" className="space-y-4">
        <h2 className="text-[22px] font-bold">{t('setup.keyTitle')}</h2>
        <ol className="list-decimal space-y-1.5 pl-5 text-[16px] leading-snug">
          <li>
            {tRich('setup.step1', {
              link: (
                <a href={OMLET_CONSOLE} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-bold underline">
                  {t('setup.consoleLink')}
                  <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                </a>
              ),
            })}
          </li>
          <li>{t('setup.step2')}</li>
          <li>{t('setup.step3')}</li>
        </ol>
        <Field label={t('common.omletApiKey')} htmlFor="setup-key">
          <TextInput id="setup-key" type="password" value={key} onChange={(e) => setKey(e.target.value)} required autoComplete="off" />
        </Field>
      </Card>

      <Button type="submit" size="lg" className="w-full" isLoading={saving} loadingText={t('setup.checkingKey')}>
        {t('setup.submit')}
      </Button>
    </form>
  );
}

export default function SetupPage() {
  return (
    <AppShell>
      <Setup />
    </AppShell>
  );
}
