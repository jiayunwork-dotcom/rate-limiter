export interface RuleConfig {
  id: string;
  name: string;
  apiPath: string;
  method: string;
  algorithm: AlgorithmType;
  enabled: boolean;
  version: number;
  limit: number;
  windowSeconds: number;
  dimensions: RuleDimensions;
  tokenBucketConfig?: TokenBucketConfig;
  leakyBucketConfig?: LeakyBucketConfig;
  shapingConfig?: ShapingConfig;
  grayRelease?: GrayReleaseConfig;
  createdAt: string;
  updatedAt: string;
}

export type AlgorithmType = 'token_bucket' | 'leaky_bucket' | 'fixed_window' | 'sliding_window' | 'sliding_log';

export interface RuleDimensions {
  dimensions: Dimension[];
  combineMode: 'AND' | 'OR';
}

export interface Dimension {
  type: DimensionType;
  value?: string;
  headerKey?: string;
}

export type DimensionType = 'api_path' | 'method' | 'user_id' | 'tenant_id' | 'client_ip' | 'header';

export interface TokenBucketConfig {
  refillRate: number;
  capacity: number;
  tokensPerReq: number;
}

export interface LeakyBucketConfig {
  outflowRate: number;
  capacity: number;
}

export interface ShapingConfig {
  enabled: boolean;
  maxQueueDepth: number;
  maxWaitMs: number;
  priorityEnabled: boolean;
}

export interface GrayReleaseConfig {
  enabled: boolean;
  trafficPercent: number;
  startAt?: string;
  fullAt?: string;
}

export interface RuleVersion {
  version: number;
  config: Partial<RuleConfig>;
  createdAt: string;
  changedBy: string;
}

export interface QuotaConfig {
  id: string;
  level: QuotaLevel;
  tenantId?: string;
  userId?: string;
  apiPath?: string;
  limit: number;
  windowSeconds: number;
  inherited: boolean;
  overrideValue?: number;
  currentUsage: number;
  usagePercent: number;
}

export type QuotaLevel = 'global' | 'tenant' | 'user' | 'api';

export interface QuotaTreeNode {
  id: string;
  name: string;
  level: QuotaLevel;
  limit: number;
  currentUsage: number;
  usagePercent: number;
  children?: QuotaTreeNode[];
  overQuota: boolean;
}

export interface RateLimitEvent {
  id: number;
  timestamp: string;
  requestId: string;
  allowed: boolean;
  ruleId: string;
  ruleName: string;
  limit: number;
  remaining: number;
  apiPath: string;
  method: string;
  userId: string;
  tenantId: string;
  clientIp: string;
  triggeredLevel: QuotaLevel;
  mode: 'distributed' | 'local';
}

export interface TrafficSeriesPoint {
  timestamp: string;
  apiPath: string;
  allowed: number;
  rejected: number;
}

export interface TenantShareData {
  tenantId: string;
  tenantName: string;
  requestCount: number;
  percentage: number;
}

export interface HeatmapData {
  hour: number;
  weekday: number;
  count: number;
}

export interface AdaptiveStatus {
  enabled: boolean;
  currentCoefficient: number;
  originalCoefficient: number;
  p99LatencyMs: number;
  targetP99LatencyMs: number;
  errorRate: number;
  errorRateThreshold: number;
  stableSince?: string;
  manualOverride: boolean;
  pidState: PIDState;
  latencyHistory: TimeSeriesPoint[];
  errorRateHistory: TimeSeriesPoint[];
  coefficientHistory: TimeSeriesPoint[];
}

export interface PIDState {
  kp: number;
  ki: number;
  kd: number;
  integral: number;
  lastError: number;
  output: number;
}

export interface TimeSeriesPoint {
  timestamp: string;
  value: number;
}

export interface AdaptiveConfigUpdate {
  targetP99LatencyMs: number;
  errorRateThreshold: number;
  minCoefficient: number;
  maxCoefficient: number;
  kp: number;
  ki: number;
  kd: number;
}

export interface RuleTemplate {
  id: string;
  name: string;
  description: string;
  algorithm: AlgorithmType;
  limit: number;
  windowSeconds: number;
  tokenBucketConfig?: TokenBucketConfig;
  leakyBucketConfig?: LeakyBucketConfig;
  shapingConfig?: ShapingConfig;
  createdAt: string;
  updatedAt: string;
}
