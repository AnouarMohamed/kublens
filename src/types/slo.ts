export type SLOStatus = "healthy" | "at_risk" | "breached";

export interface SLOSignal {
  label: string;
  value: string;
  tone: string;
}

export interface SLOObjective {
  id: string;
  name: string;
  category: string;
  summary: string;
  window: string;
  status: SLOStatus;
  targetValue: string;
  currentValue: string;
  targetPercent: number;
  currentPercent: number;
  errorBudgetUsedPercent: number;
  budgetRemainingPercent: number;
  burnRate: number;
  signals: SLOSignal[];
}

export interface SLOOverview {
  generatedAt: string;
  summary: string;
  healthyObjectives: number;
  atRiskObjectives: number;
  breachedObjectives: number;
  alerts: string[];
  objectives: SLOObjective[];
}
