export interface AuditEntry {
  id: string;
  timestamp: string;
  requestId?: string;
  method: string;
  path: string;
  route?: string;
  action?: string;
  status: number;
  durationMs: number;
  bytes: number;
  clientIp?: string;
  user?: string;
  role?: string;
  success: boolean;
  previousHash?: string;
  hash?: string;
  signature?: string;
}

export interface AuditLogResponse {
  total: number;
  items: AuditEntry[];
}

export interface AuditVerification {
  id: string;
  ok: boolean;
  message: string;
  previousHash?: string;
  hash?: string;
  signature?: string;
  verifiedAt: string;
}

export interface StreamEvent<T = unknown> {
  type: string;
  timestamp: string;
  payload: T;
}
