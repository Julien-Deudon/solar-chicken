import React from 'react';
import { inputClass } from '@/components/ui/Input';

interface FieldProps {
  label: React.ReactNode;
  htmlFor: string;
  hint?: React.ReactNode;
  error?: string | null;
  className?: string;
  children: React.ReactNode;
}

type Describable = React.ReactElement<{ 'aria-describedby'?: string }>;

/**
 * Libellé + champ + aide / erreur. Le champ enfant doit porter id={htmlFor} ;
 * s'il est seul, il reçoit aussi aria-describedby vers l'aide et l'erreur.
 */
export const Field: React.FC<FieldProps> = ({ label, htmlFor, hint, error, className = '', children }) => {
  const describedBy = [hint ? `${htmlFor}-hint` : null, error ? `${htmlFor}-error` : null]
    .filter(Boolean)
    .join(' ');
  const control =
    describedBy && React.isValidElement(children)
      ? React.cloneElement(children as Describable, {
          'aria-describedby': (children as Describable).props['aria-describedby'] ?? describedBy,
        })
      : children;

  return (
    <div className={className}>
      <label htmlFor={htmlFor} className="mb-1.5 block text-[15px] font-bold">
        {label}
      </label>
      {control}
      {hint && (
        <p id={`${htmlFor}-hint`} className="mt-1.5 text-sm text-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${htmlFor}-error`} className="mt-1.5 text-sm font-bold text-alert">
          {error}
        </p>
      )}
    </div>
  );
};

// Chevron neutre (gris moyen lisible en clair comme en sombre)
const CHEVRON =
  "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%238794A8' stroke-width='2.5' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")";

export const Select: React.FC<React.SelectHTMLAttributes<HTMLSelectElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <select className={`${inputClass} appearance-none bg-[length:16px] bg-[right_0.9rem_center] bg-no-repeat pr-10 ${className}`} style={{ backgroundImage: CHEVRON }} {...props}>
    {children}
  </select>
);

export const TextInput: React.FC<React.InputHTMLAttributes<HTMLInputElement>> = ({
  className = '',
  ...props
}) => <input className={`${inputClass} ${className}`} {...props} />;
