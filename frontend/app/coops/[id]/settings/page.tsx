'use client';

import React, { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useParams, useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { KeyRound, MapPin, Server } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { BackLink } from '@/components/ui/BackLink';
import { Button, buttonBase, buttonSizes, buttonVariants } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Field, Select, TextInput } from '@/components/ui/Field';
import { ErrorState, PageLoader } from '@/components/ui/States';
import { coopsApi, getErrorMessage, getErrorStatus, systemApi } from '@/lib/api';
import { useI18n, type TFunction } from '@/lib/i18n';
import { LOCALES, LOCALE_NAMES, type Locale } from '@/lib/locale';
import { formatDateTime, formatRelative, formatTime, isValidTimeZone } from '@/lib/time';
import type { CoopDetail, SystemInfo, UpdateCoopRequest } from '@/types';

const COMMON_TIMEZONES = [
  'Europe/Paris',
  'Europe/Brussels',
  'Europe/Luxembourg',
  'Europe/Zurich',
  'Europe/London',
  'Europe/Dublin',
  'Europe/Madrid',
  'Europe/Lisbon',
  'Europe/Berlin',
  'Europe/Rome',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Toronto',
  'America/Montreal',
  'America/Guadeloupe',
  'America/Martinique',
  'America/Cayenne',
  'Indian/Reunion',
  'Indian/Mayotte',
  'Australia/Sydney',
  'Pacific/Auckland',
  'Pacific/Noumea',
  'Pacific/Tahiti',
];

interface FormState {
  name: string;
  latitude: string;
  longitude: string;
  timezone: string;
  language: Locale;
  omletApiKey: string;
}

type FormErrors = Partial<Record<keyof FormState, string>>;

const toForm = (detail: CoopDetail): FormState => ({
  name: detail.coop.name,
  latitude: String(detail.coop.latitude),
  longitude: String(detail.coop.longitude),
  timezone: detail.coop.timezone,
  language: detail.coop.language ?? 'fr',
  omletApiKey: '',
});

const parseCoordinate = (value: string) => Number(value.trim().replace(',', '.'));

function validate(form: FormState, t: TFunction): FormErrors {
  const errors: FormErrors = {};
  if (!form.name.trim()) errors.name = t('common.nameRequired');
  const lat = parseCoordinate(form.latitude);
  if (!form.latitude.trim() || !Number.isFinite(lat) || lat < -90 || lat > 90) {
    errors.latitude = t('coopSettings.latitudeError');
  }
  const lon = parseCoordinate(form.longitude);
  if (!form.longitude.trim() || !Number.isFinite(lon) || lon < -180 || lon > 180) {
    errors.longitude = t('coopSettings.longitudeError');
  }
  if (!isValidTimeZone(form.timezone.trim())) {
    errors.timezone = t('coopSettings.timeZoneError');
  }
  return errors;
}

const SystemCard: React.FC<{ timeZone: string }> = ({ timeZone }) => {
  const { t } = useI18n();
  const [info, setInfo] = useState<SystemInfo | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    systemApi
      .get()
      .then(setInfo)
      .catch((e) => setError(getErrorMessage(e, 'coopSettings.serverUnavailable')));
  }, []);

  return (
    <Card as="section">
      <h2 className="mb-3 flex items-center gap-2 text-lg font-semibold text-ink">
        <Server className="h-5 w-5 text-muted" aria-hidden="true" />
        {t('coopSettings.server')}
      </h2>
      {error && <p className="text-sm text-ink">{error}</p>}
      {!info && !error && <p className="text-sm text-muted">{t('common.loading')}</p>}
      {info && (
        <dl className="grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted">{t('coopSettings.mode')}</dt>
            <dd className="font-medium text-ink">
              {info.mode === 'shadow' ? t('coopSettings.modeShadow') : t('coopSettings.modeLive')}
            </dd>
          </div>
          <div>
            <dt className="text-muted">{t('coopSettings.version')}</dt>
            <dd className="font-medium text-ink">{info.version || '—'}</dd>
          </div>
          <div>
            <dt className="text-muted">{t('coopSettings.lastTick')}</dt>
            <dd className="font-medium text-ink">
              {formatTime(info.lastTick, timeZone)}{' '}
              <span className="font-normal text-muted">
                ({formatRelative(info.lastTick, Date.parse(info.now) || Date.now()) || t('coopSettings.never')})
              </span>
            </dd>
          </div>
          <div>
            <dt className="text-muted">{t('coopSettings.lastPlan')}</dt>
            <dd className="font-medium text-ink">{formatDateTime(info.lastPlan, timeZone)}</dd>
          </div>
          <div>
            <dt className="text-muted">{t('coopSettings.started')}</dt>
            <dd className="font-medium text-ink">{formatDateTime(info.startedAt, timeZone)}</dd>
          </div>
        </dl>
      )}
    </Card>
  );
};

function CoopSettings() {
  const { t } = useI18n();
  const params = useParams<{ id: string }>();
  const coopId = params.id;
  const router = useRouter();

  const [detail, setDetail] = useState<CoopDetail | null>(null);
  const [loadError, setLoadError] = useState<{ message: string; status?: number } | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
  const [locating, setLocating] = useState(false);

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const data = await coopsApi.get(coopId);
      setDetail(data);
      setForm(toForm(data));
    } catch (e) {
      setLoadError({ message: getErrorMessage(e, 'common.coopLoadFailed'), status: getErrorStatus(e) });
    }
  }, [coopId]);

  useEffect(() => {
    load();
  }, [load]);

  if (loadError) {
    return (
      <ErrorState
        title={loadError.status === 404 ? t('common.coopNotFound') : undefined}
        message={loadError.message}
        onRetry={loadError.status === 404 ? undefined : load}
      />
    );
  }
  if (!detail || !form) return <PageLoader />;

  const { coop } = detail;
  const set = (field: keyof FormState) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setForm({ ...form, [field]: e.target.value });
    if (errors[field]) setErrors({ ...errors, [field]: undefined });
  };

  const locate = () => {
    if (!window.isSecureContext || !navigator.geolocation) {
      toast.error(t('coopSettings.geolocationNeedsHttps'));
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setForm((current) =>
          current && {
            ...current,
            latitude: position.coords.latitude.toFixed(4),
            longitude: position.coords.longitude.toFixed(4),
          }
        );
        setErrors((current) => ({ ...current, latitude: undefined, longitude: undefined }));
        toast.success(t('coopSettings.positionDetected'));
        setLocating(false);
      },
      (err) => {
        toast.error(
          err.code === err.PERMISSION_DENIED ? t('coopSettings.geolocationDenied') : t('coopSettings.positionUnavailable')
        );
        setLocating(false);
      },
      { enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 }
    );
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const found = validate(form, t);
    setErrors(found);
    if (Object.values(found).some(Boolean)) return;

    const body: UpdateCoopRequest = {};
    const name = form.name.trim();
    const latitude = parseCoordinate(form.latitude);
    const longitude = parseCoordinate(form.longitude);
    const timezone = form.timezone.trim();
    if (name !== coop.name) body.name = name;
    if (latitude !== coop.latitude) body.latitude = latitude;
    if (longitude !== coop.longitude) body.longitude = longitude;
    if (timezone !== coop.timezone) body.timezone = timezone;
    if (form.language !== (coop.language ?? 'fr')) body.language = form.language;
    if (form.omletApiKey.trim()) body.omletApiKey = form.omletApiKey.trim();

    if (Object.keys(body).length === 0) {
      toast(t('common.noChanges'));
      return;
    }

    setSaving(true);
    try {
      const updated = await coopsApi.update(coop.id, body);
      setDetail(updated);
      setForm(toForm(updated));
      toast.success(t('coopSettings.saved'));
      router.push(`/coops/${coop.id}`);
    } catch (err) {
      const message = getErrorMessage(err, 'coopSettings.saveFailed');
      // « Clé API Omlet refusée : … » / "Omlet API key rejected: …" : affiché aussi sous le champ.
      if (body.omletApiKey && /clé|key/i.test(message)) {
        setErrors((current) => ({ ...current, omletApiKey: message }));
      }
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <BackLink href={`/coops/${coop.id}`}>{coop.name}</BackLink>
        <h1 className="text-2xl font-bold text-ink">{t('coopSettings.title')}</h1>
      </div>

      <form onSubmit={submit} noValidate className="space-y-4">
        <Card as="section" className="space-y-4">
          <Field label={t('common.name')} htmlFor="coop-name" error={errors.name}>
            <TextInput
              id="coop-name"
              value={form.name}
              onChange={set('name')}
              required
              autoComplete="off"
              aria-invalid={!!errors.name || undefined}
            />
          </Field>

          <div>
            <div className="mb-2 flex items-center justify-between gap-2">
              <div className="min-w-0">
                <p className="text-sm font-medium text-ink">{t('common.position')}</p>
                <p className="text-xs text-muted">{t('coopSettings.positionHint')}</p>
              </div>
              <Button variant="outline" size="sm" onClick={locate} isLoading={locating} className="shrink-0 whitespace-nowrap">
                {!locating && <MapPin className="h-4 w-4" aria-hidden="true" />}
                {t('common.myPosition')}
              </Button>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t('common.latitude')} htmlFor="coop-lat" error={errors.latitude}>
                <TextInput
                  id="coop-lat"
                  type="number"
                  step="any"
                  min={-90}
                  max={90}
                  value={form.latitude}
                  onChange={set('latitude')}
                  aria-invalid={!!errors.latitude || undefined}
                />
              </Field>
              <Field label={t('common.longitude')} htmlFor="coop-lon" error={errors.longitude}>
                <TextInput
                  id="coop-lon"
                  type="number"
                  step="any"
                  min={-180}
                  max={180}
                  value={form.longitude}
                  onChange={set('longitude')}
                  aria-invalid={!!errors.longitude || undefined}
                />
              </Field>
            </div>
          </div>

          <Field
            label={t('common.timeZone')}
            htmlFor="coop-tz"
            hint={t('coopSettings.timeZoneHint')}
            error={errors.timezone}
          >
            <TextInput
              id="coop-tz"
              list="coop-tz-list"
              value={form.timezone}
              onChange={set('timezone')}
              autoComplete="off"
              autoCapitalize="off"
              spellCheck={false}
              placeholder={t('coopSettings.timeZonePlaceholder')}
              aria-invalid={!!errors.timezone || undefined}
              aria-describedby={errors.timezone ? 'coop-tz-hint coop-tz-error' : 'coop-tz-hint'}
            />
            <datalist id="coop-tz-list">
              {COMMON_TIMEZONES.map((tz) => (
                <option key={tz} value={tz} />
              ))}
            </datalist>
          </Field>

          <Field label={t('coopSettings.language')} htmlFor="coop-language" hint={t('coopSettings.languageHint')}>
            <Select
              id="coop-language"
              value={form.language}
              onChange={(e) => setForm({ ...form, language: e.target.value as Locale })}
            >
              {LOCALES.map((l) => (
                <option key={l} value={l} lang={l}>
                  {LOCALE_NAMES[l]}
                </option>
              ))}
            </Select>
          </Field>
        </Card>

        <Card as="section" className="space-y-3">
          <h2 className="flex items-center gap-2 text-lg font-semibold text-ink">
            <KeyRound className="h-5 w-5 text-muted" aria-hidden="true" />
            {t('coopSettings.omletAccount')}
          </h2>
          <p className="text-sm text-muted">{coop.hasApiKey ? t('coopSettings.keySaved') : t('coopSettings.noKey')}</p>
          <Field
            label={coop.hasApiKey ? t('coopSettings.newKey') : t('common.omletApiKey')}
            htmlFor="coop-api-key"
            hint={t('coopSettings.keyHint')}
            error={errors.omletApiKey}
          >
            <TextInput
              id="coop-api-key"
              type="password"
              value={form.omletApiKey}
              onChange={set('omletApiKey')}
              autoComplete="new-password"
              autoCapitalize="off"
              spellCheck={false}
              placeholder={coop.hasApiKey ? t('coopSettings.keepKeyPlaceholder') : t('coopSettings.pasteKeyPlaceholder')}
            />
          </Field>
        </Card>

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Link href={`/coops/${coop.id}`} className={`${buttonBase} ${buttonVariants.outline} ${buttonSizes.md}`}>
            {t('common.cancel')}
          </Link>
          <Button type="submit" isLoading={saving} loadingText={t('common.saving')}>
            {t('common.save')}
          </Button>
        </div>
      </form>

      <SystemCard timeZone={coop.timezone} />
    </div>
  );
}

export default function CoopSettingsPage() {
  return (
    <AppShell>
      <CoopSettings />
    </AppShell>
  );
}
