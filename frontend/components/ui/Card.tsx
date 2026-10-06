import React from 'react';

interface CardProps {
  children: React.ReactNode;
  className?: string;
}

/** Surface d'une section : à plat, sans ombre. Les lignes à l'intérieur portent la structure. */
export const Card: React.FC<CardProps & { as?: 'div' | 'section' | 'article' }> = ({
  children,
  className = '',
  as: Tag = 'div',
}) => <Tag className={`rounded-sheet bg-surface p-4 sm:p-5 ${className}`}>{children}</Tag>;

export const CardHeader: React.FC<CardProps> = ({ children, className = '' }) => (
  <div className={`mb-3 ${className}`}>{children}</div>
);

export const CardTitle: React.FC<CardProps> = ({ children, className = '' }) => (
  <h2 className={`text-xl font-bold ${className}`}>{children}</h2>
);

export const CardContent: React.FC<CardProps> = ({ children, className = '' }) => (
  <div className={className}>{children}</div>
);

/** Titre de section posé sur le fond (hors surface). */
export const SectionTitle: React.FC<{ id?: string; children: React.ReactNode; action?: React.ReactNode }> = ({
  id,
  children,
  action,
}) => (
  <div className="mb-2 flex items-end justify-between gap-3 px-1">
    <h2 id={id} className="text-[22px] font-bold leading-tight">
      {children}
    </h2>
    {action}
  </div>
);
