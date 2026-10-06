'use client';

import React, { useCallback, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useParams, useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { Check, DoorClosed, Wheat } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { BackLink } from '@/components/ui/BackLink';
import { Button, buttonBase, buttonSizes, buttonVariants } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Field, Select, TextInput } from '@/components/ui/Field';
import { ErrorState, Notice, PageLoader } from '@/components/ui/States';
import { coopsApi, getErrorMessage } from '@/lib/api';
import { useI18n } from '@/lib/i18n';
import {
  defaultStrategy,
  deviceTypeLabel,
  doorStateLabel,
  roleLabel,
  strategyHint,
  strategyLabel,
} from '@/lib/labels';
import type { CoopDetail, DeviceRole, OmletDeviceView, Strategy } from '@/types';

const DOOR_ROLES: DeviceRole[] = ['main_door', 'nest_box', 'door'];
const DOOR_STRATEGIES: Strategy[] = ['command', 'onboard', 'monitor'];

const isFeederType = (device: OmletDeviceView) => device.deviceType === 'Feeder';

interface Draft {
  role: DeviceRole;
  name: string;
  nameTouched: boolean;
  strategy: Strategy;
}

function suggestedName(device: OmletDeviceView, role: DeviceRole) {
  return role === 'nest_box' ? roleLabel('nest_box') : device.name;
}

function AddDevice() {
  const { t } = useI18n();
  const params = useParams<{ id: string }>();
  const coopId = params.id;
  const router = useRouter();

  const [detail, setDetail] = useState<CoopDetail | null>(null);
  const [omletDevices, setOmletDevices] = useState<OmletDeviceView[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<OmletDeviceView | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);

  // Sur téléphone, le formulaire est sous la liste : on l'amène à l'écran.
  useEffect(() => {
    if (selected) formRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }, [selected]);

  const load = useCallback(async () => {
    setError(null);
    setOmletDevices(null);
    try {
      const coop = await coopsApi.get(coopId);
      setDetail(coop);
      setOmletDevices(await coopsApi.omletDevices(coopId));
    } catch (e) {
      setError(getErrorMessage(e, 'addDevice.loadFailed'));
    }
  }, [coopId]);

  useEffect(() => {
    load();
  }, [load]);

  const mainDoor = detail?.devices.find((d) => d.role === 'main_door') ?? null;

  const choose = (device: OmletDeviceView) => {
    const role: DeviceRole = isFeederType(device) ? 'feeder' : mainDoor ? 'nest_box' : 'main_door';
    setSelected(device);
    setDraft({ role, name: suggestedName(device, role), nameTouched: false, strategy: defaultStrategy(role) });
  };

  const changeRole = (role: DeviceRole) => {
    if (!draft || !selected) return;
    setDraft({
      ...draft,
      role,
      strategy: defaultStrategy(role),
      name: draft.nameTouched ? draft.name : suggestedName(selected, role),
    });
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected || !draft) return;
    setSaving(true);
    try {
      const device = await coopsApi.addDevice(coopId, {
        omletDeviceId: selected.deviceId,
        role: draft.role,
        name: draft.name.trim() || undefined,
        strategy: draft.strategy,
      });
      toast.success(t('addDevice.added', { name: device.name }));
      router.push(`/devices/${device.id}/settings`);
    } catch (err) {
      toast.error(getErrorMessage(err, 'addDevice.addFailed'));
      setSaving(false);
    }
  };

  const header = (
    <div>
      <BackLink href={`/coops/${coopId}`}>{detail?.coop.name ?? t('addDevice.coopFallback')}</BackLink>
      <h1 className="text-2xl font-bold text-ink">{t('common.addDevice')}</h1>
      <p className="mt-1 text-sm text-muted">{t('addDevice.subtitle')}</p>
    </div>
  );

  if (error) {
    return (
      <div className="space-y-4">
        {header}
        <ErrorState message={error} onRetry={load}>
          {detail && !detail.coop.hasApiKey && (
            <Link href={`/coops/${coopId}/settings`} className={`${buttonBase} ${buttonVariants.outline} ${buttonSizes.sm}`}>
              {t('addDevice.addKey')}
            </Link>
          )}
        </ErrorState>
      </div>
    );
  }

  if (!detail || !omletDevices) {
    return (
      <div className="space-y-4">
        {header}
        <PageLoader label={t('addDevice.reading')} />
      </div>
    );
  }

  const roleOptions: DeviceRole[] = selected && isFeederType(selected) ? ['feeder'] : DOOR_ROLES;
  const strategyOptions: Strategy[] = selected && isFeederType(selected) ? ['monitor'] : DOOR_STRATEGIES;

  return (
    <div className="space-y-4">
      {header}

      {omletDevices.length === 0 ? (
        <Card>
          <p className="text-[17px] font-bold">{t('addDevice.none')}</p>
          <p className="mt-1 text-[15px] text-muted">{t('addDevice.noneHint')}</p>
        </Card>
      ) : (
        <ul className="overflow-hidden rounded-sheet bg-surface" aria-label={t('addDevice.listLabel')}>
          {omletDevices.map((device, i) => {
            const isSelected = selected?.deviceId === device.deviceId;
            const Icon = isFeederType(device) ? Wheat : DoorClosed;
            const facts = [
              deviceTypeLabel(device.deviceType),
              device.doorState ? doorStateLabel(device.doorState).toLowerCase() : null,
              device.powerSource === 'battery' ? t('addDevice.onBattery') : device.powerSource ? t('addDevice.onMains') : null,
              device.hasLight ? t('addDevice.withLight') : null,
            ].filter(Boolean);
            return (
              <li key={device.deviceId} className={i > 0 ? 'border-t border-line/60' : ''}>
                <button
                  type="button"
                  onClick={() => choose(device)}
                  disabled={device.alreadyAdded}
                  aria-pressed={isSelected}
                  className={`flex min-h-[72px] w-full items-center gap-3.5 px-4 py-3 text-left disabled:cursor-not-allowed ${
                    isSelected ? 'bg-sun/15' : 'hover:bg-ink/[0.03]'
                  }`}
                >
                  <span className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ${device.alreadyAdded ? 'bg-canvas text-muted' : 'bg-canvas text-ink'}`} aria-hidden="true">
                    <Icon className="h-[22px] w-[22px]" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className={`block truncate text-[17px] font-bold ${device.alreadyAdded ? 'text-muted' : ''}`}>{device.name}</span>
                    <span className="block text-[15px] leading-snug text-muted">{facts.join(', ')}</span>
                    {device.alreadyAdded && <span className="block text-[15px] font-bold text-ok">{t('addDevice.alreadyAdded')}</span>}
                    {!device.alreadyAdded && !device.sameGroup && <span className="block text-[15px] font-bold">{t('addDevice.otherGroup')}</span>}
                  </span>
                  {isSelected && <Check className="h-6 w-6 shrink-0 text-ink" aria-hidden="true" />}
                </button>
              </li>
            );
          })}
        </ul>
      )}

      {selected && draft && (
        <form onSubmit={submit} ref={formRef} className="scroll-mt-20">
          <Card as="section" className="space-y-4">
            <h2 className="text-[22px] font-bold">{selected.name}</h2>

            {!selected.sameGroup && (
              <Notice tone="warning">{t('addDevice.otherGroupWarning')}</Notice>
            )}

            <Field label={t('common.role')} htmlFor="add-role">
              <Select
                id="add-role"
                value={draft.role}
                onChange={(e) => changeRole(e.target.value as DeviceRole)}
                disabled={roleOptions.length === 1}
              >
                {roleOptions.map((role) => (
                  <option key={role} value={role} disabled={role === 'main_door' && !!mainDoor}>
                    {roleLabel(role)}
                    {role === 'main_door' && mainDoor ? t('common.alreadyMainDoor', { name: mainDoor.name }) : ''}
                  </option>
                ))}
              </Select>
            </Field>

            <Field label={t('common.name')} htmlFor="add-name" hint={t('addDevice.omletName', { name: selected.name })}>
              <TextInput
                id="add-name"
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value, nameTouched: true })}
                autoComplete="off"
              />
            </Field>

            <Field label={t('common.strategy')} htmlFor="add-strategy" hint={strategyHint(draft.strategy, draft.role)}>
              <Select
                id="add-strategy"
                value={draft.strategy}
                onChange={(e) => setDraft({ ...draft, strategy: e.target.value as Strategy })}
                disabled={strategyOptions.length === 1}
              >
                {strategyOptions.map((strategy) => (
                  <option key={strategy} value={strategy}>
                    {strategyLabel(strategy)}
                  </option>
                ))}
              </Select>
            </Field>

            <Button type="submit" className="w-full" isLoading={saving} loadingText={t('addDevice.adding')}>
              {t('addDevice.submit')}
            </Button>
            <p className="text-center text-sm text-muted">{t('addDevice.footnote')}</p>
          </Card>
        </form>
      )}
    </div>
  );
}

export default function AddDevicePage() {
  return (
    <AppShell>
      <AddDevice />
    </AppShell>
  );
}
