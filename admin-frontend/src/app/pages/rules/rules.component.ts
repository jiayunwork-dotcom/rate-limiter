import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule, FormBuilder, FormGroup, Validators, FormArray } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDialogModule, MatDialog } from '@angular/material/dialog';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatPaginatorModule } from '@angular/material/paginator';
import { MatSortModule } from '@angular/material/sort';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService } from '../../services/api.service';
import { RuleConfig, AlgorithmType, Dimension, DimensionType } from '../../models/models';

@Component({
  selector: 'app-rules',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule,
    MatTableModule, MatButtonModule, MatInputModule, MatSelectModule,
    MatCheckboxModule, MatDialogModule, MatSlideToggleModule,
    MatPaginatorModule, MatSortModule, MatIconModule, MatTooltipModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">限流规则管理</h1>
      <button mat-raised-button color="primary" (click)="openRuleDialog()">
        <mat-icon>add</mat-icon>新建规则
      </button>
    </div>

    <div class="card">
      <div class="card-header">规则列表</div>
      <div class="card-content">
        <div class="toolbar">
          <mat-form-field appearance="outline" style="width: 300px;">
            <mat-label>搜索规则</mat-label>
            <input matInput [(ngModel)]="searchText" (input)="applyFilter()" placeholder="规则名称/API路径">
            <mat-icon matSuffix>search</mat-icon>
          </mat-form-field>
          <mat-form-field appearance="outline" style="width: 150px;">
            <mat-label>状态</mat-label>
            <mat-select [(ngModel)]="filterEnabled" (selectionChange)="applyFilter()">
              <mat-option [value]="null">全部</mat-option>
              <mat-option [value]="true">启用</mat-option>
              <mat-option [value]="false">禁用</mat-option>
            </mat-select>
          </mat-form-field>
          <span style="flex:1"></span>
          <button mat-stroked-button [disabled]="!selectedIds.length" (click)="bulkToggle(true)">
            批量启用({{ selectedIds.length }})
          </button>
          <button mat-stroked-button [disabled]="!selectedIds.length" (click)="bulkToggle(false)">
            批量禁用({{ selectedIds.length }})
          </button>
        </div>

        <table mat-table [dataSource]="filteredRules" matSort class="full-width-table">
          <ng-container matColumnDef="select">
            <th mat-header-cell *matHeaderCellDef>
              <mat-checkbox (change)="$event ? toggleAll() : null"
                [checked]="selectedIds.length === filteredRules.length && filteredRules.length > 0"
                [indeterminate]="selectedIds.length > 0 && selectedIds.length < filteredRules.length">
              </mat-checkbox>
            </th>
            <td mat-cell *matCellDef="let row">
              <mat-checkbox (change)="toggleSelection(row.id)" [checked]="selectedIds.includes(row.id)"></mat-checkbox>
            </td>
          </ng-container>
          <ng-container matColumnDef="name">
            <th mat-header-cell *matHeaderCellDef mat-sort-header>规则名称</th>
            <td mat-cell *matCellDef="let row">
              <strong>{{ row.name }}</strong>
              <div style="font-size:12px;color:#666;margin-top:2px;">v{{ row.version }} · {{ row.algorithm }}</div>
            </td>
          </ng-container>
          <ng-container matColumnDef="api">
            <th mat-header-cell *matHeaderCellDef>API/方法</th>
            <td mat-cell *matCellDef="let row">
              <span class="tag tag-blue">{{ row.method }}</span>
              <code>{{ row.apiPath }}</code>
            </td>
          </ng-container>
          <ng-container matColumnDef="limit">
            <th mat-header-cell *matHeaderCellDef>配额配置</th>
            <td mat-cell *matCellDef="let row">
              <div>{{ row.limit }} 次 / {{ row.windowSeconds }}s</div>
              <div style="font-size:12px;color:#666;margin-top:4px;">
                维度: {{ getDimSummary(row) }}
              </div>
            </td>
          </ng-container>
          <ng-container matColumnDef="shaping">
            <th mat-header-cell *matHeaderCellDef>流量整形</th>
            <td mat-cell *matCellDef="let row">
              <span *ngIf="row.shapingConfig?.enabled" class="tag tag-green">队列模式</span>
              <span *ngIf="!row.shapingConfig?.enabled" class="tag tag-yellow">直接拒绝</span>
              <span *ngIf="row.shapingConfig?.priorityEnabled" class="tag tag-blue" style="margin-left:4px;">优先级</span>
            </td>
          </ng-container>
          <ng-container matColumnDef="status">
            <th mat-header-cell *matHeaderCellDef>状态</th>
            <td mat-cell *matCellDef="let row">
              <mat-slide-toggle [checked]="row.enabled" (change)="toggleRule(row)">
                {{ row.enabled ? '启用' : '禁用' }}
              </mat-slide-toggle>
              <span *ngIf="row.grayRelease?.enabled" class="tag tag-yellow" style="margin-left:8px;">
                灰度 {{ row.grayRelease.trafficPercent }}%
              </span>
            </td>
          </ng-container>
          <ng-container matColumnDef="actions">
            <th mat-header-cell *matHeaderCellDef>操作</th>
            <td mat-cell *matCellDef="let row">
              <button mat-icon-button matTooltip="编辑" (click)="openRuleDialog(row)">
                <mat-icon>edit</mat-icon>
              </button>
              <button mat-icon-button matTooltip="版本" (click)="showVersions(row)">
                <mat-icon>history</mat-icon>
              </button>
              <button mat-icon-button matTooltip="删除" color="warn" (click)="deleteRule(row)">
                <mat-icon>delete</mat-icon>
              </button>
            </td>
          </ng-container>

          <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
          <tr mat-row *matRowDef="let row; columns: displayedColumns;"></tr>
        </table>

        <div *ngIf="!filteredRules.length" style="text-align:center;padding:48px;color:#999;">
          暂无规则数据
        </div>
      </div>
    </div>
  `
})
export class RulesComponent implements OnInit {
  rules: RuleConfig[] = [];
  filteredRules: RuleConfig[] = [];
  searchText = '';
  filterEnabled: boolean | null = null;
  selectedIds: string[] = [];
  displayedColumns = ['select', 'name', 'api', 'limit', 'shaping', 'status', 'actions'];

  algorithmOptions: { value: AlgorithmType; label: string }[] = [
    { value: 'token_bucket', label: '令牌桶' },
    { value: 'leaky_bucket', label: '漏桶' },
    { value: 'fixed_window', label: '固定窗口' },
    { value: 'sliding_window', label: '滑动窗口' },
    { value: 'sliding_log', label: '滑动日志' }
  ];

  dimensionTypes: { value: DimensionType; label: string }[] = [
    { value: 'api_path', label: 'API路径' },
    { value: 'method', label: '请求方法' },
    { value: 'user_id', label: '用户ID' },
    { value: 'tenant_id', label: '租户ID' },
    { value: 'client_ip', label: '来源IP' },
    { value: 'header', label: '自定义Header' }
  ];

  constructor(
    private api: ApiService,
    private fb: FormBuilder,
    private dialog: MatDialog
  ) {}

  ngOnInit(): void {
    this.loadRules();
  }

  private loadRules(): void {
    this.api.listRules().subscribe(rules => {
      this.rules = rules;
      this.applyFilter();
    });
  }

  applyFilter(): void {
    this.filteredRules = this.rules.filter(r => {
      const matchSearch = !this.searchText ||
        r.name.toLowerCase().includes(this.searchText.toLowerCase()) ||
        r.apiPath.toLowerCase().includes(this.searchText.toLowerCase());
      const matchEnabled = this.filterEnabled === null || r.enabled === this.filterEnabled;
      return matchSearch && matchEnabled;
    });
  }

  getDimSummary(rule: RuleConfig): string {
    return rule.dimensions?.dimensions?.map(d => {
      const dt = this.dimensionTypes.find(x => x.value === d.type);
      return dt?.label || d.type;
    }).join(rule.dimensions?.combineMode === 'AND' ? ' + ' : ' / ') || '无';
  }

  toggleRule(rule: RuleConfig): void {
    this.api.toggleRule(rule.id).subscribe(() => this.loadRules());
  }

  toggleSelection(id: string): void {
    const idx = this.selectedIds.indexOf(id);
    if (idx >= 0) this.selectedIds.splice(idx, 1);
    else this.selectedIds.push(id);
  }

  toggleAll(): void {
    if (this.selectedIds.length === this.filteredRules.length) {
      this.selectedIds = [];
    } else {
      this.selectedIds = [...this.filteredRules.map(r => r.id)];
    }
  }

  bulkToggle(enabled: boolean): void {
    this.api.bulkToggleRules(this.selectedIds, enabled).subscribe(() => {
      this.selectedIds = [];
      this.loadRules();
    });
  }

  deleteRule(rule: RuleConfig): void {
    if (confirm(`确认删除规则 "${rule.name}" ?`)) {
      this.api.deleteRule(rule.id).subscribe(() => this.loadRules());
    }
  }

  showVersions(rule: RuleConfig): void {
    this.api.getRuleVersions(rule.id).subscribe(versions => {
      const v = prompt(`版本历史 (最新v${rule.version}):\n` +
        versions.map(vr => `v${vr.version} - ${vr.createdAt.slice(0,19)} - ${vr.changedBy}`).join('\n') +
        `\n\n输入版本号回滚:`);
      if (v && parseInt(v)) {
        this.api.rollbackRule(rule.id, parseInt(v)).subscribe(() => this.loadRules());
      }
    });
  }

  openRuleDialog(rule?: RuleConfig): void {
    const dialogRef = this.dialog.open(RuleFormDialogComponent, {
      width: '720px',
      data: { rule, fb: this.fb, dimTypes: this.dimensionTypes, algoOpts: this.algorithmOptions }
    });
    dialogRef.afterClosed().subscribe(result => {
      if (result) {
        const obs = rule ? this.api.updateRule(rule.id, result) : this.api.createRule(result);
        obs.subscribe(() => this.loadRules());
      }
    });
  }
}

import { MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { Inject, Component } from '@angular/core';

@Component({
  selector: 'app-rule-form-dialog',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule,
    MatDialogModule, MatInputModule, MatSelectModule,
    MatButtonModule, MatCheckboxModule, MatIconModule
  ],
  template: `
    <h2 mat-dialog-title>{{ editing ? '编辑规则' : '新建规则' }}</h2>
    <form [formGroup]="form" mat-dialog-content style="display:flex;flex-direction:column;gap:16px;">
      <div class="form-row">
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>规则名称</mat-label>
          <input matInput formControlName="name" required>
        </mat-form-field>
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>限流算法</mat-label>
          <mat-select formControlName="algorithm" required>
            <mat-option *ngFor="let a of data.algoOpts" [value]="a.value">{{ a.label }}</mat-option>
          </mat-select>
        </mat-form-field>
      </div>

      <div class="form-row">
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>API路径</mat-label>
          <input matInput formControlName="apiPath" placeholder="* 或 /api/v1/users" required>
        </mat-form-field>
        <mat-form-field appearance="outline" style="width:150px;">
          <mat-label>方法</mat-label>
          <mat-select formControlName="method">
            <mat-option value="*">ALL</mat-option>
            <mat-option value="GET">GET</mat-option>
            <mat-option value="POST">POST</mat-option>
            <mat-option value="PUT">PUT</mat-option>
            <mat-option value="DELETE">DELETE</mat-option>
            <mat-option value="PATCH">PATCH</mat-option>
          </mat-select>
        </mat-form-field>
      </div>

      <div class="form-row">
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>限流阈值</mat-label>
          <input matInput type="number" formControlName="limit" required>
        </mat-form-field>
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>窗口大小(秒)</mat-label>
          <input matInput type="number" formControlName="windowSeconds" required>
        </mat-form-field>
        <mat-checkbox formControlName="enabled" style="margin-top:18px;">启用</mat-checkbox>
      </div>

      <div class="card" style="margin:0;">
        <div class="card-header">限流维度</div>
        <div class="card-content" formArrayName="dimensions">
          <mat-radio-group [(ngModel)]="combineMode" [ngModelOptions]="{standalone:true}" style="margin-bottom:12px;">
            <mat-radio-button value="OR" style="margin-right:16px;">独立计数</mat-radio-button>
            <mat-radio-button value="AND">AND组合</mat-radio-button>
          </mat-radio-group>
          <div *ngFor="let d of dimsArr.controls; let i=index" class="form-row">
            <mat-form-field appearance="outline" style="flex:1;min-width:180px;">
              <mat-label>维度类型</mat-label>
              <mat-select [formControlName]="i">
                <mat-option *ngFor="let dt of data.dimTypes" [value]="dt.value">{{ dt.label }}</mat-option>
              </mat-select>
            </mat-form-field>
            <button mat-icon-button color="warn" (click)="dimsArr.removeAt(i)"><mat-icon>delete</mat-icon></button>
          </div>
          <button mat-stroked-button type="button" (click)="addDimension()">+ 添加维度</button>
        </div>
      </div>

      <div formGroupName="tokenBucketConfig" *ngIf="form.value.algorithm === 'token_bucket'" class="card" style="margin:0;">
        <div class="card-header">令牌桶配置</div>
        <div class="card-content">
          <div class="form-row">
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>补充速率(令牌/秒)</mat-label>
              <input matInput type="number" formControlName="refillRate">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>桶容量</mat-label>
              <input matInput type="number" formControlName="capacity">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>每请求消耗</mat-label>
              <input matInput type="number" formControlName="tokensPerReq">
            </mat-form-field>
          </div>
        </div>
      </div>

      <div formGroupName="shapingConfig" class="card" style="margin:0;">
        <div class="card-header">
          <mat-checkbox formControlName="enabled">启用流量整形</mat-checkbox>
        </div>
        <div class="card-content" *ngIf="form.value.shapingConfig?.enabled">
          <div class="form-row">
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>最大队列深度</mat-label>
              <input matInput type="number" formControlName="maxQueueDepth">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>最大等待时间(ms)</mat-label>
              <input matInput type="number" formControlName="maxWaitMs">
            </mat-form-field>
            <mat-checkbox formControlName="priorityEnabled" style="margin-top:18px;">支持优先级</mat-checkbox>
          </div>
        </div>
      </div>
    </form>

    <div mat-dialog-actions style="justify-content:flex-end;">
      <button mat-button mat-dialog-close>取消</button>
      <button mat-raised-button color="primary" [disabled]="form.invalid" (click)="onSubmit()">保存</button>
    </div>
  `
})
export class RuleFormDialogComponent {
  form: FormGroup;
  combineMode: 'AND' | 'OR' = 'OR';
  editing: boolean;

  constructor(
    public dialogRef: MatDialogRef<RuleFormDialogComponent>,
    @Inject(MAT_DIALOG_DATA) public data: any
  ) {
    this.editing = !!data.rule;
    const r = data.rule || {};
    this.combineMode = r.dimensions?.combineMode || 'OR';
    const dims = r.dimensions?.dimensions?.map((d: Dimension) => d.type) || ['tenant_id'];

    this.form = data.fb.group({
      name: [r.name || '', Validators.required],
      apiPath: [r.apiPath || '/*', Validators.required],
      method: [r.method || '*'],
      algorithm: [r.algorithm || 'token_bucket', Validators.required],
      limit: [r.limit || 1000, Validators.required],
      windowSeconds: [r.windowSeconds || 60, Validators.required],
      enabled: [r.enabled !== false],
      dimensions: data.fb.array(dims),
      tokenBucketConfig: data.fb.group({
        refillRate: [r.tokenBucketConfig?.refillRate || 16],
        capacity: [r.tokenBucketConfig?.capacity || 1000],
        tokensPerReq: [r.tokenBucketConfig?.tokensPerReq || 1]
      }),
      shapingConfig: data.fb.group({
        enabled: [r.shapingConfig?.enabled || false],
        maxQueueDepth: [r.shapingConfig?.maxQueueDepth || 100],
        maxWaitMs: [r.shapingConfig?.maxWaitMs || 2000],
        priorityEnabled: [r.shapingConfig?.priorityEnabled || false]
      })
    });
  }

  get dimsArr(): FormArray {
    return this.form.get('dimensions') as FormArray;
  }

  addDimension(): void {
    this.dimsArr.push(this.data.fb.control('tenant_id'));
  }

  onSubmit(): void {
    const val = { ...this.form.value };
    val.dimensions = {
      combineMode: this.combineMode,
      dimensions: val.dimensions.map((t: DimensionType) => ({ type: t }))
    };
    if (val.algorithm !== 'token_bucket') delete val.tokenBucketConfig;
    this.dialogRef.close(val);
  }
}
