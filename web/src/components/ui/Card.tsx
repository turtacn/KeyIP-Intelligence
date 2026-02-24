import React from 'react';

interface CardProps {
  children: React.ReactNode;
  className?: string;
  padding?: 'none' | 'sm' | 'md' | 'lg';
  header?: React.ReactNode;
  footer?: React.ReactNode;
  bodyClassName?: string;
}

const Card: React.FC<CardProps> = ({
  children,
  className = '',
  padding = 'md',
  header,
  footer,
  bodyClassName = ''
}) => {
  const paddingStyles = {
    none: 'p-0',
    sm: 'p-4',
    md: 'p-6',
    lg: 'p-8',
  };

  return (
    <div className={`bg-white rounded-lg shadow-sm border border-slate-200 overflow-hidden flex flex-col ${className}`}>
      {header && (
        <div className="px-6 py-4 border-b border-slate-100 font-semibold text-slate-800 flex-shrink-0">
          {header}
        </div>
      )}
      <div className={`${paddingStyles[padding]} ${bodyClassName} flex-1 min-h-0`}>
        {children}
      </div>
      {footer && (
        <div className="px-6 py-4 border-t border-slate-100 bg-slate-50 flex-shrink-0">
          {footer}
        </div>
      )}
    </div>
  );
};

export default Card;
