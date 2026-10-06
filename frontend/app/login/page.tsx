'use client';

import React, { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { AuthLayout } from '@/components/layout/AuthLayout';
import { Button, buttonBase, buttonSizes, buttonVariants } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { authApi, getErrorMessage, systemApi } from '@/lib/api';
import { useI18n } from '@/lib/i18n';
import { useAuthStore } from '@/lib/store';
import type { Bootstrap } from '@/types';

export default function LoginPage() {
  const { t } = useI18n();
  const router = useRouter();
  const { setAuth } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [boot, setBoot] = useState<Bootstrap | null>(null);

  useEffect(() => {
    systemApi.bootstrap().then(setBoot).catch(() => setBoot(null));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    try {
      const response = await authApi.login({ email: email.trim(), password });
      setAuth(response.user, response.token);
      router.replace('/dashboard');
    } catch (error) {
      toast.error(getErrorMessage(error, 'auth.signInFailed'));
      setIsLoading(false);
    }
  };

  if (boot?.needsSetup) {
    return (
      <AuthLayout title="Solar Chicken" subtitle={t('auth.tagline')}>
        <p className="text-[17px] leading-snug">{t('auth.firstRun')}</p>
        <Link href="/register" className={`${buttonBase} ${buttonVariants.primary} ${buttonSizes.lg} mt-5 w-full`}>
          {t('auth.createTheAccount')}
        </Link>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Solar Chicken" subtitle={t('auth.tagline')}>
      <form onSubmit={handleSubmit}>
        <Input label={t('auth.email')} type="email" autoComplete="email" inputMode="email" autoCapitalize="off" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <Input label={t('auth.password')} type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        <Button type="submit" size="lg" className="mt-2 w-full" isLoading={isLoading} loadingText={t('auth.signingIn')}>
          {t('auth.signIn')}
        </Button>
      </form>
      {boot?.registrationOpen && (
        <p className="mt-6 text-center text-[15px] text-muted">
          {t('auth.noAccount')}{' '}
          <Link href="/register" className="font-bold text-ink underline">
            {t('auth.createAccount')}
          </Link>
        </p>
      )}
    </AuthLayout>
  );
}
