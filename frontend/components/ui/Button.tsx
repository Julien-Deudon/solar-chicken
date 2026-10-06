import React from 'react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
  isLoading?: boolean;
  /** Texte affiché pendant le chargement (par défaut : le contenu habituel). */
  loadingText?: React.ReactNode;
}

// primary : encre pleine (action principale d'un écran) ; outline/secondary : surface cerclée ;
// danger : rouge crête, réservé à ce qui est destructif ou à une porte en défaut.
export const buttonVariants = {
  primary: 'bg-ink text-canvas hover:bg-ink/90 active:bg-ink/80',
  secondary: 'bg-surface text-ink ring-1 ring-inset ring-line hover:bg-canvas active:bg-line/40',
  outline: 'bg-surface text-ink ring-1 ring-inset ring-line hover:bg-canvas active:bg-line/40',
  danger: 'bg-alert text-white hover:bg-alert/90 active:bg-alert/80',
  ghost: 'bg-transparent text-ink hover:bg-ink/5 active:bg-ink/10',
};

// Cibles tactiles : 44 px mini, 48 px pour les actions principales.
export const buttonSizes = {
  sm: 'min-h-[44px] px-3.5 text-[15px]',
  md: 'min-h-[48px] px-4 text-base',
  lg: 'min-h-[52px] px-6 text-lg',
};

export const buttonBase =
  'inline-flex select-none items-center justify-center gap-2 rounded-xl font-bold transition-colors disabled:cursor-not-allowed disabled:opacity-45';

export const Button: React.FC<ButtonProps> = ({
  children,
  variant = 'primary',
  size = 'md',
  isLoading = false,
  loadingText,
  className = '',
  disabled,
  type = 'button',
  ...props
}) => (
  <button
    type={type}
    className={`${buttonBase} ${buttonVariants[variant]} ${buttonSizes[size]} ${className}`}
    disabled={disabled || isLoading}
    aria-busy={isLoading || undefined}
    {...props}
  >
    {isLoading ? (
      <>
        <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden="true" />
        {loadingText ?? children}
      </>
    ) : (
      children
    )}
  </button>
);
