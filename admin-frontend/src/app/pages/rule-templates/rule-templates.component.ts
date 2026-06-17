import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { MatTableModule } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDialogModule, MatDialog } from '@angular/material/dialog';
import { MatSortModule } from '@angular/material/sort';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { Inject } from '@angular/core';
import { ApiService } from '../../services/api.service';
import { RuleTemplate, AlgorithmType } from '../../models/models';

@Component({
  selector: 'app-rule-templates',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule,
    MatTableModule, MatButtonModule, MatInputModule, MatSelectModule,
    MatCheckboxModule, MatDialogModule, MatSortModule, MatIconModule, MatTooltipModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">规则模板管理</h1>
      <button mat-raised-button color="primary" (click)="openTemplateDialog()">
        <mat-icon>add</mat-icon>新建模板
      </button>
    </div>

    <div class="card">
      <div class="card-header">模板列表</div>
      <div class="card-content">
        <div class="toolbar">
          <mat-form-field appearance="outline" style="width: 300px;">
            <mat-label>搜索模板</mat-label>
            <input matInput [(ngModel)]="searchText" (input)="applyFilter()" placeholder="模板名称/描述">
            <mat-icon matSuffix>search</mat-icon>
          </mat-form-field>
        </div>

        <table mat-table [dataSource]="filteredTemplates" matSort class="full-width-table">
          <ng-container matColumnDef="name">
            <th mat-header-cell *matHeaderCellDef mat-sort-header>模板名称</th>
            <td mat-cell *matCellDef="let row">
              <strong>{{ row.name }}</strong>
              <div style="font-size:12px;color:#666;margin-top:2px;">{{ row.description }}</div>
            </td>
          </ng-container>
          <ng-container matColumnDef="algorithm">
            <th mat-header-cell *matHeaderCellDef>算法类型</th>
            <td mat-cell *matCellDef="let row">
              <span class="tag tag-blue">{{ getAlgorithmLabel(row.algorithm) }}</span>
            </td>
          </ng-container>
          <ng-container matColumnDef="limit">
            <th mat-header-cell *matHeaderCellDef>限流配置</th>
            <td mat-cell *matCellDef="let row">
              <div>{{ row.limit }} 次 / {{ row.windowSeconds }}s</div>
              <div style="font-size:12px;color:#666;margin-top:4px;">
                <span *ngIf="row.tokenBucketConfig">
                  令牌桶: {{ row.tokenBucketConfig.refillRate }}/s, 容量 {{ row.tokenBucketConfig.capacity }}
                </span>
              </div>
            </td>
          </ng-container>
          <ng-container matColumnDef="shaping">
            <th mat-header-cell *matHeaderCellDef>流量整形</th>
            <td mat-cell *matCellDef="let row">
              <span *ngIf="row.shapingConfig?.enabled" class="tag tag-green">队列模式</span>
              <span *ngIf="!row.shapingConfig?.enabled" class="tag tag-yellow">直接拒绝</span>
            </td>
          </ng-container>
          <ng-container matColumnDef="updatedAt">
            <th mat-header-cell *matHeaderCellDef mat-sort-header>更新时间</th>
            <td mat-cell *matCellDef="let row">{{ row.updatedAt.slice(0, 19) }}</td>
          </ng-container>
          <ng-container matColumnDef="actions">
            <th mat-header-cell *matHeaderCellDef>操作</th>
            <td mat-cell *matCellDef="let row">
              <button mat-icon-button matTooltip="编辑" (click)="openTemplateDialog(row)">
                <mat-icon>edit</mat-icon>
              </button>
              <button mat-icon-button matTooltip="删除" color="warn" (click)="deleteTemplate(row)">
                <mat-icon>delete</mat-icon>
              </button>
            </td>
          </ng-container>

          <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
          <tr mat-row *matRowDef="let row; columns: displayedColumns;"></tr>
        </table>

        <div *ngIf="!filteredTemplates.length" style="text-align:center;padding:48px;color:#999;">
          暂无模板数据
        </div>
      </div>
    </div>
  `,
  styles: [`
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 20px;
    }
    .page-title {
      font-size: 24px;
      font-weight: 600;
      margin: 0;
    }
    .card {
      background: #fff;
      border-radius: 8px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.08);
      margin-bottom: 20px;
    }
    .card-header {
      padding: 16px 20px;
      border-bottom: 1px solid #eee;
      font-weight: 600;
    }
    .card-content {
      padding: 20px;
    }
    .toolbar {
      display: flex;
      gap: 12px;
      margin-bottom: 16px;
      align-items: center;
    }
    .full-width-table {
      width: 100%;
    }
    .tag {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 4px;
      font-size: 12px;
      font-weight: 500;
    }
    .tag-blue {
      background: #e3f2fd;
      color: #1976d2;
    }
    .tag-green {
      background: #e8f5e9;
      color: #388e3c;
    }
    .tag-yellow {
      background: #fff8e1;
      color: #f57c00;
    }
  `]
})
export class RuleTemplatesComponent implements OnInit {
  templates: RuleTemplate[] = [];
  filteredTemplates: RuleTemplate[] = [];
  searchText = '';
  displayedColumns = ['name', 'algorithm', 'limit', 'shaping', 'updatedAt', 'actions'];

  algorithmOptions: { value: AlgorithmType; label: string }[] = [
    { value: 'token_bucket', label: '令牌桶' },
    { value: 'leaky_bucket', label: '漏桶' },
    { value: 'fixed_window', label: '固定窗口' },
    { value: 'sliding_window', label: '滑动窗口' },
    { value: 'sliding_log', label: '滑动日志' }
  ];

  constructor(
    private api: ApiService,
    private fb: FormBuilder,
    private dialog: MatDialog
  ) {}

  ngOnInit(): void {
    this.loadTemplates();
  }

  private loadTemplates(): void {
    this.api.listTemplates().subscribe(result => {
      this.templates = result.data;
      this.applyFilter();
    });
  }

  applyFilter(): void {
    this.filteredTemplates = this.templates.filter(t => {
      const matchSearch = !this.searchText ||
        t.name.toLowerCase().includes(this.searchText.toLowerCase()) ||
        t.description.toLowerCase().includes(this.searchText.toLowerCase());
      return matchSearch;
    });
  }

  getAlgorithmLabel(algo: AlgorithmType): string {
    const opt = this.algorithmOptions.find(a => a.value === algo);
    return opt?.label || algo;
  }

  deleteTemplate(template: RuleTemplate): void {
    if (confirm(`确认删除模板 "${template.name}" ?`)) {
      this.api.deleteTemplate(template.id).subscribe(() => this.loadTemplates());
    }
  }

  openTemplateDialog(template?: RuleTemplate): void {
    const dialogRef = this.dialog.open(TemplateFormDialogComponent, {
      width: '720px',
      data: { template, fb: this.fb, algoOpts: this.algorithmOptions }
    });
    dialogRef.afterClosed().subscribe(result => {
      if (result) {
        const obs = template ? this.api.updateTemplate(template.id, result) : this.api.createTemplate(result);
        obs.subscribe(() => this.loadTemplates());
      }
    });
  }
}

@Component({
  selector: 'app-template-form-dialog',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule,
    MatDialogModule, MatInputModule, MatSelectModule,
    MatButtonModule, MatCheckboxModule, MatIconModule
  ],
  template: `
    <h2 mat-dialog-title>{{ editing ? '编辑模板' : '新建模板' }}</h2>
    <form [formGroup]="form" mat-dialog-content style="display:flex;flex-direction:column;gap:16px;">
      <div class="form-row">
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>模板名称</mat-label>
          <input matInput formControlName="name" required>
        </mat-form-field>
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>限流算法</mat-label>
          <mat-select formControlName="algorithm" required>
            <mat-option *ngFor="let a of data.algoOpts" [value]="a.value">{{ a.label }}</mat-option>
          </mat-select>
        </mat-form-field>
      </div>

      <mat-form-field appearance="outline">
        <mat-label>模板描述</mat-label>
        <textarea matInput formControlName="description" rows="2" placeholder="描述此模板的适用场景"></textarea>
      </mat-form-field>

      <div class="form-row">
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>限流阈值</mat-label>
          <input matInput type="number" formControlName="limit" required>
        </mat-form-field>
        <mat-form-field appearance="outline" class="form-field">
          <mat-label>窗口大小(秒)</mat-label>
          <input matInput type="number" formControlName="windowSeconds" required>
        </mat-form-field>
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
  `,
  styles: [`
    .form-row {
      display: flex;
      gap: 16px;
    }
    .form-field {
      flex: 1;
      min-width: 0;
    }
    .card {
      background: #fafafa;
      border-radius: 8px;
      border: 1px solid #e0e0e0;
    }
    .card-header {
      padding: 12px 16px;
      border-bottom: 1px solid #e0e0e0;
      font-weight: 500;
    }
    .card-content {
      padding: 16px;
    }
  `]
})
export class TemplateFormDialogComponent {
  form: FormGroup;
  editing: boolean;

  constructor(
    public dialogRef: MatDialogRef<TemplateFormDialogComponent>,
    @Inject(MAT_DIALOG_DATA) public data: any
  ) {
    this.editing = !!data.template;
    const t = data.template || {};

    this.form = data.fb.group({
      name: [t.name || '', Validators.required],
      description: [t.description || ''],
      algorithm: [t.algorithm || 'token_bucket', Validators.required],
      limit: [t.limit || 1000, Validators.required],
      windowSeconds: [t.windowSeconds || 60, Validators.required],
      tokenBucketConfig: data.fb.group({
        refillRate: [t.tokenBucketConfig?.refillRate || 16],
        capacity: [t.tokenBucketConfig?.capacity || 1000],
        tokensPerReq: [t.tokenBucketConfig?.tokensPerReq || 1]
      }),
      shapingConfig: data.fb.group({
        enabled: [t.shapingConfig?.enabled || false],
        maxQueueDepth: [t.shapingConfig?.maxQueueDepth || 100],
        maxWaitMs: [t.shapingConfig?.maxWaitMs || 2000],
        priorityEnabled: [t.shapingConfig?.priorityEnabled || false]
      })
    });
  }

  onSubmit(): void {
    const val = { ...this.form.value };
    if (val.algorithm !== 'token_bucket') delete val.tokenBucketConfig;
    if (!val.shapingConfig?.enabled) {
      val.shapingConfig = { enabled: false, maxQueueDepth: 0, maxWaitMs: 0, priorityEnabled: false };
    }
    this.dialogRef.close(val);
  }
}
