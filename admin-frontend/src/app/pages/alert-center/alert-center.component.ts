import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatTabsModule } from '@angular/material/tabs';
import { MatDialogModule, MatDialog } from '@angular/material/dialog';
import {
  AlertEvent, AlertRule, AlertStats, AlertStatus, AlertSeverity,
  AlertTriggerType, AlertScopeType, PaginatedAlertResult, PaginatedAlertRuleResult,
  AlertAggregationRule, AlertSuppressionRule, AlertAggregationGroup,
  PaginatedAggregationRuleResult, PaginatedSuppressionRuleResult, PaginatedAggregationGroupResult,
  AggregationDimensionType
} from '../../models/models';
import { ApiService } from '../../services/api.service';
import { WebSocketService } from '../../services/websocket.service';
import { Subscription } from 'rxjs';

@Component({
  selector: 'app-alert-center',
  standalone: true,
  imports: [
    CommonModule, FormsModule,
    MatTableModule, MatInputModule, MatSelectModule, MatButtonModule,
    MatPaginatorModule, MatIconModule, MatCardModule, MatSlideToggleModule,
    MatFormFieldModule, MatTabsModule, MatDialogModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">告警中心</h1>
      <div class="header-actions">
        <button mat-stroked-button (click)="refreshAll()">
          <mat-icon>refresh</mat-icon>刷新
        </button>
      </div>
    </div>

    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">触发中告警</div>
        <div class="stat-value" style="color: #f44336;">{{ stats.firingCount }}</div>
        <div class="stat-change">当前正在触发的告警(不含被抑制)</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">今日新增</div>
        <div class="stat-value" style="color: #ff9800;">{{ stats.todayNewCount }}</div>
        <div class="stat-change">今天新增告警数</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">本周总量</div>
        <div class="stat-value" style="color: #2196f3;">{{ stats.weekTotalCount }}</div>
        <div class="stat-change">近7天告警总量</div>
      </div>
    </div>

    <div class="alert-layout">
      <div class="alert-main">
        <div class="card">
          <div class="card-header">
            告警列表
            <div class="header-right">
              <span style="margin-right:16px;">
                <mat-slide-toggle [(ngModel)]="aggregateView" (change)="onAggregateViewChange()">
                  聚合视图
                </mat-slide-toggle>
              </span>
              <span style="margin-right:16px;">
                <mat-slide-toggle [(ngModel)]="showSuppressed" (change)="onShowSuppressedChange()">
                  显示被抑制告警
                </mat-slide-toggle>
              </span>
              <span style="color:#666;font-size:13px;">
                共 {{ totalCount }} 条记录
              </span>
            </div>
          </div>
          <div class="card-content" style="padding: 16px 24px 0;">
            <div class="filter-bar">
              <mat-form-field appearance="outline" style="width:140px;">
                <mat-label>状态</mat-label>
                <mat-select [(ngModel)]="filters.status" (selectionChange)="loadAlerts(1)">
                  <mat-option value="">全部</mat-option>
                  <mat-option value="firing">触发中</mat-option>
                  <mat-option value="acknowledged">已确认</mat-option>
                  <mat-option value="resolved">已恢复</mat-option>
                  <mat-option value="expired">已过期</mat-option>
                </mat-select>
              </mat-form-field>
              <mat-form-field appearance="outline" style="width:140px;">
                <mat-label>严重等级</mat-label>
                <mat-select [(ngModel)]="filters.severity" (selectionChange)="loadAlerts(1)">
                  <mat-option value="">全部</mat-option>
                  <mat-option value="critical">严重</mat-option>
                  <mat-option value="warning">警告</mat-option>
                  <mat-option value="info">提示</mat-option>
                </mat-select>
              </mat-form-field>
              <mat-form-field appearance="outline" style="width:180px;">
                <mat-label>规则ID</mat-label>
                <input matInput [(ngModel)]="filters.ruleId" (keyup.enter)="loadAlerts(1)">
              </mat-form-field>
              <mat-form-field appearance="outline" style="width:200px;">
                <mat-label>维度值</mat-label>
                <input matInput [(ngModel)]="filters.dimensionValue" (keyup.enter)="loadAlerts(1)">
              </mat-form-field>
            </div>
          </div>
          <div class="card-content" style="padding:0;">
            <ng-container *ngIf="!aggregateView">
              <table mat-table [dataSource]="alerts" class="full-width-table">
                <ng-container matColumnDef="severity">
                  <th mat-header-cell *matHeaderCellDef style="width:80px;">等级</th>
                  <td mat-cell *matCellDef="let row">
                    <span class="severity-badge" [ngClass]="'sev-' + row.severity"
                          [style.opacity]="row.suppressed ? 0.5 : 1">
                      {{ getSeverityLabel(row.severity) }}
                    </span>
                  </td>
                </ng-container>
                <ng-container matColumnDef="status">
                  <th mat-header-cell *matHeaderCellDef style="width:90px;">状态</th>
                  <td mat-cell *matCellDef="let row">
                    <span class="status-badge" [ngClass]="'status-' + row.status"
                          [style.opacity]="row.suppressed ? 0.5 : 1">
                      {{ getStatusLabel(row.status) }}
                    </span>
                    <span *ngIf="row.suppressed" class="suppressed-tag">
                      已抑制(被{{ row.suppressedByRuleId }})
                    </span>
                  </td>
                </ng-container>
                <ng-container matColumnDef="ruleName">
                  <th mat-header-cell *matHeaderCellDef>告警规则</th>
                  <td mat-cell *matCellDef="let row">
                    <div [ngClass]="{'suppressed-text': row.suppressed}">{{ row.ruleName }}</div>
                    <div style="font-size:11px;color:#999;">{{ row.alertRuleId }}</div>
                  </td>
                </ng-container>
                <ng-container matColumnDef="dimension">
                  <th mat-header-cell *matHeaderCellDef style="width:220px;">触发维度</th>
                  <td mat-cell *matCellDef="let row">
                    <div style="font-size:12px;">
                      <span style="color:#666;">{{ row.dimensionType }}:</span>
                    </div>
                    <div style="font-size:12px;font-weight:500;word-break:break-all;"
                         [ngClass]="{'suppressed-text': row.suppressed}">
                      {{ row.dimensionValue }}
                    </div>
                  </td>
                </ng-container>
                <ng-container matColumnDef="value">
                  <th mat-header-cell *matHeaderCellDef style="width:140px;">当前值/阈值</th>
                  <td mat-cell *matCellDef="let row">
                    <div style="font-size:13px;" [ngClass]="{'suppressed-text': row.suppressed}">
                      <span [ngClass]="{'text-red': row.currentValue > row.thresholdValue}">
                        {{ formatValue(row.currentValue) }}
                      </span>
                      <span style="color:#999;"> / {{ formatValue(row.thresholdValue) }}</span>
                    </div>
                  </td>
                </ng-container>
                <ng-container matColumnDef="time">
                  <th mat-header-cell *matHeaderCellDef style="width:160px;">触发时间</th>
                  <td mat-cell *matCellDef="let row">
                    <span [ngClass]="{'suppressed-text': row.suppressed}">
                      {{ formatTime(row.createdAt) }}
                    </span>
                  </td>
                </ng-container>
                <ng-container matColumnDef="actions">
                  <th mat-header-cell *matHeaderCellDef style="width:100px;">操作</th>
                  <td mat-cell *matCellDef="let row">
                    <button *ngIf="row.status === 'firing' && !row.suppressed"
                            mat-button color="primary" (click)="acknowledge(row)">
                      确认
                    </button>
                    <span *ngIf="row.suppressed" style="font-size:12px;color:#999;">被抑制</span>
                    <span *ngIf="row.status !== 'firing' && !row.suppressed"
                          style="color:#999;font-size:13px;">
                      已处理
                    </span>
                  </td>
                </ng-container>

                <ng-container matColumnDef="detail">
                  <td mat-cell *matCellDef="let row" [attr.colspan]="8" class="detail-cell">
                    <div class="detail-content" *ngIf="expandedId === row.id">
                      <div class="detail-row">
                        <span class="detail-label">告警详情：</span>
                      </div>
                      <div class="detail-row">
                        <span class="detail-label">首次触发时间：</span>
                        <span>{{ formatTime(row.firingStartedAt) }}</span>
                      </div>
                      <div class="detail-row">
                        <span class="detail-label">最近触发时间：</span>
                        <span>{{ formatTime(row.lastFiringAt) }}</span>
                      </div>
                      <div class="detail-row" *ngIf="row.acknowledgedBy">
                        <span class="detail-label">确认人：</span>
                        <span>{{ row.acknowledgedBy }}</span>
                      </div>
                      <div class="detail-row" *ngIf="row.acknowledgedAt">
                        <span class="detail-label">确认时间：</span>
                        <span>{{ formatTime(row.acknowledgedAt) }}</span>
                      </div>
                      <div class="detail-row" *ngIf="row.resolvedAt">
                        <span class="detail-label">恢复时间：</span>
                        <span>{{ formatTime(row.resolvedAt) }}</span>
                      </div>
                      <div class="detail-row" *ngIf="row.suppressed">
                        <span class="detail-label">抑制状态：</span>
                        <span>被规则 {{ row.suppressedByRuleId }} 抑制</span>
                      </div>
                      <div class="detail-row" *ngIf="row.triggerSnapshot">
                        <span class="detail-label">触发快照：</span>
                        <code class="snapshot-code">{{ JSON.stringify(row.triggerSnapshot, null, 2) }}</code>
                      </div>
                    </div>
                  </td>
                </ng-container>

                <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
                <tr mat-row *matRowDef="let row; columns: displayedColumns;"
                    (click)="toggleExpand(row)" class="clickable-row"
                    [ngClass]="{'expanded': expandedId === row.id, 'suppressed-row': row.suppressed}"></tr>
                <tr mat-row *matRowDef="let row; columns: ['detail']; when: isDetailRow"></tr>
              </table>

              <div *ngIf="!alerts.length" style="text-align:center;padding:48px;color:#999;">
                暂无告警数据
              </div>
            </ng-container>

            <ng-container *ngIf="aggregateView">
              <table mat-table [dataSource]="aggregationGroups" class="full-width-table">
                <ng-container matColumnDef="expand">
                  <th mat-header-cell *matHeaderCellDef style="width:40px;"></th>
                  <td mat-cell *matCellDef="let row">
                    <button mat-icon-button class="expand-btn"
                            (click)="toggleGroupExpand(row, $event)">
                      <mat-icon>{{ expandedGroupId === row.id ? 'expand_more' : 'chevron_right' }}</mat-icon>
                    </button>
                  </td>
                </ng-container>
                <ng-container matColumnDef="severity">
                  <th mat-header-cell *matHeaderCellDef style="width:80px;">等级</th>
                  <td mat-cell *matCellDef="let row">
                    <span class="severity-badge" [ngClass]="'sev-' + row.severity">
                      {{ getSeverityLabel(row.severity) }}
                    </span>
                  </td>
                </ng-container>
                <ng-container matColumnDef="status">
                  <th mat-header-cell *matHeaderCellDef style="width:90px;">状态</th>
                  <td mat-cell *matCellDef="let row">
                    <span class="status-badge" [ngClass]="'status-' + row.status">
                      {{ getStatusLabel(row.status) }}
                    </span>
                  </td>
                </ng-container>
                <ng-container matColumnDef="dimensionType">
                  <th mat-header-cell *matHeaderCellDef style="width:120px;">聚合维度</th>
                  <td mat-cell *matCellDef="let row">
                    {{ getDimensionTypeLabel(row.dimensionType) }}
                  </td>
                </ng-container>
                <ng-container matColumnDef="dimensionValue">
                  <th mat-header-cell *matHeaderCellDef>维度值</th>
                  <td mat-cell *matCellDef="let row">
                    <div style="font-weight:500;">{{ row.dimensionValue }}</div>
                    <div style="font-size:12px;color:#666;margin-top:2px;">
                      触发 {{ row.triggerCount }} 次 · 涉及 {{ row.uniqueValues?.length || 1 }} 个维度值
                    </div>
                  </td>
                </ng-container>
                <ng-container matColumnDef="time">
                  <th mat-header-cell *matHeaderCellDef style="width:200px;">时间范围</th>
                  <td mat-cell *matCellDef="let row">
                    <div style="font-size:12px;">
                      首次: {{ formatTime(row.firstTriggeredAt) }}
                    </div>
                    <div style="font-size:12px;color:#666;">
                      最近: {{ formatTime(row.lastTriggeredAt) }}
                    </div>
                  </td>
                </ng-container>

                <ng-container matColumnDef="groupDetail">
                  <td mat-cell *matCellDef="let row" [attr.colspan]="6" class="detail-cell">
                    <div class="detail-content" *ngIf="expandedGroupId === row.id">
                      <div class="detail-row">
                        <span class="detail-label">聚合规则ID：</span>
                        <span>{{ row.aggregationRuleId }}</span>
                      </div>
                      <div class="detail-row">
                        <span class="detail-label">窗口结束时间：</span>
                        <span>{{ formatTime(row.windowEndsAt) }}</span>
                      </div>
                      <div class="detail-row">
                        <span class="detail-label">涉及维度值：</span>
                      </div>
                      <div class="unique-values-list">
                        <span *ngFor="let val of row.uniqueValues" class="unique-value-tag">
                          {{ val }}
                        </span>
                      </div>
                      <div class="detail-row" style="margin-top:12px;">
                        <button mat-button (click)="loadGroupEvents(row)">
                          查看原始告警列表
                        </button>
                      </div>
                      <div *ngIf="groupEventsMap[row.id]?.length" style="margin-top:12px;">
                        <div style="font-weight:500;margin-bottom:8px;">原始告警(前{{ groupEventsMap[row.id].length }}条):</div>
                        <div *ngFor="let evt of groupEventsMap[row.id]" class="nested-alert-item">
                          <span class="severity-badge small" [ngClass]="'sev-' + evt.severity">
                            {{ getSeverityLabel(evt.severity) }}
                          </span>
                          <span style="margin-left:8px;">{{ evt.ruleName }}</span>
                          <span style="margin-left:8px;color:#666;">
                            {{ evt.dimensionType }}: {{ evt.dimensionValue }}
                          </span>
                          <span style="float:right;color:#999;font-size:12px;">
                            {{ formatTime(evt.createdAt) }}
                          </span>
                        </div>
                      </div>
                    </div>
                  </td>
                </ng-container>

                <tr mat-header-row *matHeaderRowDef="aggDisplayedColumns"></tr>
                <tr mat-row *matRowDef="let row; columns: aggDisplayedColumns;"
                    (click)="toggleGroupExpand(row)" class="clickable-row"
                    [ngClass]="{'expanded': expandedGroupId === row.id}"></tr>
                <tr mat-row *matRowDef="let row; columns: ['groupDetail']; when: isGroupDetailRow"></tr>
              </table>

              <div *ngIf="!aggregationGroups.length" style="text-align:center;padding:48px;color:#999;">
                暂无聚合告警数据
              </div>
            </ng-container>

            <div style="padding:16px 24px;border-top:1px solid #eee;">
              <mat-paginator
                [length]="totalCount"
                [pageSize]="pageSize"
                [pageSizeOptions]="[20, 50, 100]"
                [pageIndex]="page - 1"
                (page)="onPageChange($event)">
              </mat-paginator>
            </div>
          </div>
        </div>
      </div>

      <div class="alert-sidebar">
        <div class="card">
          <mat-tabs>
            <mat-tab label="告警规则">
              <ng-template matTabContent>
                <div class="tab-content">
                  <div class="tab-header">
                    <button mat-icon-button (click)="openRuleDialog()">
                      <mat-icon>add</mat-icon>
                    </button>
                  </div>
                  <div *ngFor="let rule of alertRules" class="rule-item">
                    <div class="rule-header">
                      <span class="rule-name" [ngClass]="{'muted': !rule.enabled}">{{ rule.name }}</span>
                      <mat-slide-toggle [checked]="rule.enabled"
                                        (change)="toggleRule(rule)"
                                        style="transform: scale(0.8);">
                      </mat-slide-toggle>
                    </div>
                    <div class="rule-meta">
                      <span class="severity-dot" [ngClass]="'dot-' + rule.severity"></span>
                      {{ getSeverityLabel(rule.severity) }}
                      <span style="margin-left:8px;color:#999;">|</span>
                      <span style="margin-left:8px;">{{ getTriggerTypeLabel(rule.triggerType) }}</span>
                    </div>
                    <div class="rule-scope">
                      范围: {{ getScopeLabel(rule.scopeType) }}
                      <span *ngIf="rule.scopeValue" style="color:#666;"> ({{ rule.scopeValue }})</span>
                    </div>
                    <div class="rule-actions">
                      <button mat-button size="small" (click)="editRule(rule)">编辑</button>
                      <button mat-button color="warn" size="small" (click)="deleteRule(rule)">删除</button>
                    </div>
                  </div>
                  <div *ngIf="!alertRules.length" style="text-align:center;padding:24px;color:#999;font-size:13px;">
                    暂无告警规则
                  </div>
                </div>
              </ng-template>
            </mat-tab>
            <mat-tab label="聚合规则">
              <ng-template matTabContent>
                <div class="tab-content">
                  <div class="tab-header">
                    <button mat-icon-button (click)="openAggRuleDialog()">
                      <mat-icon>add</mat-icon>
                    </button>
                  </div>
                  <div *ngFor="let rule of aggregationRules" class="rule-item">
                    <div class="rule-header">
                      <span class="rule-name" [ngClass]="{'muted': !rule.enabled}">{{ rule.name }}</span>
                      <mat-slide-toggle [checked]="rule.enabled"
                                        (change)="toggleAggRule(rule)"
                                        style="transform: scale(0.8);">
                      </mat-slide-toggle>
                    </div>
                    <div class="rule-meta">
                      维度: {{ getDimensionTypeLabel(rule.dimensionType) }}
                    </div>
                    <div class="rule-scope">
                      窗口: {{ rule.windowSeconds }}秒
                    </div>
                    <div class="rule-actions">
                      <button mat-button size="small" (click)="editAggRule(rule)">编辑</button>
                      <button mat-button color="warn" size="small" (click)="deleteAggRule(rule)">删除</button>
                    </div>
                  </div>
                  <div *ngIf="!aggregationRules.length" style="text-align:center;padding:24px;color:#999;font-size:13px;">
                    暂无聚合规则
                  </div>
                </div>
              </ng-template>
            </mat-tab>
            <mat-tab label="抑制规则">
              <ng-template matTabContent>
                <div class="tab-content">
                  <div class="tab-header">
                    <button mat-icon-button (click)="openSuppRuleDialog()">
                      <mat-icon>add</mat-icon>
                    </button>
                  </div>
                  <div *ngFor="let rule of suppressionRules" class="rule-item">
                    <div class="rule-header">
                      <span class="rule-name" [ngClass]="{'muted': !rule.enabled}">{{ rule.name }}</span>
                      <mat-slide-toggle [checked]="rule.enabled"
                                        (change)="toggleSuppRule(rule)"
                                        style="transform: scale(0.8);">
                      </mat-slide-toggle>
                    </div>
                    <div class="rule-meta">
                      <span class="severity-dot" [ngClass]="'dot-' + rule.sourceSeverity"></span>
                      {{ getSeverityLabel(rule.sourceSeverity) }} 源 →
                      {{ getSeverityLabel(rule.targetSeverity) }} 目标
                    </div>
                    <div class="rule-scope">
                      匹配字段: {{ rule.matchDimensionFields || '无' }}
                    </div>
                    <div class="rule-actions">
                      <button mat-button size="small" (click)="editSuppRule(rule)">编辑</button>
                      <button mat-button color="warn" size="small" (click)="deleteSuppRule(rule)">删除</button>
                    </div>
                  </div>
                  <div *ngIf="!suppressionRules.length" style="text-align:center;padding:24px;color:#999;font-size:13px;">
                    暂无抑制规则
                  </div>
                </div>
              </ng-template>
            </mat-tab>
          </mat-tabs>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .alert-layout {
      display: flex;
      gap: 24px;
      align-items: flex-start;
    }
    .alert-main {
      flex: 1;
      min-width: 0;
    }
    .alert-sidebar {
      width: 340px;
      flex-shrink: 0;
    }
    .header-right {
      float: right;
      display: flex;
      align-items: center;
    }
    .header-actions {
      display: flex;
      gap: 12px;
    }
    .tab-content {
      padding: 12px;
      max-height: 600px;
      overflow-y: auto;
    }
    .tab-header {
      display: flex;
      justify-content: flex-end;
      margin-bottom: 8px;
    }
    .severity-badge {
      padding: 3px 10px;
      border-radius: 4px;
      font-size: 12px;
      font-weight: 500;
    }
    .severity-badge.small {
      padding: 1px 6px;
      font-size: 11px;
    }
    .sev-critical { background: #ffebee; color: #c62828; }
    .sev-warning { background: #fff8e1; color: #f57f17; }
    .sev-info { background: #e3f2fd; color: #1565c0; }

    .status-badge {
      padding: 3px 10px;
      border-radius: 4px;
      font-size: 12px;
      font-weight: 500;
    }
    .status-firing { background: #ffebee; color: #c62828; }
    .status-acknowledged { background: #fff8e1; color: #f57f17; }
    .status-resolved { background: #e8f5e9; color: #2e7d32; }
    .status-expired { background: #f5f5f5; color: #9e9e9e; }

    .suppressed-tag {
      display: block;
      margin-top: 4px;
      font-size: 11px;
      color: #999;
      font-style: italic;
    }
    .suppressed-text {
      color: #999;
      font-style: italic;
    }
    .suppressed-row {
      opacity: 0.7;
    }

    .clickable-row {
      cursor: pointer;
      transition: background 0.2s;
    }
    .clickable-row:hover {
      background: #f5f5f5;
    }
    .clickable-row.expanded {
      background: #fafafa;
    }
    .detail-cell {
      padding: 0 !important;
    }
    .detail-content {
      padding: 16px 24px;
      background: #fafafa;
      border-top: 1px solid #eee;
      border-bottom: 1px solid #eee;
    }
    .detail-row {
      margin-bottom: 8px;
      font-size: 13px;
    }
    .detail-label {
      color: #666;
      margin-right: 8px;
    }
    .snapshot-code {
      display: block;
      background: #f5f5f5;
      padding: 12px;
      border-radius: 4px;
      font-size: 12px;
      white-space: pre-wrap;
      margin-top: 8px;
    }
    .text-red {
      color: #f44336;
      font-weight: 500;
    }
    .expand-btn {
      width: 32px;
      height: 32px;
      line-height: 32px;
      padding: 0;
    }
    .unique-values-list {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      margin-top: 4px;
    }
    .unique-value-tag {
      background: #e3f2fd;
      color: #1565c0;
      padding: 2px 8px;
      border-radius: 3px;
      font-size: 12px;
    }
    .nested-alert-item {
      padding: 8px 12px;
      background: #fff;
      border: 1px solid #eee;
      border-radius: 4px;
      margin-bottom: 4px;
      font-size: 12px;
    }

    .rule-item {
      padding: 12px;
      border: 1px solid #eee;
      border-radius: 6px;
      margin-bottom: 8px;
    }
    .rule-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 6px;
    }
    .rule-name {
      font-weight: 500;
      font-size: 14px;
    }
    .rule-name.muted {
      color: #999;
    }
    .rule-meta {
      font-size: 12px;
      color: #666;
      margin-bottom: 4px;
      display: flex;
      align-items: center;
    }
    .severity-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      display: inline-block;
      margin-right: 4px;
    }
    .dot-critical { background: #f44336; }
    .dot-warning { background: #ff9800; }
    .dot-info { background: #2196f3; }

    .rule-scope {
      font-size: 12px;
      color: #999;
      margin-bottom: 8px;
    }
    .rule-actions {
      display: flex;
      gap: 8px;
    }
    .rule-actions button {
      font-size: 12px;
      min-width: auto;
      padding: 0 8px;
      line-height: 28px;
    }
  `]
})
export class AlertCenterComponent implements OnInit, OnDestroy {
  stats: AlertStats = { firingCount: 0, todayNewCount: 0, weekTotalCount: 0 };
  alerts: AlertEvent[] = [];
  alertTotal = 0;
  aggregationGroups: AlertAggregationGroup[] = [];
  aggTotal = 0;
  page = 1;
  pageSize = 20;
  displayedColumns = ['severity', 'status', 'ruleName', 'dimension', 'value', 'time', 'actions'];
  aggDisplayedColumns = ['expand', 'severity', 'status', 'dimensionType', 'dimensionValue', 'time'];
  expandedId: number | null = null;
  expandedGroupId: number | null = null;
  groupEventsMap: Record<number, AlertEvent[]> = {};

  aggregateView = false;
  showSuppressed = false;

  filters: any = {
    status: '',
    severity: '',
    ruleId: '',
    dimensionValue: ''
  };

  alertRules: AlertRule[] = [];
  aggregationRules: AlertAggregationRule[] = [];
  suppressionRules: AlertSuppressionRule[] = [];
  private wsSub: Subscription | null = null;
  private aggWsSub: Subscription | null = null;

  JSON = JSON;

  get totalCount(): number {
    return this.aggregateView ? this.aggTotal : this.alertTotal;
  }

  constructor(
    private api: ApiService,
    private wsService: WebSocketService,
    private dialog: MatDialog
  ) {}

  ngOnInit(): void {
    this.loadStats();
    this.loadAlerts();
    this.loadRules();
    this.loadAggregationRules();
    this.loadSuppressionRules();

    this.wsSub = this.wsService.alerts$.subscribe(alert => {
      this.loadStats();
      if (!this.aggregateView && this.page === 1) {
        this.loadAlerts(1);
      }
    });

    this.aggWsSub = this.wsService.aggregations$.subscribe(agg => {
      this.loadStats();
      if (this.aggregateView && this.page === 1) {
        this.loadAlerts(1);
      }
    });
  }

  ngOnDestroy(): void {
    this.wsSub?.unsubscribe();
    this.aggWsSub?.unsubscribe();
  }

  loadStats(): void {
    this.api.getAlertStats().subscribe(stats => {
      this.stats = stats;
    });
  }

  loadAlerts(page: number = this.page): void {
    this.page = page;
    if (this.aggregateView) {
      this.loadAggregationGroups(page);
    } else {
      this.loadRawAlerts(page);
    }
  }

  loadRawAlerts(page: number = this.page): void {
    this.page = page;
    const params: any = {
      page: this.page,
      pageSize: this.pageSize
    };
    if (this.filters.status) params.status = this.filters.status;
    if (this.filters.severity) params.severity = this.filters.severity;
    if (this.filters.ruleId) params.ruleId = this.filters.ruleId;
    if (this.filters.dimensionValue) params.dimensionValue = this.filters.dimensionValue;
    params.includeSuppressed = this.showSuppressed;

    this.api.listAlertEvents(params).subscribe(res => {
      this.alerts = res.data;
      this.alertTotal = res.total;
    });
  }

  loadAggregationGroups(page: number = this.page): void {
    this.page = page;
    const params: any = {
      page: this.page,
      pageSize: this.pageSize
    };

    this.api.listAggregationGroups(params).subscribe(res => {
      this.aggregationGroups = res.data;
      this.aggTotal = res.total;
    });
  }

  loadRules(): void {
    this.api.listAlertRules({ page: 1, pageSize: 100 }).subscribe(res => {
      this.alertRules = res.data;
    });
  }

  loadAggregationRules(): void {
    this.api.listAggregationRules({ page: 1, pageSize: 100 }).subscribe(res => {
      this.aggregationRules = res.data;
    });
  }

  loadSuppressionRules(): void {
    this.api.listSuppressionRules({ page: 1, pageSize: 100 }).subscribe(res => {
      this.suppressionRules = res.data;
    });
  }

  refreshAll(): void {
    this.loadStats();
    this.loadAlerts();
    this.loadRules();
    this.loadAggregationRules();
    this.loadSuppressionRules();
  }

  onAggregateViewChange(): void {
    this.page = 1;
    this.expandedId = null;
    this.expandedGroupId = null;
    this.loadAlerts(1);
  }

  onShowSuppressedChange(): void {
    if (!this.aggregateView) {
      this.page = 1;
      this.loadAlerts(1);
    }
  }

  onPageChange(e: PageEvent): void {
    this.pageSize = e.pageSize;
    this.loadAlerts(e.pageIndex + 1);
  }

  toggleExpand(row: AlertEvent): void {
    this.expandedId = this.expandedId === row.id ? null : row.id;
  }

  isDetailRow = (index: number, item: AlertEvent) => {
    return this.expandedId === item.id;
  };

  toggleGroupExpand(row: AlertAggregationGroup, event?: Event): void {
    if (event) {
      event.stopPropagation();
    }
    this.expandedGroupId = this.expandedGroupId === row.id ? null : row.id;
  }

  isGroupDetailRow = (index: number, item: AlertAggregationGroup) => {
    return this.expandedGroupId === item.id;
  };

  loadGroupEvents(group: AlertAggregationGroup): void {
    if (this.groupEventsMap[group.id]) {
      return;
    }
    this.api.getAggregationGroupEvents(group.id, { page: 1, pageSize: 10 }).subscribe(res => {
      this.groupEventsMap[group.id] = res.data;
    });
  }

  acknowledge(alert: AlertEvent): void {
    this.api.acknowledgeAlert(alert.id, 'admin').subscribe(() => {
      this.loadStats();
      this.loadAlerts();
    });
  }

  toggleRule(rule: AlertRule): void {
    this.api.toggleAlertRule(rule.id, !rule.enabled).subscribe(() => {
      rule.enabled = !rule.enabled;
    });
  }

  toggleAggRule(rule: AlertAggregationRule): void {
    this.api.toggleAggregationRule(rule.id, !rule.enabled).subscribe(() => {
      rule.enabled = !rule.enabled;
    });
  }

  toggleSuppRule(rule: AlertSuppressionRule): void {
    this.api.toggleSuppressionRule(rule.id, !rule.enabled).subscribe(() => {
      rule.enabled = !rule.enabled;
    });
  }

  openRuleDialog(): void {
    alert('新增告警规则功能请通过 API 调用');
  }

  editRule(rule: AlertRule): void {
    alert('编辑告警规则功能请通过 API 调用');
  }

  deleteRule(rule: AlertRule): void {
    if (confirm(`确定要删除告警规则 "${rule.name}" 吗？`)) {
      this.api.deleteAlertRule(rule.id).subscribe(() => {
        this.loadRules();
      });
    }
  }

  openAggRuleDialog(): void {
    alert('新增聚合规则功能请通过 API 调用');
  }

  editAggRule(rule: AlertAggregationRule): void {
    alert('编辑聚合规则功能请通过 API 调用');
  }

  deleteAggRule(rule: AlertAggregationRule): void {
    if (confirm(`确定要删除聚合规则 "${rule.name}" 吗？`)) {
      this.api.deleteAggregationRule(rule.id).subscribe(() => {
        this.loadAggregationRules();
      });
    }
  }

  openSuppRuleDialog(): void {
    alert('新增抑制规则功能请通过 API 调用');
  }

  editSuppRule(rule: AlertSuppressionRule): void {
    alert('编辑抑制规则功能请通过 API 调用');
  }

  deleteSuppRule(rule: AlertSuppressionRule): void {
    if (confirm(`确定要删除抑制规则 "${rule.name}" 吗？`)) {
      this.api.deleteSuppressionRule(rule.id).subscribe(() => {
        this.loadSuppressionRules();
      });
    }
  }

  getSeverityLabel(sev: AlertSeverity): string {
    const labels: Record<AlertSeverity, string> = {
      critical: '严重',
      warning: '警告',
      info: '提示'
    };
    return labels[sev] || sev;
  }

  getStatusLabel(status: AlertStatus): string {
    const labels: Record<AlertStatus, string> = {
      firing: '触发中',
      acknowledged: '已确认',
      resolved: '已恢复',
      expired: '已过期'
    };
    return labels[status] || status;
  }

  getTriggerTypeLabel(type: AlertTriggerType): string {
    const labels: Record<AlertTriggerType, string> = {
      threshold: '阈值触发',
      rate: '速率触发',
      duration: '持续触发'
    };
    return labels[type] || type;
  }

  getScopeLabel(scope: AlertScopeType): string {
    const labels: Record<AlertScopeType, string> = {
      global: '全局',
      api: 'API',
      tenant: '租户'
    };
    return labels[scope] || scope;
  }

  getDimensionTypeLabel(type: AggregationDimensionType | string): string {
    const labels: Record<string, string> = {
      api_path: 'API路径',
      tenant_id: '租户',
      rule_id: '规则'
    };
    return labels[type] || type;
  }

  formatTime(ts: string): string {
    if (!ts) return '-';
    const d = new Date(ts);
    return `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')} ` +
      `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:` +
      `${d.getSeconds().toString().padStart(2, '0')}`;
  }

  formatValue(val: number): string {
    if (val >= 1000000) return (val / 1000000).toFixed(1) + 'M';
    if (val >= 1000) return (val / 1000).toFixed(1) + 'K';
    return val.toFixed(val % 1 === 0 ? 0 : 2);
  }
}
