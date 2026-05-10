// Domain entity types

export type Jurisdiction = 'CN' | 'US' | 'EP' | 'JP' | 'KR' | 'Other';
export type RiskLevel = 'HIGH' | 'MEDIUM' | 'LOW' | 'NONE';

export interface MaterialProperty {
  type: string;
  value: number | string;
  unit: string;
  testCondition?: string;
  source?: string;
}

export interface Molecule {
  id: string;
  smiles: string;
  inchi?: string;
  molecularWeight?: number;
  fingerprint?: string;
  properties?: MaterialProperty[];
  name?: string; // Added for display
}

export interface Claim {
  id: string;
  patentId: string;
  type: 'independent' | 'dependent';
  text: string;
  elements?: string[];
  markushRanges?: string[];
}

export interface FamilyMember {
  id: string;
  patent_number: string;
  title: string;
  jurisdiction: string;
  filing_date: string;
  applicant: string;
  relationship: string;
  legal_status?: string;
}

export interface FamilyResponse {
  patent_id: string;
  patent_number: string;
  family_id?: string;
  members: FamilyMember[];
  total_members: number;
}

export interface Patent {
  id: string;
  publicationNumber: string;
  title: string;
  abstract: string;
  filingDate: string;
  publicationDate: string;
  grantDate?: string;
  legalStatus: string;
  ipcCodes: string[];
  assignee: string;
  inventors: string[];
  claims?: Claim[];
  citations?: string[];
}

export interface Company {
  id: string;
  name: string;
  country: string;
  portfolioSize: number;
  type?: string;
}

export interface InfringementAlert {
  id: string;
  targetPatentId: string;
  triggerMoleculeId: string;
  riskLevel: RiskLevel;
  literalScore: number;
  docScore?: number; // Doctrine of Equivalents score
  detectedAt: string;
  status: 'new' | 'reviewed' | 'escalated';
}

export interface LifecycleEvent {
  id: string;
  patentId: string;
  jurisdiction: Jurisdiction;
  eventType: string;
  dueDate: string;
  feeAmount?: number;
  currency?: string;
  status: 'pending' | 'completed' | 'overdue';
}

export interface PortfolioScore {
  coverage: number;
  concentration: number;
  aging: number;
  totalValue: number;
  healthGrade: 'A' | 'B' | 'C' | 'D' | 'F';
}

export interface ConstellationPoint {
  id: string;
  patent_number: string;
  x: number;
  y: number;
  point_type: 'own_patent' | 'competitor_patent' | 'public_patent' | 'molecule';
  assignee?: string;
  tech_domain?: string;
  value_score?: number;
  filing_year?: number;
  legal_status?: string;
  cluster_label?: string;
}

export interface ConstellationCluster {
  cluster_id: string;
  label: string;
  center_x: number;
  center_y: number;
  point_count: number;
  tech_domain?: string;
}

export interface WhiteSpaceRegion {
  region_id: string;
  center_x: number;
  center_y: number;
  description?: string;
  tech_domains?: string[];
  score: number;
}

export interface ConstellationData {
  portfolio_id: string;
  points: ConstellationPoint[];
  clusters?: ConstellationCluster[];
  white_spaces?: WhiteSpaceRegion[];
  total_points: number;
}

export interface DashboardMetrics {
  totalPatents: number;
  activePatents: number;
  pendingPatents: number;
  highRiskAlerts: number;
  dueThisMonth: number;
  monthlyApplicationTrend: { month: string; filed: number; granted: number }[];
  jurisdictionBreakdown: { jurisdiction: string; count: number }[];
  competitorComparison: { name: string; portfolioSize: number }[];
  upcomingDeadlines: LifecycleEvent[];
  recentAlerts: InfringementAlert[];
  portfolioHealthScore: number;
}
