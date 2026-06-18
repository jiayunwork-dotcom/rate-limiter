import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
  RuleConfig, RuleVersion, QuotaConfig, QuotaTreeNode, RateLimitEvent,
  TrafficSeriesPoint, TenantShareData, HeatmapData, AdaptiveStatus, AdaptiveConfigUpdate,
  RuleTemplate, AlertRule, AlertEvent, AlertStats,
  PaginatedAlertResult, PaginatedAlertRuleResult, AlertStatus, AlertSeverity,
  AlertAggregationRule, AlertSuppressionRule, AlertAggregationGroup,
  PaginatedAggregationRuleResult, PaginatedSuppressionRuleResult, PaginatedAggregationGroupResult,
  AggregationDimensionType, AuditLog, AuditStats, TimelineNode,
  PaginatedAuditLogResult, AuditOperationType, AuditResourceType
} from '../models/models';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private baseUrl = '/api/v1';

  constructor(private http: HttpClient) {}

  health(): Observable<any> {
    return this.http.get(`${this.baseUrl}/health`);
  }

  listRules(search?: string, enabled?: boolean): Observable<RuleConfig[]> {
    let params = new HttpParams();
    if (search) params = params.set('search', search);
    if (enabled !== undefined) params = params.set('enabled', String(enabled));
    return this.http.get<RuleConfig[]>(`${this.baseUrl}/rules`, { params });
  }

  getRule(id: string): Observable<RuleConfig> {
    return this.http.get<RuleConfig>(`${this.baseUrl}/rules/${id}`);
  }

  createRule(rule: Partial<RuleConfig>): Observable<RuleConfig> {
    return this.http.post<RuleConfig>(`${this.baseUrl}/rules`, rule);
  }

  updateRule(id: string, rule: Partial<RuleConfig>): Observable<RuleConfig> {
    return this.http.put<RuleConfig>(`${this.baseUrl}/rules/${id}`, rule);
  }

  deleteRule(id: string): Observable<any> {
    return this.http.delete(`${this.baseUrl}/rules/${id}`);
  }

  toggleRule(id: string): Observable<RuleConfig> {
    return this.http.patch<RuleConfig>(`${this.baseUrl}/rules/${id}/toggle`, {});
  }

  bulkToggleRules(ids: string[], enabled: boolean): Observable<any> {
    return this.http.post(`${this.baseUrl}/rules/bulk-toggle`, { ids, enabled });
  }

  getRuleVersions(id: string): Observable<RuleVersion[]> {
    return this.http.get<RuleVersion[]>(`${this.baseUrl}/rules/${id}/versions`);
  }

  rollbackRule(id: string, version: number): Observable<RuleConfig> {
    return this.http.post<RuleConfig>(`${this.baseUrl}/rules/${id}/rollback`, { version });
  }

  listEvents(params: {
    startTime?: string;
    endTime?: string;
    tenantId?: string;
    userId?: string;
    apiPath?: string;
    ruleId?: string;
    allowed?: boolean;
    page?: number;
    pageSize?: number;
  }): Observable<{ total: number; items: RateLimitEvent[] }> {
    let httpParams = new HttpParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== null) httpParams = httpParams.set(k, String(v));
    });
    return this.http.get<{ total: number; items: RateLimitEvent[] }>(`${this.baseUrl}/events`, { params: httpParams });
  }

  getTrafficSeries(startTime?: string, endTime?: string): Observable<TrafficSeriesPoint[]> {
    let params = new HttpParams();
    if (startTime) params = params.set('start_time', startTime);
    if (endTime) params = params.set('end_time', endTime);
    return this.http.get<TrafficSeriesPoint[]>(`${this.baseUrl}/dashboard/traffic`, { params });
  }

  getTenantShare(): Observable<TenantShareData[]> {
    return this.http.get<TenantShareData[]>(`${this.baseUrl}/dashboard/tenant-share`);
  }

  getHeatmap(): Observable<HeatmapData[]> {
    return this.http.get<HeatmapData[]>(`${this.baseUrl}/dashboard/heatmap`);
  }

  listQuotas(): Observable<QuotaConfig[]> {
    return this.http.get<QuotaConfig[]>(`${this.baseUrl}/quotas`);
  }

  getQuotaTree(): Observable<QuotaTreeNode[]> {
    return this.http.get<QuotaTreeNode[]>(`${this.baseUrl}/quotas/tree`);
  }

  upsertQuota(quota: Partial<QuotaConfig>): Observable<QuotaConfig> {
    return this.http.post<QuotaConfig>(`${this.baseUrl}/quotas`, quota);
  }

  deleteQuota(id: string): Observable<any> {
    return this.http.delete(`${this.baseUrl}/quotas/${id}`);
  }

  getAdaptiveStatus(): Observable<AdaptiveStatus> {
    return this.http.get<AdaptiveStatus>(`${this.baseUrl}/adaptive/status`);
  }

  updateAdaptiveConfig(config: AdaptiveConfigUpdate): Observable<any> {
    return this.http.put(`${this.baseUrl}/adaptive/config`, config);
  }

  overrideAdaptiveCoeff(coefficient: number): Observable<any> {
    return this.http.post(`${this.baseUrl}/adaptive/override`, { coefficient });
  }

  clearAdaptiveOverride(): Observable<any> {
    return this.http.delete(`${this.baseUrl}/adaptive/override`);
  }

  listTemplates(search?: string): Observable<{ total: number; data: RuleTemplate[] }> {
    let params = new HttpParams();
    if (search) params = params.set('search', search);
    return this.http.get<{ total: number; data: RuleTemplate[] }>(`${this.baseUrl}/templates`, { params });
  }

  listAllTemplates(): Observable<RuleTemplate[]> {
    return this.http.get<RuleTemplate[]>(`${this.baseUrl}/templates/all`);
  }

  getTemplate(id: string): Observable<RuleTemplate> {
    return this.http.get<RuleTemplate>(`${this.baseUrl}/templates/${id}`);
  }

  createTemplate(template: Partial<RuleTemplate>): Observable<RuleTemplate> {
    return this.http.post<RuleTemplate>(`${this.baseUrl}/templates`, template);
  }

  updateTemplate(id: string, template: Partial<RuleTemplate>): Observable<RuleTemplate> {
    return this.http.put<RuleTemplate>(`${this.baseUrl}/templates/${id}`, template);
  }

  deleteTemplate(id: string): Observable<any> {
    return this.http.delete(`${this.baseUrl}/templates/${id}`);
  }

  listAlertRules(params?: {
    search?: string;
    enabled?: boolean;
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedAlertRuleResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null) httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedAlertRuleResult>(`${this.baseUrl}/alert-rules`, { params: httpParams });
  }

  getAlertRule(id: string): Observable<AlertRule> {
    return this.http.get<AlertRule>(`${this.baseUrl}/alert-rules/${id}`);
  }

  createAlertRule(rule: Partial<AlertRule>): Observable<AlertRule> {
    return this.http.post<AlertRule>(`${this.baseUrl}/alert-rules`, rule);
  }

  updateAlertRule(id: string, rule: Partial<AlertRule>): Observable<AlertRule> {
    return this.http.put<AlertRule>(`${this.baseUrl}/alert-rules/${id}`, rule);
  }

  deleteAlertRule(id: string): Observable<any> {
    return this.http.delete(`${this.baseUrl}/alert-rules/${id}`);
  }

  toggleAlertRule(id: string, enabled: boolean): Observable<any> {
    return this.http.patch(`${this.baseUrl}/alert-rules/${id}/toggle`, { enabled });
  }

  listAlertEvents(params?: {
    status?: AlertStatus;
    severity?: AlertSeverity;
    ruleId?: string;
    dimensionType?: string;
    dimensionValue?: string;
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedAlertResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null && String(v) !== '') httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedAlertResult>(`${this.baseUrl}/alert-events`, { params: httpParams });
  }

  getAlertEvent(id: number): Observable<AlertEvent> {
    return this.http.get<AlertEvent>(`${this.baseUrl}/alert-events/${id}`);
  }

  acknowledgeAlert(id: number, acknowledgedBy?: string): Observable<any> {
    return this.http.post(`${this.baseUrl}/alert-events/${id}/acknowledge`, { acknowledgedBy });
  }

  getAlertStats(): Observable<AlertStats> {
    return this.http.get<AlertStats>(`${this.baseUrl}/alert-events/stats`);
  }

  listAggregationRules(params?: {
    enabled?: boolean;
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedAggregationRuleResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null && String(v) !== '') httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedAggregationRuleResult>(`${this.baseUrl}/alert-aggregation-rules`, { params: httpParams });
  }

  getAggregationRule(id: string): Observable<AlertAggregationRule> {
    return this.http.get<AlertAggregationRule>(`${this.baseUrl}/alert-aggregation-rules/${id}`);
  }

  createAggregationRule(rule: Partial<AlertAggregationRule>): Observable<AlertAggregationRule> {
    return this.http.post<AlertAggregationRule>(`${this.baseUrl}/alert-aggregation-rules`, rule);
  }

  updateAggregationRule(id: string, rule: Partial<AlertAggregationRule>): Observable<AlertAggregationRule> {
    return this.http.put<AlertAggregationRule>(`${this.baseUrl}/alert-aggregation-rules/${id}`, rule);
  }

  deleteAggregationRule(id: string): Observable<any> {
    return this.http.delete(`${this.baseUrl}/alert-aggregation-rules/${id}`);
  }

  toggleAggregationRule(id: string, enabled: boolean): Observable<any> {
    return this.http.patch(`${this.baseUrl}/alert-aggregation-rules/${id}/toggle`, { enabled });
  }

  listSuppressionRules(params?: {
    enabled?: boolean;
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedSuppressionRuleResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null && String(v) !== '') httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedSuppressionRuleResult>(`${this.baseUrl}/alert-suppression-rules`, { params: httpParams });
  }

  getSuppressionRule(id: string): Observable<AlertSuppressionRule> {
    return this.http.get<AlertSuppressionRule>(`${this.baseUrl}/alert-suppression-rules/${id}`);
  }

  createSuppressionRule(rule: Partial<AlertSuppressionRule>): Observable<AlertSuppressionRule> {
    return this.http.post<AlertSuppressionRule>(`${this.baseUrl}/alert-suppression-rules`, rule);
  }

  updateSuppressionRule(id: string, rule: Partial<AlertSuppressionRule>): Observable<AlertSuppressionRule> {
    return this.http.put<AlertSuppressionRule>(`${this.baseUrl}/alert-suppression-rules/${id}`, rule);
  }

  deleteSuppressionRule(id: string): Observable<any> {
    return this.http.delete(`${this.baseUrl}/alert-suppression-rules/${id}`);
  }

  toggleSuppressionRule(id: string, enabled: boolean): Observable<any> {
    return this.http.patch(`${this.baseUrl}/alert-suppression-rules/${id}/toggle`, { enabled });
  }

  listAggregationGroups(params?: {
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedAggregationGroupResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null && String(v) !== '') httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedAggregationGroupResult>(`${this.baseUrl}/alert-aggregation-groups`, { params: httpParams });
  }

  getAggregationGroupEvents(groupId: number, params?: {
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedAlertResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null && String(v) !== '') httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedAlertResult>(`${this.baseUrl}/alert-aggregation-groups/${groupId}/events`, { params: httpParams });
  }

  listAuditLogs(params?: {
    operator?: string;
    resourceType?: AuditResourceType;
    resourceId?: string;
    operationType?: AuditOperationType;
    startTime?: string;
    endTime?: string;
    page?: number;
    pageSize?: number;
  }): Observable<PaginatedAuditLogResult> {
    let httpParams = new HttpParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined && v !== null && String(v) !== '') httpParams = httpParams.set(k, String(v));
      });
    }
    return this.http.get<PaginatedAuditLogResult>(`${this.baseUrl}/audit-logs`, { params: httpParams });
  }

  getAuditLog(id: number): Observable<AuditLog> {
    return this.http.get<AuditLog>(`${this.baseUrl}/audit-logs/${id}`);
  }

  getAuditTimeline(resourceId: string): Observable<TimelineNode[]> {
    let httpParams = new HttpParams();
    httpParams = httpParams.set('resourceId', resourceId);
    return this.http.get<TimelineNode[]>(`${this.baseUrl}/audit-logs/timeline`, { params: httpParams });
  }

  getAuditStats(): Observable<AuditStats> {
    return this.http.get<AuditStats>(`${this.baseUrl}/audit-logs/stats`);
  }

  listAuditOperators(): Observable<string[]> {
    return this.http.get<string[]>(`${this.baseUrl}/audit-logs/operators`);
  }

  rollbackAuditOperation(id: number): Observable<any> {
    return this.http.post<any>(`${this.baseUrl}/audit-logs/${id}/rollback`, {});
  }
}
