import React from 'react';

// Un ciel de journée figé (lever vers 7 h, coucher vers 19 h), écho de la bande du poulailler.
const SKY =
  'linear-gradient(90deg, #1E2B4A 0%, #1E2B4A 22%, #4A5D8F 27%, #F5B935 30%, #F3D98F 32%, #C9DDEC 36%, #E3EEF6 55%, #C9DDEC 74%, #F3D98F 78%, #F5B935 80%, #4A5D8F 83%, #1E2B4A 88%, #1E2B4A 100%)';

/** Cadre des écrans sans connexion : le ciel, le nom, une phrase, puis le formulaire. */
export const AuthLayout: React.FC<{ title: string; subtitle: string; children: React.ReactNode }> = ({ title, subtitle, children }) => (
  <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-5 py-10">
    <div className="relative h-24 overflow-hidden rounded-[20px]" style={{ background: SKY }} aria-hidden="true">
      <span className="absolute left-[42%] top-7 h-5 w-5 rounded-full bg-[#F5B935] shadow-[0_0_18px_6px_rgba(245,185,53,0.55)] ring-2 ring-white" />
    </div>
    <h1 className="mt-7 text-[40px] font-extrabold leading-none tracking-[-0.02em]">{title}</h1>
    <p className="mt-2 text-lg text-muted">{subtitle}</p>
    <div className="mt-8">{children}</div>
  </main>
);
