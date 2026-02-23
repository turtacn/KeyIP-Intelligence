import React from 'react';
import Badge from './Badge';

export type Status = 'active' | 'pending' | 'completed' | 'error' | 'inactive';

interface StatusBadgeProps {
  status: Status;
  label?: string;
  className?: string;
}

const StatusBadge: React.FC<StatusBadgeProps> = ({ status, label, className = '' }) => {
  const variants = {
    active: 'success',
    pending: 'warning',
    completed: 'info',
    error: 'danger',
    inactive: 'default',
  } as const;

  return (
    <Badge
      variant={variants[status]}
      className={`capitalize ${className}`}
    >
      {label || status}
    </Badge>
  );
};

export default StatusBadge;
