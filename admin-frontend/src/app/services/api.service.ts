import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
  RuleConfig, RuleVersion, QuotaConfig, QuotaTreeNode, RateLimitEvent,
  TrafficSeriesPoint, TenantShareData, HeatmapData, AdaptiveStatus, AdaptiveConfigUpdate,
  RuleTemplate
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
}
