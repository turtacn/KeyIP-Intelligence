import React from 'react';

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info';
  size?: 'sm' | 'md';
  className?: string;
}

const Badge: React.FC<BadgeProps> = ({
  children,
  variant = 'default',
  size = 'md',
  className = ''
}) => {
  const variants = {
    default: 'bg-slate-100 text-slate-700 ring-slate-600/10',
    success: 'bg-green-50 text-green-700 ring-green-600/20',
    warning: 'bg-yellow-50 text-yellow-700 ring-yellow-600/20',
    danger: 'bg-red-50 text-red-700 ring-red-600/10',
    info: 'bg-blue-50 text-blue-700 ring-blue-700/10',
  };

  const sizes = {
    sm: 'px-1.5 py-0.5 text-xs',
    md: 'px-2.5 py-0.5 text-sm',
  };

  return (
    <span
      className={`
        inline-flex items-center rounded-md font-medium ring-1 ring-inset
        ${variants[variant]}
        ${sizes[size]}
        ${className}
      `}
    >
      {children}
    </span>
  );
};

export default Badge;
