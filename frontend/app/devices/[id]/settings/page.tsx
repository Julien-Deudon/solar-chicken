'use client';

import React, { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { Trash2 } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { LightSettings } from '@/components/device/LightSettings';
import { RuleMomentEditor } from '@/components/device/RuleMomentEditor';
import { RulePreview } from '@/components/device/RulePreview';
import { BackLink } from '@/components/ui/BackLink';
import { Button } from '@/components/ui/Button';
import { Card, SectionTitle } from '@/components/ui/Card';
import { Field, Select, TextInput } from '@/components/ui/Field';
import { ErrorState, Notice, PageLoader } from '@/components/ui/States';
import { Switch } from '@/components/ui/Switch';
import { devicesApi, getErrorMessage, isCancelled } from '@/lib/api';
import { useDeviceContext } from '@/lib/hooks';
import { useI18n } from '@/lib/i18n';
import { deviceTypeLabel, roleLabel, strategyHint, strategyLabel } from '@/lib/labels';
import { ofDevice } from '@/lib/coop';
import { DEFAULT_RULE, getMoment, isDoor, rulesEqual, toRulePayload, withMoment } from '@/lib/rule';
import type { Device, DeviceRole, PreviewDay, Rule, Strategy, UpdateDeviceRequest } from '@/types';

interface DeviceForm {
  name: string;
  role: DeviceRole;
  strategy: Strategy;
  enabled: boolean;
}

const toForm = (device: Device): DeviceForm => ({
  name: device.name,
  role: device.role,
  strategy: device.strategy,
  enabled: device.enabled,
});

function deviceChanges(form: DeviceForm, device: Device): UpdateDeviceRequest {
  const body: UpdateDeviceRequest = {};
  if (form.name.trim() !== device.name) body.name = form.name.trim();
  if (form.role !== device.role) body.role = form.role;
  if (form.strategy !== device.strategy) body.strategy = form.strategy;
  if (form.enabled !== device.enabled) body.enabled = form.enabled;
  return body;
}

const ruleChanged = (rule: Rule | null, device: Device) =>
  !!rule && (!device.rule || !rulesEqual(rule, device.rule));

function DeviceSettings() {
  const { t } = useI18n();
  const params = useParams<{ id: string }>();
  const deviceId = params.id;
  const router = useRouter();
  const { coop, device, error, reload } = useDeviceContext(deviceId);

  const [form, setForm] = useState<DeviceForm | null>(null);
  const [rule, setRule] = useState<Rule | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [preview, setPreview] = useState<PreviewDay[] | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  // Brouillons initialisés au premier chargement (puis après chaque enregistrement).
  useEffect(() => {
    if (device && !form) {
      setForm(toForm(device));
      setRule(isDoor(device) ? device.rule ?? DEFAULT_RULE : null);
    }
  }, [device, form]);

  // Aperçu en direct (POST /devices/:id/rule/preview), 500 ms après la dernière modification.
  useEffect(() => {
    if (!rule) return;
    const controller = new AbortController();
    setPreviewLoading(true);
    const timer = window.setTimeout(async () => {
      try {
        const days = await devicesApi.previewRule(deviceId, toRulePayload(rule), controller.signal);
        setPreview(days);
        setPreviewError(null);
      } catch (e) {
        if (isCancelled(e)) return;
        setPreviewError(getErrorMessage(e, 'deviceSettings.previewUnavailable'));
      } finally {
        if (!controller.signal.aborted) setPreviewLoading(false);
      }
    }, 500);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [deviceId, rule]);

  const dirty =
    !!device && !!form && (Object.keys(deviceChanges(form, device)).length > 0 || ruleChanged(rule, device));

  // Avertit avant de quitter / recharger la page avec des modifications non enregistrées.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [dirty]);

  if (error && !device) return <ErrorState message={error} onRetry={reload} />;
  if (!device || !coop || !form) return <PageLoader label={t('common.deviceLoading')} />;

  const door = isDoor(device);
  const tz = coop.coop.timezone;
  const otherDoors = coop.devices.filter((d) => d.id !== device.id && isDoor(d));
  const otherMainDoor = coop.devices.find((d) => d.id !== device.id && d.role === 'main_door');
  const roleOptions: DeviceRole[] = door ? ['main_door', 'nest_box', 'door'] : ['feeder'];
  const strategyOptions: Strategy[] = door ? ['command', 'onboard', 'monitor'] : ['monitor'];
  const showLight = device.hasLight && form.strategy === 'command';

  const previewNote = !device.enabled
    ? t('deviceSettings.noteDisabled')
    : device.strategy === 'monitor'
      ? t('deviceSettings.noteMonitor')
      : form.strategy !== device.strategy || form.enabled !== device.enabled
        ? t('deviceSettings.notePending')
        : null;

  const save = async () => {
    if (!form.name.trim()) {
      toast.error(t('common.nameRequired'));
      return;
    }
    const body = deviceChanges(form, device);
    const saveRule = ruleChanged(rule, device);
    if (Object.keys(body).length === 0 && !saveRule) {
      toast(t('common.noChanges'));
      return;
    }

    setSaving(true);
    if (Object.keys(body).length > 0) {
      try {
        await devicesApi.update(device.id, body);
      } catch (e) {
        toast.error(getErrorMessage(e, 'deviceSettings.saveDeviceFailed'));
        setSaving(false);
        return;
      }
    }

    if (saveRule && rule) {
      try {
        await devicesApi.putRule(device.id, toRulePayload(rule));
      } catch (e) {
        const message = getErrorMessage(e, 'deviceSettings.saveRuleFailed');
        setPreviewError(message);
        toast.error(t('deviceSettings.ruleNotSaved', { message }));
        // L'appareil a peut-être été enregistré : on relit, en gardant le brouillon des horaires.
        const fresh = await reload();
        if (fresh) setForm(toForm(fresh.device));
        setSaving(false);
        return;
      }
    }

    toast.success(t('deviceSettings.saved'));
    const fresh = await reload();
    if (fresh) {
      setForm(toForm(fresh.device));
      setRule(isDoor(fresh.device) ? fresh.device.rule ?? DEFAULT_RULE : null);
    }
    setSaving(false);
  };

  const cancel = () => {
    setForm(toForm(device));
    setRule(door ? device.rule ?? DEFAULT_RULE : null);
  };

  const remove = async () => {
    const ok = window.confirm(t('deviceSettings.confirmRemove', { name: device.name }));
    if (!ok) return;
    setDeleting(true);
    try {
      await devicesApi.remove(device.id);
      toast.success(t('deviceSettings.removed', { name: device.name }));
      router.replace(`/coops/${coop.coop.id}`);
    } catch (e) {
      toast.error(getErrorMessage(e, 'deviceSettings.removeFailed'));
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <BackLink href={`/devices/${device.id}`}>{device.name}</BackLink>
        <h1 className="text-[30px] font-extrabold leading-tight">{t('deviceSettings.title', { of: ofDevice(device) })}</h1>
        <p className="text-[15px] text-muted">
          {deviceTypeLabel(device.deviceType)}
          {device.hasLight ? t('deviceSettings.withLight') : ''}
        </p>
      </div>

      {door && rule ? (
        <>
          <Card as="section" className="space-y-7">
            <RuleMomentEditor
              kind="open"
              moment={getMoment(rule, 'open')}
              onChange={(moment) => setRule(withMoment(rule, 'open', moment))}
              otherDoors={otherDoors}
            />
            <div className="border-t border-line/60" />
            <RuleMomentEditor
              kind="close"
              moment={getMoment(rule, 'close')}
              onChange={(moment) => setRule(withMoment(rule, 'close', moment))}
              otherDoors={otherDoors}
            />
          </Card>

          {showLight && (
            <Card as="section">
              <LightSettings rule={rule} onChange={(patch) => setRule({ ...rule, ...patch })} />
            </Card>
          )}

          <Card>
            <RulePreview days={preview} error={previewError} loading={previewLoading} timeZone={tz} note={previewNote} />
          </Card>
        </>
      ) : (
        <Notice tone="info">{t('deviceSettings.feederNotice')}</Notice>
      )}

      <section aria-labelledby="device-title">
        <SectionTitle id="device-title">{t('deviceSettings.device')}</SectionTitle>
        <Card className="space-y-5">
          <Field label={t('common.name')} htmlFor="device-name">
            <TextInput
              id="device-name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              autoComplete="off"
              required
            />
          </Field>

          <Field label={t('common.role')} htmlFor="device-role">
            <Select
              id="device-role"
              value={form.role}
              onChange={(e) => setForm({ ...form, role: e.target.value as DeviceRole })}
              disabled={roleOptions.length === 1}
            >
              {roleOptions.map((role) => (
                <option key={role} value={role} disabled={role === 'main_door' && !!otherMainDoor}>
                  {roleLabel(role)}
                  {role === 'main_door' && otherMainDoor ? t('common.alreadyMainDoor', { name: otherMainDoor.name }) : ''}
                </option>
              ))}
            </Select>
          </Field>

          <Field label={t('common.strategy')} htmlFor="device-strategy" hint={strategyHint(form.strategy, form.role)}>
            <Select
              id="device-strategy"
              value={form.strategy}
              onChange={(e) => setForm({ ...form, strategy: e.target.value as Strategy })}
              disabled={strategyOptions.length === 1}
            >
              {strategyOptions.map((strategy) => (
                <option key={strategy} value={strategy}>
                  {strategyLabel(strategy)}
                </option>
              ))}
            </Select>
          </Field>

          <div className="border-t border-line/60 pt-4">
            <Switch
              id="device-enabled"
              label={t('deviceSettings.automation')}
              description={
                !door
                  ? t('deviceSettings.automationFeeder')
                  : form.enabled
                    ? t('deviceSettings.automationOn')
                    : t('deviceSettings.automationOff')
              }
              checked={form.enabled}
              onChange={(enabled) => setForm({ ...form, enabled })}
            />
          </div>
        </Card>
      </section>

      <div className="px-1">
        <Button variant="ghost" size="sm" className="!text-alert" onClick={remove} isLoading={deleting}>
          {!deleting && <Trash2 className="h-4 w-4" aria-hidden="true" />}
          {t('deviceSettings.remove')}
        </Button>
        <p className="mt-1 px-3.5 text-sm text-muted">{t('deviceSettings.removeHint')}</p>
      </div>

      {dirty && (
        <div className="sticky bottom-[76px] z-20 sm:bottom-4">
          <div className="flex items-center gap-2 rounded-2xl bg-surface/95 p-2 pl-4 shadow-[0_6px_24px_-8px_rgba(30,43,74,0.35)] ring-1 ring-line backdrop-blur">
            <p className="min-w-0 flex-1 text-[15px] font-bold" aria-live="polite">
              {t('deviceSettings.unsaved')}
            </p>
            <Button variant="ghost" size="sm" onClick={cancel} disabled={saving}>
              {t('common.cancel')}
            </Button>
            <Button size="sm" onClick={save} isLoading={saving} loadingText={t('common.saving')}>
              {t('common.save')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function DeviceSettingsPage() {
  const params = useParams<{ id: string }>();
  return (
    <AppShell>
      {/* Remonté à chaque appareil : les brouillons ne passent jamais d'un appareil à l'autre. */}
      <DeviceSettings key={params.id} />
    </AppShell>
  );
}
