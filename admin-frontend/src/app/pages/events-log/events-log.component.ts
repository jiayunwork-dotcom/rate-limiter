import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { MatSortModule } from '@angular/material/sort';
import { MatIconModule } from '@angular/material/icon';
import { ApiService } from '../../services/api.service';
import { RateLimitEvent, QuotaLevel } from '../../models/models';

@Component({
  selector: 'app-events-log',
  standalone: true,
  imports: [
    CommonModule, FormsModule,
    MatTableModule, MatInputModule, MatSelectModule, MatButtonModule,
    MatDatepickerModule, MatNativeDateModule, MatPaginatorModule,
    MatSortModule, MatIconModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">限流告警日志</h1>
      <button mat-stroked-button (click)="loadEvents()">
        <mat-icon>refresh</mat-icon>刷新
      </button>
    </div>

    <div class="card">
      <div class="card-header">筛选条件</div>
      <div class="card-content">
        <div class="filter-bar">
          <mat-form-field appearance="outline" style="width:200px;">
            <mat-label>开始时间</mat-label>
            <input matInput [matDatepicker]="startPicker" [(ngModel)]="filters.startDate">
            <mat-datepicker-toggle matIconSuffix [for]="startPicker"></mat-datepicker-toggle>
            <mat-datepicker #startPicker></mat-datepicker>
          </mat-form-field>
          <mat-form-field appearance="outline" style="width:200px;">
            <mat-label>结束时间</mat-label>
            <input matInput [matDatepicker]="endPicker" [(ngModel)]="filters.endDate">
            <mat-datepicker-toggle matIconSuffix [for]="endPicker"></mat-datepicker-toggle>
            <mat-datepicker #endPicker></mat-datepicker>
          </mat-form-field>
          <mat-form-field appearance="outline" style="width:180px;">
            <mat-label>结果</mat-label>
            <mat-select [(ngModel)]="filters.allowed" (selectionChange)="loadEvents(1)">
              <mat-option [value]="null">全部</mat-option>
              <mat-option [value]="true">通过</mat-option>
              <mat-option [value]="false">拒绝</mat-option>
            </mat-select>
          </mat-form-field>
          <mat-form-field appearance="outline" style="width:180px;">
            <mat-label>触发层级</mat-label>
            <mat-select [(ngModel)]="filters.level" (selectionChange)="loadEvents(1)">
              <mat-option value="">全部</mat-option>
              <mat-option value="global">全局</mat-option>
              <mat-option value="tenant">租户</mat-option>
              <mat-option value="user">用户</mat-option>
              <mat-option value="api">API</mat-option>
            </mat-select>
          </mat-form-field>
          <span style="flex:1;"></span>
          <mat-form-field appearance="outline" style="width:180px;">
            <mat-label>租户ID</mat-label>
            <input matInput [(ngModel)]="filters.tenantId" (keyup.enter)="loadEvents(1)">
          </mat-form-field>
          <mat-form-field appearance="outline" style="width:180px;">
            <mat-label>API路径</mat-label>
            <input matInput [(ngModel)]="filters.apiPath" (keyup.enter)="loadEvents(1)">
          </mat-form-field>
          <mat-form-field appearance="outline" style="width:180px;">
            <mat-label>规则ID</mat-label>
            <input matInput [(ngModel)]="filters.ruleId" (keyup.enter)="loadEvents(1)">
          </mat-form-field>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        日志列表
        <span style="float:right;color:#666;font-size:13px;">
          共 {{ total }} 条记录
        </span>
      </div>
      <div class="card-content" style="padding:0;">
        <table mat-table [dataSource]="events" matSort class="full-width-table">
          <ng-container matColumnDef="timestamp">
            <th mat-header-cell *matHeaderCellDef mat-sort-header style="width:160px;">时间</th>
            <td mat-cell *matCellDef="let row">
              {{ formatTime(row.timestamp) }}
            </td>
          </ng-container>
          <ng-container matColumnDef="result">
            <th mat-header-cell *matHeaderCellDef style="width:80px;">结果</th>
            <td mat-cell *matCellDef="let row">
              <span *ngIf="row.allowed" class="tag tag-green">通过</span>
              <span *ngIf="!row.allowed" class="tag tag-red">拒绝</span>
            </td>
          </ng-container>
          <ng-container matColumnDef="api">
            <th mat-header-cell *matHeaderCellDef style="width:260px;">API</th>
            <td mat-cell *matCellDef="let row">
              <span class="tag tag-blue">{{ row.method }}</span>
              <code style="font-size:12px;">{{ row.apiPath }}</code>
            </td>
          </ng-container>
          <ng-container matColumnDef="rule">
            <th mat-header-cell *matHeaderCellDef>规则</th>
            <td mat-cell *matCellDef="let row">
              <div>{{ row.ruleName }}</div>
              <div style="font-size:11px;color:#999;">{{ row.ruleId }}</div>
            </td>
          </ng-container>
          <ng-container matColumnDef="dimensions">
            <th mat-header-cell *matHeaderCellDef style="width:240px;">维度</th>
            <td mat-cell *matCellDef="let row">
              <div *ngIf="row.tenantId" style="font-size:12px;">
                <span style="color:#666;">租户:</span> {{ row.tenantId }}
              </div>
              <div *ngIf="row.userId" style="font-size:12px;">
                <span style="color:#666;">用户:</span> {{ row.userId }}
              </div>
              <div style="font-size:12px;">
                <span style="color:#666;">IP:</span> {{ row.clientIp }}
              </div>
            </td>
          </ng-container>
          <ng-container matColumnDef="quota">
            <th mat-header-cell *matHeaderCellDef style="width:160px;">配额</th>
            <td mat-cell *matCellDef="let row">
              <div>剩余 {{ row.remaining }} / {{ row.limit }}</div>
              <span class="tag" [ngClass]="getLevelTagClass(row.triggeredLevel)">
                {{ getLevelLabel(row.triggeredLevel) }}
              </span>
              <span *ngIf="row.mode === 'local'" class="tag tag-yellow" style="margin-left:4px;">
                本地模式
              </span>
            </td>
          </ng-container>

          <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
          <tr mat-row *matRowDef="let row; columns: displayedColumns;"></tr>
        </table>

        <div *ngIf="!events.length" style="text-align:center;padding:48px;color:#999;">
          暂无日志数据
        </div>

        <div style="padding:16px 24px;border-top:1px solid #eee;">
          <mat-paginator
            [length]="total"
            [pageSize]="pageSize"
            [pageSizeOptions]="[20, 50, 100]"
            [pageIndex]="page - 1"
            (page)="onPageChange($event)">
          </mat-paginator>
        </div>
      </div>
    </div>
  `
})
export class EventsLogComponent implements OnInit {
  events: RateLimitEvent[] = [];
  total = 0;
  page = 1;
  pageSize = 20;
  displayedColumns = ['timestamp', 'result', 'api', 'rule', 'dimensions', 'quota'];

  filters: any = {
    startDate: null,
    endDate: null,
    allowed: null,
    level: '',
    tenantId: '',
    userId: '',
    apiPath: '',
    ruleId: ''
  };

  private levelLabels: Record<QuotaLevel, string> = {
    global: '全局', tenant: '租户', user: '用户', api: 'API'
  };

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    const now = new Date();
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000);
    this.filters.startDate = yesterday;
    this.filters.endDate = now;
    this.loadEvents();
  }

  loadEvents(page: number = this.page): void {
    this.page = page;
    const params: any = {
      page: this.page,
      pageSize: this.pageSize,
      tenantId: this.filters.tenantId || undefined,
      apiPath: this.filters.apiPath || undefined,
      ruleId: this.filters.ruleId || undefined
    };
    if (this.filters.allowed !== null) params.allowed = this.filters.allowed;
    if (this.filters.startDate) {
      params.startTime = new Date(this.filters.startDate).toISOString();
    }
    if (this.filters.endDate) {
      const d = new Date(this.filters.endDate);
      d.setHours(23, 59, 59, 999);
      params.endTime = d.toISOString();
    }
    this.api.listEvents(params).subscribe(res => {
      this.events = res.items;
      this.total = res.total;
    });
  }

  onPageChange(e: PageEvent): void {
    this.pageSize = e.pageSize;
    this.loadEvents(e.pageIndex + 1);
  }

  formatTime(ts: string): string {
    const d = new Date(ts);
    return `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')} ` +
      `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:` +
      `${d.getSeconds().toString().padStart(2, '0')}`;
  }

  getLevelLabel(level: QuotaLevel): string {
    return this.levelLabels[level] || level;
  }

  getLevelTagClass(level: QuotaLevel): string {
    switch (level) {
      case 'global': return 'tag-red';
      case 'tenant': return 'tag-yellow';
      case 'user': return 'tag-blue';
      case 'api': return 'tag-green';
      default: return 'tag-blue';
    }
  }
}
