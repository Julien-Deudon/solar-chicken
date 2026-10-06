'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/store';
import { PageLoader } from '@/components/ui/States';

export default function Home() {
  const router = useRouter();
  const { token } = useAuthStore();

  useEffect(() => {
    const hasToken = !!token || !!localStorage.getItem('token');
    router.replace(hasToken ? '/dashboard' : '/login');
  }, [token, router]);

  return <PageLoader />;
}
