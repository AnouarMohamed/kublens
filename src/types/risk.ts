export interface RiskCheck {
  name: string;
  category?: string;
  passed: boolean;
  detail: string;
  suggestion: string;
  score: number;
}

export interface RiskReport {
  score: number;
  level: string;
  summary: string;
  checks: RiskCheck[];
}

export interface RiskAnalyzeRequest {
  manifest: string;
}

export interface ResourceApplyRiskResponse {
  message: string;
  requiresForce: boolean;
  report: RiskReport;
}
