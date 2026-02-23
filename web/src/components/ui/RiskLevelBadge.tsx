import React from 'react';
import Badge from './Badge';

export type RiskLevel = 'HIGH' | 'MEDIUM' | 'LOW' | 'NONE';

interface RiskLevelBadgeProps {
  level: RiskLevel;
  className?: string;
}

const RiskLevelBadge: React.FC<RiskLevelBadgeProps> = ({ level, className = '' }) => {
  const variants = {
    HIGH: 'danger',
    MEDIUM: 'warning',
    LOW: 'info',
    NONE: 'success',
  } as const;

  return (
    <Badge
      variant={variants[level]}
      className={`font-semibold ${className}`}
    >
      {level} RISK
    </Badge>
  );
};

export default RiskLevelBadge;
