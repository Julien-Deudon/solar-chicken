'use client';

import React, { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { Bell, Check, ChevronRight, Edit2, LogOut, Plus, Send, Trash2 } from 'lucide-react';
import { AppShell } from '@/components/layout/AppShell';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Chips } from '@/components/ui/Chips';
import { Input } from '@/components/ui/Input';
import { ErrorState, PageLoader } from '@/components/ui/States';
import { coopsApi, getErrorMessage, settingsApi } from '@/lib/api';
// translateNow : t() hors React, pour fromApi() appelé dans un chargement (pas de dépendance à la langue).
import { t as translateNow, useI18n } from '@/lib/i18n';
import { LOCALES, LOCALE_NAMES, type Locale } from '@/lib/locale';
import { useAuthStore } from '@/lib/store';
import type { CoopView, NotificationSettings, TelegramAccount } from '@/types';

/** Compte en cours d'édition (isNew : pas encore enregistré). */
type EditableAccount = TelegramAccount & { isNew?: boolean };

interface SettingsDraft extends Omit<NotificationSettings, 'telegramAccounts'> {
  telegramAccounts: EditableAccount[];
}

type Preference = 'notifyDailySchedule' | 'notifyOnOpen' | 'notifyOnClose' | 'notifyOnLight' | 'notifyOnError';

/** Libellés : settings.pref.<préférence>. */
const PREFERENCES: Preference[] = ['notifyDailySchedule', 'notifyOnOpen', 'notifyOnClose', 'notifyOnLight', 'notifyOnError'];

const TELEGRAM_LINK = 'text-ink hover:underline';

/** Comptes : nouveau format (liste) en priorité, sinon ancien format à un seul compte. */
function fromApi(data: NotificationSettings): SettingsDraft {
  const accounts: EditableAccount[] =
    data.telegramAccounts && data.telegramAccounts.length > 0
      ? data.telegramAccounts
      : data.telegramBotToken && data.telegramChatId
        ? [{ id: '1', name: translateNow('settings.mainAccount'), botToken: data.telegramBotToken, chatId: data.telegramChatId }]
        : [];
  return {
    ...data,
    telegramAccounts: accounts,
    notifyDailySchedule: data.notifyDailySchedule ?? true,
    notifyOnOpen: data.notifyOnOpen ?? true,
    notifyOnClose: data.notifyOnClose ?? true,
    notifyOnLight: data.notifyOnLight ?? true,
    notifyOnError: data.notifyOnError ?? true,
  };
}

function toApi(draft: SettingsDraft): Partial<NotificationSettings> {
  return {
    telegramAccounts: draft.telegramAccounts.map(({ id, name, botToken, chatId }) => ({
      id,
      name: name.trim(),
      botToken: botToken.trim(),
      chatId: chatId.trim(),
    })),
    notifyDailySchedule: draft.notifyDailySchedule,
    notifyOnOpen: draft.notifyOnOpen,
    notifyOnClose: draft.notifyOnClose,
    notifyOnLight: draft.notifyOnLight,
    notifyOnError: draft.notifyOnError,
  };
}

function NotificationSettingsScreen() {
  const { t, tRich } = useI18n();
  const [settings, setSettings] = useState<SettingsDraft | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [testingAccountId, setTestingAccountId] = useState<string | null>(null);
  const [editingAccountId, setEditingAccountId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      setSettings(fromApi(await settingsApi.getNotifications()));
    } catch (error) {
      setLoadError(getErrorMessage(error, 'settings.loadFailed'));
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (loadError) return <ErrorState message={loadError} onRetry={load} />;
  if (!settings) return <PageLoader />;

  const botFather = (
    <a href="https://t.me/BotFather" target="_blank" rel="noopener noreferrer" className={TELEGRAM_LINK}>
      @BotFather
    </a>
  );
  const userInfoBot = (
    <a href="https://t.me/userinfobot" target="_blank" rel="noopener noreferrer" className={TELEGRAM_LINK}>
      @userinfobot
    </a>
  );

  const handleSave = async () => {
    const incomplete = settings.telegramAccounts.find((a) => !a.botToken.trim() || !a.chatId.trim());
    if (incomplete) {
      toast.error(t('settings.incomplete', { name: incomplete.name }));
      setEditingAccountId(incomplete.id);
      return;
    }
    setSaving(true);
    try {
      const saved = await settingsApi.updateNotifications(toApi(settings));
      setSettings(fromApi(saved));
      setEditingAccountId(null);
      toast.success(t('settings.saved'));
    } catch (error) {
      toast.error(getErrorMessage(error, 'settings.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async (account: EditableAccount) => {
    setTestingAccountId(account.id);
    try {
      await settingsApi.testNotification({
        telegramBotToken: account.botToken.trim(),
        telegramChatId: account.chatId.trim(),
      });
      toast.success(t('settings.testSent', { name: account.name }));
    } catch (error) {
      toast.error(getErrorMessage(error, 'settings.testFailed'));
    } finally {
      setTestingAccountId(null);
    }
  };

  const addAccount = () => {
    const account: EditableAccount = {
      id: Date.now().toString(),
      name: t('settings.newAccountName', { n: settings.telegramAccounts.length + 1 }),
      botToken: '',
      chatId: '',
      isNew: true,
    };
    setSettings({ ...settings, telegramAccounts: [...settings.telegramAccounts, account] });
    setEditingAccountId(account.id);
  };

  const removeAccount = (account: EditableAccount) => {
    if (!account.isNew && !window.confirm(t('settings.confirmRemove', { name: account.name }))) return;
    setSettings({ ...settings, telegramAccounts: settings.telegramAccounts.filter((a) => a.id !== account.id) });
  };

  const updateAccount = (id: string, field: 'name' | 'botToken' | 'chatId', value: string) => {
    setSettings({
      ...settings,
      telegramAccounts: settings.telegramAccounts.map((a) => (a.id === id ? { ...a, [field]: value } : a)),
    });
  };

  return (
    <div className="space-y-4">
      <h2 className="px-1 text-[22px] font-bold">{t('settings.notifications')}</h2>

      <Card as="section">
        <div className="mb-4">
          <h2 className="flex items-center gap-2 text-lg font-semibold text-ink">
            <Bell className="h-5 w-5 text-ink" aria-hidden="true" />
            {t('settings.telegramAccounts')}
          </h2>
          <p className="mt-1 text-sm text-muted">{t('settings.telegramIntro')}</p>
        </div>

        <div className="space-y-4">
          {settings.telegramAccounts.map((account) => {
            const isEditing = editingAccountId === account.id || account.isNew;
            return (
              <div
                key={account.id}
                className={`space-y-3 rounded-xl p-4 ${
                  isEditing ? 'border-2 border-line bg-info' : 'border border-ok/30 bg-ok/10'
                }`}
              >
                <div className="flex items-center justify-between gap-2">
                  <h3 className="flex min-w-0 items-center gap-2 font-medium text-ink">
                    {!isEditing && <Check className="h-5 w-5 shrink-0 text-ok" aria-hidden="true" />}
                    <span className="truncate">{account.name}</span>
                  </h3>
                  <div className="flex items-center">
                    {!isEditing && (
                      <button
                        type="button"
                        onClick={() => setEditingAccountId(account.id)}
                        className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-xl text-ink hover:bg-info"
                        aria-label={t('settings.editAccount', { name: account.name })}
                      >
                        <Edit2 className="h-5 w-5" aria-hidden="true" />
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={() => removeAccount(account)}
                      className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-xl text-alert hover:bg-alert/10"
                      aria-label={t('settings.deleteAccount', { name: account.name })}
                    >
                      <Trash2 className="h-5 w-5" aria-hidden="true" />
                    </button>
                  </div>
                </div>

                {isEditing ? (
                  <>
                    <Input
                      label={t('settings.accountName')}
                      value={account.name}
                      onChange={(e) => updateAccount(account.id, 'name', e.target.value)}
                      placeholder={t('settings.accountNamePlaceholder')}
                      containerClassName=""
                    />
                    <Input
                      label={t('settings.botToken')}
                      type="password"
                      autoComplete="off"
                      value={account.botToken}
                      onChange={(e) => updateAccount(account.id, 'botToken', e.target.value)}
                      placeholder="123456:ABC-DEF…"
                      containerClassName=""
                      hint={tRich('settings.botTokenHint', { link: botFather })}
                    />
                    <Input
                      label={t('settings.chatId')}
                      inputMode="numeric"
                      value={account.chatId}
                      onChange={(e) => updateAccount(account.id, 'chatId', e.target.value)}
                      placeholder="123456789"
                      containerClassName=""
                      hint={tRich('settings.chatIdHint', { link: userInfoBot })}
                    />
                    {account.botToken && account.chatId && (
                      <Button
                        variant="secondary"
                        onClick={() => handleTest(account)}
                        isLoading={testingAccountId === account.id}
                        loadingText={t('common.sending')}
                        className="w-full"
                      >
                        <Send className="h-4 w-4" aria-hidden="true" />
                        {t('settings.test')}
                      </Button>
                    )}
                  </>
                ) : (
                  <p className="text-sm text-muted">{t('settings.configured')}</p>
                )}
              </div>
            );
          })}

          <Button variant="outline" onClick={addAccount} className="w-full">
            <Plus className="h-4 w-4" aria-hidden="true" />
            {t('settings.addAccount')}
          </Button>
        </div>
      </Card>

      <Card as="section">
        <h2 className="text-lg font-semibold text-ink">{t('settings.preferences')}</h2>
        <p className="mb-2 mt-1 text-sm text-muted">{t('settings.preferencesHint')}</p>
        <div className="divide-y divide-line">
          {PREFERENCES.map((key) => (
            <label key={key} className="flex min-h-[48px] cursor-pointer items-center gap-3 py-1">
              <input
                type="checkbox"
                checked={settings[key]}
                onChange={(e) => setSettings({ ...settings, [key]: e.target.checked })}
                className="h-5 w-5 rounded border-line text-ink accent-[rgb(var(--ink))]"
              />
              <span className="text-ink">{t(`settings.pref.${key}`)}</span>
            </label>
          ))}
        </div>
      </Card>

      <Button onClick={handleSave} isLoading={saving} loadingText={t('common.saving')} className="w-full">
        {t('settings.saveNotifications')}
      </Button>

      <Card>
        <h2 className="mb-2 font-medium text-ink">{t('settings.howTo')}</h2>
        <ol className="list-inside list-decimal space-y-1.5 text-sm text-muted">
          <li>{tRich('settings.howToStep1', { link: botFather })}</li>
          <li>{t('settings.howToStep2')}</li>
          <li>{tRich('settings.chatIdHint', { link: userInfoBot })}</li>
          <li>{t('settings.howToStep4')}</li>
        </ol>
      </Card>
    </div>
  );
}

function CoopAndAccount() {
  const { t } = useI18n();
  const router = useRouter();
  const { user, logout } = useAuthStore();
  const [coops, setCoops] = useState<CoopView[] | null>(null);

  useEffect(() => {
    coopsApi.list().then(setCoops).catch(() => setCoops([]));
  }, []);

  return (
    <>
      <section aria-labelledby="coop-settings-title">
        <h2 id="coop-settings-title" className="mb-2 px-1 text-[22px] font-bold">
          {t('settings.coop')}
        </h2>
        <ul className="overflow-hidden rounded-sheet bg-surface">
          {(coops ?? []).map((c, i) => (
            <li key={c.id} className={i > 0 ? 'border-t border-line/60' : ''}>
              <Link href={`/coops/${c.id}/settings`} className="flex min-h-[64px] items-center gap-3 px-4 py-3 hover:bg-ink/[0.03]">
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[17px] font-bold">{c.name}</span>
                  <span className="block text-[15px] text-muted">{t('settings.coopHint')}</span>
                </span>
                <ChevronRight className="h-5 w-5 shrink-0 text-muted" aria-hidden="true" />
              </Link>
            </li>
          ))}
          {coops === null && <li className="px-4 py-4 text-[15px] text-muted">{t('common.loading')}</li>}
        </ul>
      </section>

      <section aria-labelledby="account-title">
        <h2 id="account-title" className="mb-2 px-1 text-[22px] font-bold">
          {t('settings.account')}
        </h2>
        <div className="flex items-center gap-3 rounded-sheet bg-surface px-4 py-3">
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[17px] font-bold">
              {user ? `${user.firstName} ${user.lastName}`.trim() || user.email : t('settings.signedIn')}
            </span>
            {user?.email && <span className="block truncate text-[15px] text-muted">{user.email}</span>}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              logout();
              router.replace('/login');
            }}
          >
            <LogOut className="h-4 w-4" aria-hidden="true" />
            {t('settings.signOut')}
          </Button>
        </div>
      </section>
    </>
  );
}

/** Langue de l'interface, appliquée tout de suite et gardée dans ce navigateur. */
function LanguageSection() {
  const { locale, setLocale, t } = useI18n();
  return (
    <section aria-labelledby="language-title">
      <h2 id="language-title" className="mb-2 px-1 text-[22px] font-bold">
        {t('settings.languageTitle')}
      </h2>
      <div className="rounded-sheet bg-surface px-4 py-3">
        <Chips<Locale>
          label={t('settings.languageTitle')}
          value={locale}
          onChange={setLocale}
          options={LOCALES.map((l) => ({ value: l, label: LOCALE_NAMES[l], lang: l }))}
        />
        <p className="mt-2 text-sm text-muted">{t('settings.languageHint')}</p>
      </div>
    </section>
  );
}

function SettingsContent() {
  const { t } = useI18n();
  return (
    <div className="space-y-8">
      <h1 className="text-[30px] font-extrabold leading-tight">{t('settings.title')}</h1>
      <CoopAndAccount />
      <LanguageSection />
      <NotificationSettingsScreen />
    </div>
  );
}

export default function SettingsPage() {
  return (
    <AppShell>
      <SettingsContent />
    </AppShell>
  );
}
