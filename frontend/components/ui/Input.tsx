import React, { useId } from 'react';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  hint?: React.ReactNode;
  error?: string;
  containerClassName?: string;
}

// 16 px mini : évite le zoom automatique d'iOS au focus.
export const inputBaseClass =
  'block min-h-[48px] rounded-xl bg-canvas px-3.5 py-2 text-base text-ink ring-1 ring-inset ring-line placeholder:text-muted/70 focus:bg-surface focus:outline-none focus:ring-2 focus:ring-ink disabled:opacity-60';

export const inputClass = `${inputBaseClass} w-full`;

export const Input: React.FC<InputProps> = ({
  label,
  hint,
  error,
  className = '',
  containerClassName = 'mb-4',
  id,
  ...props
}) => {
  const autoId = useId();
  const inputId = id ?? autoId;
  const hintId = hint ? `${inputId}-hint` : undefined;
  const errorId = error ? `${inputId}-error` : undefined;

  return (
    <div className={containerClassName}>
      {label && (
        <label htmlFor={inputId} className="mb-1.5 block text-[15px] font-bold">
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`${inputClass} ${error ? '!ring-2 !ring-alert' : ''} ${className}`}
        aria-invalid={error ? true : undefined}
        aria-describedby={[hintId, errorId].filter(Boolean).join(' ') || undefined}
        {...props}
      />
      {hint && (
        <p id={hintId} className="mt-1.5 text-sm text-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="mt-1.5 text-sm font-bold text-alert">
          {error}
        </p>
      )}
    </div>
  );
};
