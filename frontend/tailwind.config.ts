import type { Config } from 'tailwindcss';

// Couleurs sémantiques pilotées par des variables CSS (globals.css) : clair le jour, sombre la nuit
// (préférence du téléphone). Les teintes fixes du ciel servent à la bande « course du soleil ».
const v = (name: string) => `rgb(var(--${name}) / <alpha-value>)`;

const config: Config = {
  darkMode: 'media',
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        canvas: v('canvas'), // fond de l'app (brume)
        surface: v('surface'), // surfaces (blanc / nuit claire)
        ink: v('ink'), // texte principal (encre de nuit)
        muted: v('muted'), // texte secondaire (ardoise)
        line: v('line'), // séparateurs
        sun: v('sun'), // jaune d'œuf : ouverture, soleil, focus
        ok: v('ok'), // vert pré : fait, actif
        alert: v('alert'), // rouge crête : échec, défaut
        info: v('info'), // fond des notes d'information
        sky: {
          night: '#1E2B4A',
          blue: '#4A5D8F',
          dawn: '#F5B935',
          day: '#C9DDEC',
          noon: '#E3EEF6',
        },
      },
      fontFamily: {
        display: ['var(--font-display)', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        sans: ['var(--font-body)', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
      borderRadius: {
        sheet: '22px',
      },
    },
  },
  plugins: [],
};

export default config;
