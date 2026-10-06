// Adresse de l'API vue depuis le serveur Next.js (réseau Docker : http://api:8080).
// Les rewrites sont figés au build : passer API_INTERNAL_URL au moment de `npm run build`.
const apiInternalUrl = (process.env.API_INTERNAL_URL || 'http://localhost:8080').replace(/\/+$/, '');

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: '/api/v2/:path*',
        destination: `${apiInternalUrl}/api/v2/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
