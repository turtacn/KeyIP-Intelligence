export type ServiceStatus = 'healthy' | 'degraded' | 'unhealthy' | 'not_configured';

export interface HealthServiceDetail {
  status: ServiceStatus;
  responseTime: number;
  message?: string;
  version?: string;
}

export interface HealthDetail {
  status: ServiceStatus;
  uptime: number;
  timestamp: string;
  services: Record<string, HealthServiceDetail>;
}

export interface HealthSummary {
  status: ServiceStatus;
  uptime: number;
  timestamp: string;
  servicesCount?: number;
}

/** Known backend services displayed on the health page */
export const KNOWN_SERVICES: string[] = [
  'PostgreSQL',
  'Redis',
  'Neo4j',
  'OpenSearch',
  'Milvus',
  'Kafka',
  'MinIO',
  'Keycloak',
];
