import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatChipsModule } from '@angular/material/chips';
import { MatDialogModule, MatDialog } from '@angular/material/dialog';
import {
  AuditLog, AuditStats, AuditOperationType, AuditResourceType,
  TimelineNode, PaginatedAuditLogResult, DiffField
} from '../../models/models';
import { ApiService } from '../../services/api.service';

@Component({
  selector: 'app-audit-log',
  standalone: true,
  imports: [
    CommonModule, FormsModule,
    MatTableModule, MatInputModule, MatSelectModule, MatButtonModule,
    MatPaginatorModule, MatIconModule, MatCardModule, MatFormFieldModule,
    MatDatepickerModule, MatNativeDateModule, MatSidenavModule,
    MatListModule, MatChipsModule, MatDialogModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">审计日志</h1>
      <div class="header-actions">
        <button mat-stroked-button (click)="loadData()">
          <mat-icon>refresh</mat-icon>刷新
        </button>
      </div>
    </div>

    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">今日操作总数</div>
        <div class="stat-value" style="color: #2196f3;">{{ stats.todayTotalCount || 0 }}</div>
        <div class="stat-change">今日所有用户操作</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">本周最活跃</div>
        <div class="stat-value" style="color: #ff9800;">{{ stats.weekTopOperator || '-' }}</div>
        <div class="stat-change">操作 {{ stats.weekTopCount || 0 }} 次</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">最近一次操作</div>
        <div class="stat-value small" style="color: #4caf50;">
          {{ stats.lastOperationType ? getOperationTypeLabel(stats.lastOperationType) : '-' }}
          {{ stats.lastResourceType ? '/' + getResourceTypeLabel(stats.lastResourceType) : '' }}
        </div>
        <div class="stat-change">{{ formatLastOperationTime() }}</div>
      </div>
    </div>

    <div class="card">
      <div class="filter-bar">
        <mat-form-field appearance="outline" class="filter-field">
          <mat-label>操作人</mat-label>
          <mat-select [(ngModel)]="filter.operator" (selectionChange)="loadData()">
            <mat-option value="">全部</mat-option>
            <mat-option *ngFor="let op of operators" [value]="op">{{ op }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline" class="filter-field">
          <mat-label>资源类型</mat-label>
          <mat-select [(ngModel)]="filter.resourceType" (selectionChange)="loadData()">
            <mat-option value="">全部</mat-option>
            <mat-option value="rule">规则</mat-option>
            <mat-option value="quota">配额</mat-option>
            <mat-option value="alert_rule">告警规则</mat-option>
            <mat-option value="aggregation_rule">聚合规则</mat-option>
            <mat-option value="suppression_rule">抑制规则</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline" class="filter-field">
          <mat-label>操作类型</mat-label>
          <mat-select [(ngModel)]="filter.operationType" (selectionChange)="loadData()">
            <mat-option value="">全部</mat-option>
            <mat-option value="create">创建</mat-option>
            <mat-option value="update">更新</mat-option>
            <mat-option value="delete">删除</mat-option>
            <mat-option value="toggle">开关</mat-option>
            <mat-option value="rollback">回滚</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline" class="filter-field date-field">
          <mat-label>开始时间</mat-label>
          <input matInput [matDatepicker]="startPicker" [(ngModel)]="filter.startDate" (dateChange)="loadData()">
          <mat-datepicker-toggle matSuffix [for]="startPicker"></mat-datepicker-toggle>
          <mat-datepicker #startPicker></mat-datepicker>
        </mat-form-field>
        <mat-form-field appearance="outline" class="filter-field date-field">
          <mat-label>结束时间</mat-label>
          <input matInput [matDatepicker]="endPicker" [(ngModel)]="filter.endDate" (dateChange)="loadData()">
          <mat-datepicker-toggle matSuffix [for]="endPicker"></mat-datepicker-toggle>
          <mat-datepicker #endPicker></mat-datepicker>
        </mat-form-field>
      </div>

      <table mat-table [dataSource]="auditLogs" class="audit-table" multiTemplateDataRows>
        <ng-container matColumnDef="createdAt">
          <th mat-header-cell *matHeaderCellDef>时间</th>
          <td mat-cell *matCellDef="let row">
            {{ row.createdAt | date:'yyyy-MM-dd HH:mm:ss' }}
          </td>
        </ng-container>
        <ng-container matColumnDef="operator">
          <th mat-header-cell *matHeaderCellDef>操作人</th>
          <td mat-cell *matCellDef="let row">{{ row.operator }}</td>
        </ng-container>
        <ng-container matColumnDef="operationType">
          <th mat-header-cell *matHeaderCellDef>操作类型</th>
          <td mat-cell *matCellDef="let row">
            <span class="op-tag" [ngClass]="'op-' + row.operationType">
              {{ getOperationTypeLabel(row.operationType) }}
            </span>
          </td>
        </ng-container>
        <ng-container matColumnDef="resourceType">
          <th mat-header-cell *matHeaderCellDef>资源类型</th>
          <td mat-cell *matCellDef="let row">{{ getResourceTypeLabel(row.resourceType) }}</td>
        </ng-container>
        <ng-container matColumnDef="resourceId">
          <th mat-header-cell *matHeaderCellDef>资源ID</th>
          <td mat-cell *matCellDef="let row" class="resource-id-cell">
            <span class="resource-id">{{ row.resourceId }}</span>
          </td>
        </ng-container>
        <ng-container matColumnDef="diffSummary">
          <th mat-header-cell *matHeaderCellDef>变更摘要</th>
          <td mat-cell *matCellDef="let row">
            <button mat-button class="diff-toggle-btn" (click)="$event.stopPropagation(); toggleRowExpand(row)">
              <mat-icon>{{ expandedRows.has(row.id) ? 'expand_less' : 'expand_more' }}</mat-icon>
              <span>{{ getDiffSummaryText(row.diffSummary) }}</span>
            </button>
          </td>
        </ng-container>

        <ng-container matColumnDef="expandedDetail">
          <td mat-cell *matCellDef="let row" [attr.colspan]="displayedColumns.length">
            <div class="diff-container" *ngIf="expandedRows.has(row.id)">
              <div class="diff-header">
                <span class="diff-title">变更详情</span>
              </div>
              <div class="diff-table">
                <div class="diff-row" *ngFor="let field of getDiffFields(row.diffSummary)">
                  <div class="diff-field-name">{{ field.name }}</div>
                  <div class="diff-old-value">{{ formatValue(field.oldValue) }}</div>
                  <div class="diff-arrow">→</div>
                  <div class="diff-new-value">{{ formatValue(field.newValue) }}</div>
                </div>
              </div>
            </div>
          </td>
        </ng-container>

        <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
        <tr mat-row *matRowDef="let row; columns: displayedColumns;"
            class="audit-row"
            (click)="openTimeline(row)"></tr>
        <tr mat-row *matRowDef="let row; columns: ['expandedDetail']" class="expanded-row"></tr>
      </table>

      <mat-paginator [length]="totalCount"
                     [pageSize]="pageSize"
                     [pageIndex]="pageIndex"
                     [pageSizeOptions]="[20, 50, 100]"
                     (page)="onPageChange($event)"
                     showFirstLastButtons>
      </mat-paginator>
    </div>

    <mat-sidenav-container class="timeline-container">
      <mat-sidenav #timelineDrawer mode="over" position="end" class="timeline-drawer">
        <div class="timeline-header">
          <h3>资源操作时间线</h3>
          <button mat-icon-button (click)="timelineDrawer.close()">
            <mat-icon>close</mat-icon>
          </button>
        </div>
        <div class="timeline-subtitle">
          <span class="timeline-resource-type">{{ selectedResourceType }}</span>
          <span class="timeline-resource-id">{{ selectedResourceId }}</span>
        </div>
        <div class="timeline-list">
          <div class="timeline-item"
               *ngFor="let node of timelineNodes; let first = first; let last = last"
               [ngClass]="{ 'active': node.id === selectedAuditId, 'first': first, 'last': last }">
            <div class="timeline-left">
              <div class="timeline-dot"></div>
              <div class="timeline-line" *ngIf="!last"></div>
            </div>
            <div class="timeline-content">
              <div class="timeline-time">{{ node.createdAt | date:'yyyy-MM-dd HH:mm:ss' }}</div>
              <div class="timeline-meta">
                <span class="op-tag" [ngClass]="'op-' + node.operationType">
                  {{ getOperationTypeLabel(node.operationType) }}
                </span>
                <span class="timeline-operator">{{ node.operator }}</span>
              </div>
              <div class="timeline-diff">
                <span *ngFor="let field of getDiffFields(node.diffSummary); let i = index" class="diff-chip">
                  {{ field.name }}
                </span>
              </div>
              <div class="timeline-actions" *ngIf="node.canRollback">
                <button mat-button color="warn" class="rollback-btn" (click)="confirmRollback(node)">
                  <mat-icon>undo</mat-icon>回滚
                </button>
              </div>
            </div>
          </div>
        </div>
      </mat-sidenav>
    </mat-sidenav-container>
  `,
  styles: [`
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 20px;
    }
    .page-title {
      font-size: 22px;
      font-weight: 600;
      margin: 0;
      color: #212121;
    }
    .header-actions button {
      margin-left: 8px;
    }
    .stat-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 16px;
      margin-bottom: 20px;
    }
    .stat-card {
      background: #fff;
      border-radius: 8px;
      padding: 20px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .stat-label {
      font-size: 13px;
      color: #666;
      margin-bottom: 8px;
    }
    .stat-value {
      font-size: 28px;
      font-weight: 600;
      margin-bottom: 4px;
    }
    .stat-value.small {
      font-size: 18px;
    }
    .stat-change {
      font-size: 12px;
      color: #999;
    }
    .card {
      background: #fff;
      border-radius: 8px;
      padding: 20px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .filter-bar {
      display: flex;
      gap: 12px;
      margin-bottom: 16px;
      flex-wrap: wrap;
    }
    .filter-field {
      min-width: 140px;
    }
    .date-field {
      min-width: 160px;
    }
    .audit-table {
      width: 100%;
    }
    .audit-row {
      cursor: pointer;
    }
    .audit-row:hover {
      background: #f5f5f5;
    }
    .resource-id-cell {
      max-width: 200px;
    }
    .resource-id {
      font-family: monospace;
      font-size: 12px;
      color: #666;
    }
    .op-tag {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 500;
    }
    .op-create {
      background: #e8f5e9;
      color: #2e7d32;
    }
    .op-update {
      background: #e3f2fd;
      color: #1565c0;
    }
    .op-delete {
      background: #ffebee;
      color: #c62828;
    }
    .op-toggle {
      background: #fff3e0;
      color: #e65100;
    }
    .op-rollback {
      background: #f3e5f5;
      color: #6a1b9a;
    }
    .diff-toggle-btn {
      text-align: left;
      padding: 0;
      margin: 0;
      font-size: 13px;
      color: #1976d2;
      min-width: auto;
    }
    .expanded-row {
      background: #fafafa;
    }
    .expanded-row td {
      padding: 0;
      border: none;
    }
    .diff-container {
      padding: 16px 24px;
    }
    .diff-header {
      margin-bottom: 12px;
    }
    .diff-title {
      font-weight: 600;
      font-size: 14px;
      color: #333;
    }
    .diff-table {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .diff-row {
      display: grid;
      grid-template-columns: 180px 1fr 30px 1fr;
      gap: 12px;
      align-items: center;
      padding: 8px 12px;
      background: #fff;
      border-radius: 4px;
      border: 1px solid #e0e0e0;
    }
    .diff-field-name {
      font-weight: 500;
      color: #333;
      font-size: 13px;
    }
    .diff-old-value {
      background: #ffebee;
      padding: 4px 8px;
      border-radius: 4px;
      font-size: 12px;
      color: #c62828;
      font-family: monospace;
      word-break: break-all;
    }
    .diff-arrow {
      text-align: center;
      color: #999;
    }
    .diff-new-value {
      background: #e8f5e9;
      padding: 4px 8px;
      border-radius: 4px;
      font-size: 12px;
      color: #2e7d32;
      font-family: monospace;
      word-break: break-all;
    }
    .timeline-container {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      pointer-events: none;
      z-index: 100;
    }
    .timeline-drawer {
      width: 480px;
      pointer-events: auto;
      box-shadow: -2px 0 8px rgba(0,0,0,0.15);
    }
    .timeline-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 20px;
      border-bottom: 1px solid #e0e0e0;
    }
    .timeline-header h3 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
    }
    .timeline-subtitle {
      padding: 12px 20px;
      background: #f5f5f5;
      border-bottom: 1px solid #e0e0e0;
    }
    .timeline-resource-type {
      font-size: 13px;
      color: #666;
      margin-right: 8px;
    }
    .timeline-resource-id {
      font-family: monospace;
      font-size: 12px;
      color: #333;
    }
    .timeline-list {
      padding: 16px 20px;
      max-height: calc(100vh - 140px);
      overflow-y: auto;
    }
    .timeline-item {
      display: flex;
      gap: 12px;
      position: relative;
    }
    .timeline-item.active .timeline-dot {
      background: #1976d2;
      box-shadow: 0 0 0 4px rgba(25,118,210,0.2);
    }
    .timeline-left {
      display: flex;
      flex-direction: column;
      align-items: center;
      width: 12px;
    }
    .timeline-dot {
      width: 12px;
      height: 12px;
      border-radius: 50%;
      background: #bdbdbd;
      flex-shrink: 0;
    }
    .timeline-line {
      width: 2px;
      flex: 1;
      background: #e0e0e0;
      margin-top: 4px;
    }
    .timeline-content {
      flex: 1;
      padding-bottom: 20px;
    }
    .timeline-time {
      font-size: 12px;
      color: #666;
      margin-bottom: 4px;
    }
    .timeline-meta {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;
    }
    .timeline-operator {
      font-size: 13px;
      color: #333;
      font-weight: 500;
    }
    .timeline-diff {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
      margin-bottom: 8px;
    }
    .diff-chip {
      display: inline-block;
      padding: 2px 8px;
      background: #f0f0f0;
      border-radius: 4px;
      font-size: 11px;
      color: #666;
    }
    .timeline-actions {
      margin-top: 8px;
    }
    .rollback-btn {
      font-size: 12px;
    }
  `]
})
export class AuditLogComponent implements OnInit {
  displayedColumns: string[] = ['createdAt', 'operator', 'operationType', 'resourceType', 'resourceId', 'diffSummary'];
  auditLogs: AuditLog[] = [];
  totalCount = 0;
  pageIndex = 0;
  pageSize = 20;
  operators: string[] = [];
  stats: AuditStats = {} as AuditStats;
  expandedRows: Set<number> = new Set();

  isExpansionDetailRow = (i: number, row: Object) => row.hasOwnProperty('detailRow');

  filter = {
    operator: '',
    resourceType: '',
    operationType: '',
    startDate: null as Date | null,
    endDate: null as Date | null,
    resourceId: ''
  };

  @ViewChild('timelineDrawer') timelineDrawer: any;
  timelineNodes: TimelineNode[] = [];
  selectedResourceId = '';
  selectedResourceType = '';
  selectedAuditId: number | null = null;

  constructor(
    private api: ApiService,
    private dialog: MatDialog
  ) {}

  ngOnInit(): void {
    this.loadStats();
    this.loadOperators();
    this.loadData();
  }

  loadStats(): void {
    this.api.getAuditStats().subscribe(stats => {
      this.stats = stats;
    }, () => {});
  }

  loadOperators(): void {
    this.api.listAuditOperators().subscribe(ops => {
      this.operators = ops;
    }, () => {});
  }

  loadData(): void {
    const params: any = {
      page: this.pageIndex + 1,
      pageSize: this.pageSize
    };
    if (this.filter.operator) params.operator = this.filter.operator;
    if (this.filter.resourceType) params.resourceType = this.filter.resourceType;
    if (this.filter.operationType) params.operationType = this.filter.operationType;
    if (this.filter.resourceId) params.resourceId = this.filter.resourceId;
    if (this.filter.startDate) {
      const d = this.filter.startDate;
      params.startTime = `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}T00:00:00`;
    }
    if (this.filter.endDate) {
      const d = this.filter.endDate;
      params.endTime = `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}T23:59:59`;
    }

    this.api.listAuditLogs(params).subscribe(result => {
      this.auditLogs = result.data;
      this.totalCount = result.total;
    }, () => {});
  }

  onPageChange(event: PageEvent): void {
    this.pageIndex = event.pageIndex;
    this.pageSize = event.pageSize;
    this.loadData();
  }

  toggleRowExpand(row: AuditLog): void {
    if (this.expandedRows.has(row.id)) {
      this.expandedRows.delete(row.id);
    } else {
      this.expandedRows.add(row.id);
    }
  }

  getDiffFields(diffSummary: Record<string, DiffField>): Array<{ name: string; oldValue: any; newValue: any }> {
    if (!diffSummary) return [];
    return Object.entries(diffSummary).map(([name, field]) => ({
      name,
      oldValue: field.oldValue,
      newValue: field.newValue
    }));
  }

  getDiffSummaryText(diffSummary: Record<string, DiffField>): string {
    if (!diffSummary) return '';
    const keys = Object.keys(diffSummary);
    if (keys.length === 0) return '无变更';
    if (keys.length <= 3) return keys.join(', ');
    return `${keys.slice(0, 3).join(', ')} 等 ${keys.length} 个字段`;
  }

  formatValue(value: any): string {
    if (value === null || value === undefined) return '-';
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  }

  formatLastOperationTime(): string {
    if (!this.stats || !this.stats.lastOperationTime) {
      return '暂无';
    }
    const t = new Date(this.stats.lastOperationTime);
    if (isNaN(t.getTime()) || t.getFullYear() < 1970) {
      return '暂无';
    }
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${t.getFullYear()}-${pad(t.getMonth()+1)}-${pad(t.getDate())} ${pad(t.getHours())}:${pad(t.getMinutes())}:${pad(t.getSeconds())}`;
  }

  getOperationTypeLabel(type: AuditOperationType): string {
    const map: Record<AuditOperationType, string> = {
      create: '创建',
      update: '更新',
      delete: '删除',
      toggle: '开关',
      rollback: '回滚'
    };
    return map[type] || type;
  }

  getResourceTypeLabel(type: AuditResourceType): string {
    const map: Record<AuditResourceType, string> = {
      rule: '规则',
      quota: '配额',
      alert_rule: '告警规则',
      aggregation_rule: '聚合规则',
      suppression_rule: '抑制规则'
    };
    return map[type] || type;
  }

  openTimeline(row: AuditLog): void {
    this.selectedResourceId = row.resourceId;
    this.selectedResourceType = this.getResourceTypeLabel(row.resourceType);
    this.selectedAuditId = row.id;
    this.api.getAuditTimeline(row.resourceId).subscribe(nodes => {
      this.timelineNodes = nodes;
      this.timelineDrawer.open();
    }, () => {});
  }

  confirmRollback(node: TimelineNode): void {
    if (confirm(`确定要回滚此操作吗？\n操作类型: ${this.getOperationTypeLabel(node.operationType)}\n操作人: ${node.operator}`)) {
      this.api.rollbackAuditOperation(node.id).subscribe(() => {
        alert('回滚成功');
        this.loadData();
        this.loadStats();
        if (this.selectedResourceId) {
          this.api.getAuditTimeline(this.selectedResourceId).subscribe(nodes => {
            this.timelineNodes = nodes;
          }, () => {});
        }
      }, err => {
        alert('回滚失败: ' + (err.error?.message || err.message));
      });
    }
  }
}
