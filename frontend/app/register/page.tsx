'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { AuthLayout } from '@/components/layout/AuthLayout';
import { Button, buttonBase, buttonSizes, buttonVariants } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { authApi, getErrorMessage, getErrorStatus } from '@/lib/api';
import { useI18n } from '@/lib/i18n';
import { useAuthStore } from '@/lib/store';
import type { RegisterRequest } from '@/types';

export default function RegisterPage() {
  const { t } = useI18n();
  const router = useRouter();
  const { setAuth } = useAuthStore();
  const [form, setForm] = useState<RegisterRequest>({ email: '', password: '', firstName: '', lastName: '' });
  const [isLoading, setIsLoading] = useState(false);
  const [closed, setClosed] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    const email = form.email.trim();
    try {
      await authApi.register({ ...form, email });
      // Connexion directe, puis l'assistant de démarrage.
      const session = await authApi.login({ email, password: form.password });
      setAuth(session.user, session.token);
      router.replace('/dashboard');
    } catch (error) {
      if (getErrorStatus(error) === 403) setClosed(true);
      else toast.error(getErrorMessage(error, 'auth.registerFailed'));
      setIsLoading(false);
    }
  };

  const change = (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [e.target.name]: e.target.value });

  if (closed) {
    return (
      <AuthLayout title={t('auth.closedTitle')} subtitle={t('auth.closedText')}>
        <Link href="/login" className={`${buttonBase} ${buttonVariants.outline} ${buttonSizes.md} w-full`}>
          {t('auth.backToLogin')}
        </Link>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title={t('auth.registerTitle')} subtitle={t('auth.registerSubtitle')}>
      <form onSubmit={handleSubmit}>
        <div className="grid grid-cols-2 gap-3">
          <Input label={t('auth.firstName')} name="firstName" autoComplete="given-name" value={form.firstName} onChange={change} required />
          <Input label={t('auth.lastName')} name="lastName" autoComplete="family-name" value={form.lastName} onChange={change} required />
        </div>
        <Input label={t('auth.email')} name="email" type="email" autoComplete="email" inputMode="email" autoCapitalize="off" value={form.email} onChange={change} required />
        <Input label={t('auth.password')} name="password" type="password" autoComplete="new-password" minLength={8} value={form.password} onChange={change} hint={t('auth.passwordHint')} required />
        <Button type="submit" size="lg" className="mt-2 w-full" isLoading={isLoading} loadingText={t('auth.creating')}>
          {t('auth.registerSubmit')}
        </Button>
      </form>
      <p className="mt-6 text-center text-[15px] text-muted">
        {t('auth.haveAccount')}{' '}
        <Link href="/login" className="font-bold text-ink underline">
          {t('auth.signIn')}
        </Link>
      </p>
    </AuthLayout>
  );
}
