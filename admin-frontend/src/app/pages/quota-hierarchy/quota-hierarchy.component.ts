import { Component, OnInit, Inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatTreeModule } from '@angular/material/tree';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog, MatDialogModule, MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatTooltipModule } from '@angular/material/tooltip';
import { NestedTreeControl } from '@angular/cdk/tree';
import { MatTreeNestedDataSource } from '@angular/material/tree';
import { ApiService } from '../../services/api.service';
import { QuotaTreeNode, QuotaLevel, QuotaConfig } from '../../models/models';

class FlatNode {
  id: string = '';
  name: string = '';
  level: QuotaLevel = 'global';
  limit: number = 0;
  currentUsage: number = 0;
  usagePercent: number = 0;
  overQuota: boolean = false;
  expandable: boolean = false;
  depth: number = 0;
}

@Component({
  selector: 'app-quota-hierarchy',
  standalone: true,
  imports: [
    CommonModule, FormsModule,
    MatTreeModule, MatIconModule, MatButtonModule,
    MatDialogModule, MatInputModule, MatSelectModule, MatProgressBarModule,
    MatTooltipModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">配额层级体系</h1>
      <span style="color:#666;font-size:13px;">全局 → 租户 → 用户 → API (上级配额约束下级)</span>
    </div>

    <div class="stat-grid">
      <div class="stat-card" *ngFor="let s of levelStats">
        <div class="stat-label">{{ s.label }}</div>
        <div class="stat-value">{{ s.count }}</div>
        <div class="stat-change" [ngClass]="s.over > 0 ? 'down' : 'up'">
          {{ s.over }} 个超额
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">配额层级树
        <button mat-button style="float:right;" (click)="refresh()">
          <mat-icon>refresh</mat-icon>刷新
        </button>
      </div>
      <div class="card-content">
        <mat-tree [dataSource]="dataSource" [treeControl]="treeControl" class="tree">
          <mat-tree-node *matTreeNodeDef="let node" matTreeNodeToggle matTreeNodePadding>
            <div class="tree-node-content" style="width:100%;"
              [style.background]="node.overQuota ? '#ffebee' : 'transparent'"
              [style.color]="node.overQuota ? '#c62828' : '#333'"
              [style.border-radius.px]="6"
              [style.padding.px]="8">
              <div style="flex:1;">
                <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px;">
                  <span class="tag tag-blue">{{ getLevelLabel(node.level) }}</span>
                  <strong>{{ node.name }}</strong>
                  <span *ngIf="node.overQuota" class="tag tag-red">已超额</span>
                </div>
                <div style="display:flex;align-items:center;gap:16px;font-size:13px;color:#666;">
                  <span>已用 {{ node.currentUsage | number }} / {{ node.limit | number }}</span>
                  <span>使用率 {{ node.usagePercent.toFixed(1) }}%</span>
                </div>
                <mat-progress-bar
                  mode="determinate"
                  [value]="Math.min(node.usagePercent, 100)"
                  [color]="node.usagePercent >= 90 ? 'warn' : node.usagePercent >= 70 ? 'accent' : 'primary'"
                  style="height:6px;margin-top:8px;border-radius:3px;">
                </mat-progress-bar>
              </div>
              <button mat-icon-button (click)="editQuota(node)" matTooltip="编辑配额">
                <mat-icon>edit</mat-icon>
              </button>
            </div>
          </mat-tree-node>

          <mat-nested-tree-node *matTreeNodeDef="let node; when: hasChild">
            <div class="tree-node-content" style="width:100%;"
              [style.background]="node.overQuota ? '#ffebee' : 'transparent'"
              [style.color]="node.overQuota ? '#c62828' : '#333'"
              [style.border-radius.px]="6"
              [style.padding.px]="8">
              <div class="mat-tree-node" matTreeNodePadding style="flex:1;">
                <button mat-icon-button matTreeNodeToggle [attr.aria-label]="'Toggle ' + node.name">
                  <mat-icon class="mat-icon-rtl-mirror">
                    {{ treeControl.isExpanded(node) ? 'expand_more' : 'chevron_right' }}
                  </mat-icon>
                </button>
                <div style="flex:1;">
                  <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px;">
                    <span class="tag tag-blue">{{ getLevelLabel(node.level) }}</span>
                    <strong>{{ node.name }}</strong>
                    <span *ngIf="node.overQuota" class="tag tag-red">已超额</span>
                  </div>
                  <div style="display:flex;align-items:center;gap:16px;font-size:13px;color:#666;">
                    <span>已用 {{ node.currentUsage | number }} / {{ node.limit | number }}</span>
                    <span>使用率 {{ node.usagePercent.toFixed(1) }}%</span>
                  </div>
                  <mat-progress-bar
                    mode="determinate"
                    [value]="Math.min(node.usagePercent, 100)"
                    [color]="node.usagePercent >= 90 ? 'warn' : node.usagePercent >= 70 ? 'accent' : 'primary'"
                    style="height:6px;margin-top:8px;border-radius:3px;">
                  </mat-progress-bar>
                </div>
                <button mat-icon-button (click)="editQuota(node)" matTooltip="编辑配额">
                  <mat-icon>edit</mat-icon>
                </button>
              </div>
            </div>
            <div [class.tree-children]="true" role="group" *matTreeNodeOutlet></div>
          </mat-nested-tree-node>
        </mat-tree>
      </div>
    </div>
  `
})
export class QuotaHierarchyComponent implements OnInit {
  Math = Math;
  treeControl = new NestedTreeControl<QuotaTreeNode>(node => node.children);
  dataSource = new MatTreeNestedDataSource<QuotaTreeNode>();
  levelStats: Array<{ label: string; count: number; over: number }> = [
    { label: '全局配额', count: 0, over: 0 },
    { label: '租户配额', count: 0, over: 0 },
    { label: '用户配额', count: 0, over: 0 },
    { label: 'API配额', count: 0, over: 0 }
  ];

  private levelLabels: Record<QuotaLevel, string> = {
    global: '全局', tenant: '租户', user: '用户', api: 'API'
  };

  constructor(private api: ApiService, private dialog: MatDialog) {}

  ngOnInit(): void {
    this.refresh();
  }

  refresh(): void {
    this.api.getQuotaTree().subscribe(data => {
      this.dataSource.data = data;
      this.calcStats(data);
    });
  }

  hasChild = (_: number, node: QuotaTreeNode) => !!node.children && node.children.length > 0;

  getLevelLabel(level: QuotaLevel): string {
    return this.levelLabels[level] || level;
  }

  private calcStats(nodes: QuotaTreeNode[]): void {
    const walk = (ns: QuotaTreeNode[]) => {
      for (const n of ns) {
        const idx = ['global', 'tenant', 'user', 'api'].indexOf(n.level);
        if (idx >= 0) {
          this.levelStats[idx].count++;
          if (n.overQuota) this.levelStats[idx].over++;
        }
        if (n.children) walk(n.children);
      }
    };
    this.levelStats = this.levelStats.map(s => ({ ...s, count: 0, over: 0 }));
    walk(nodes);
  }

  editQuota(node: QuotaTreeNode): void {
    const dialogRef = this.dialog.open(QuotaEditDialogComponent, {
      width: '480px',
      data: { node, levelLabels: this.levelLabels }
    });
    dialogRef.afterClosed().subscribe(result => {
      if (result) {
        const payload: Partial<QuotaConfig> = {
          level: node.level,
          tenantId: node.level === 'tenant' || node.level === 'user' || node.level === 'api'
            ? this.extractTenantId(node) : undefined,
          userId: node.level === 'user' || node.level === 'api' ? node.id.split('/')[1] : undefined,
          apiPath: node.level === 'api' ? node.name : undefined,
          limit: result.limit,
          windowSeconds: result.windowSeconds,
          inherited: result.inherited
        };
        this.api.upsertQuota(payload).subscribe(() => this.refresh());
      }
    });
  }

  private extractTenantId(node: QuotaTreeNode): string {
    const parts = node.id.split('/');
    return parts[0];
  }
}


@Component({
  selector: 'app-quota-edit-dialog',
  standalone: true,
  imports: [
    CommonModule, FormsModule, MatDialogModule,
    MatInputModule, MatSelectModule, MatButtonModule, MatCheckboxModule
  ],
  template: `
    <h2 mat-dialog-title>编辑 {{ data.levelLabels[data.node.level] }} 配额</h2>
    <div mat-dialog-content style="display:flex;flex-direction:column;gap:16px;">
      <div style="padding:16px;background:#f5f5f5;border-radius:8px;">
        <div><strong>名称:</strong> {{ data.node.name }}</div>
        <div><strong>当前使用:</strong> {{ data.node.currentUsage | number }} / {{ data.node.limit | number }}
          ({{ data.node.usagePercent.toFixed(1) }}%)
        </div>
      </div>
      <mat-checkbox [(ngModel)]="form.inherited">
        继承上级默认值
      </mat-checkbox>
      <div *ngIf="!form.inherited">
        <div class="form-row">
          <mat-form-field appearance="outline" class="form-field">
            <mat-label>配额限额</mat-label>
            <input matInput type="number" [(ngModel)]="form.limit" [disabled]="form.inherited">
          </mat-form-field>
          <mat-form-field appearance="outline" class="form-field">
            <mat-label>窗口大小(秒)</mat-label>
            <input matInput type="number" [(ngModel)]="form.windowSeconds" [disabled]="form.inherited">
          </mat-form-field>
        </div>
      </div>
    </div>
    <div mat-dialog-actions style="justify-content:flex-end;">
      <button mat-button mat-dialog-close>取消</button>
      <button mat-raised-button color="primary" (click)="onSubmit()">保存</button>
    </div>
  `
})
export class QuotaEditDialogComponent {
  form: any = {
    inherited: false,
    limit: 0,
    windowSeconds: 60
  };

  constructor(
    @Inject(MAT_DIALOG_DATA) public data: any,
    private dialogRef: MatDialogRef<QuotaEditDialogComponent>
  ) {
    this.form.limit = data.node.limit;
    this.form.windowSeconds = data.node.level === 'global' ? 60 :
      data.node.level === 'tenant' ? 60 :
      data.node.level === 'user' ? 60 : 60;
  }

  onSubmit(): void {
    this.dialogRef.close(this.form);
  }
}
