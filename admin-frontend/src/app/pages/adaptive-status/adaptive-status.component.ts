import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule, FormBuilder, FormGroup } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatInputModule } from '@angular/material/input';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatIconModule } from '@angular/material/icon';
import { Chart, registerables } from 'chart.js';
import { NgChartsModule } from 'ng2-charts';
import { interval, Subject, takeUntil } from 'rxjs';
import { ApiService } from '../../services/api.service';
import { AdaptiveStatus, AdaptiveConfigUpdate, PIDState } from '../../models/models';

Chart.register(...registerables);

@Component({
  selector: 'app-adaptive-status',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule,
    MatCardModule, MatButtonModule, MatInputModule,
    MatCheckboxModule, MatSlideToggleModule, MatIconModule, NgChartsModule
  ],
  template: `
    <div class="page-header">
      <h1 class="page-title">自适应限流状态</h1>
      <div>
        <button mat-stroked-button style="margin-right:8px;" (click)="refresh()">
          <mat-icon>refresh</mat-icon>刷新
        </button>
      </div>
    </div>

    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">自适应限流</div>
        <div class="stat-value" [style.color]="status?.enabled ? '#2e7d32' : '#999'">
          {{ status?.enabled ? '已启用' : '已禁用' }}
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">当前调节系数</div>
        <div class="stat-value" [style.color]="coeffColor">
          {{ (status?.currentCoefficient || 0) * 100 | number:'1.0-0' }}%
          <span *ngIf="status?.manualOverride" class="tag tag-yellow" style="font-size:12px;">手动覆盖</span>
        </div>
        <div class="stat-change" *ngIf="status">
          原始值: {{ (status.originalCoefficient * 100).toFixed(0) }}%
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">P99 延迟</div>
        <div class="stat-value" [style.color]="(status?.p99LatencyMs || 0) > (status?.targetP99LatencyMs || 200) ? '#c62828' : '#2e7d32'">
          {{ status?.p99LatencyMs || 0 }}ms
        </div>
        <div class="stat-change">
          目标: {{ status?.targetP99LatencyMs || 200 }}ms
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">错误率</div>
        <div class="stat-value" [style.color]="(status?.errorRate || 0) > (status?.errorRateThreshold || 0.05) ? '#c62828' : '#2e7d32'">
          {{ ((status?.errorRate || 0) * 100).toFixed(2) }}%
        </div>
        <div class="stat-change">
          阈值: {{ ((status?.errorRateThreshold || 0.05) * 100).toFixed(1) }}%
        </div>
      </div>
    </div>

    <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 24px;">
      <div class="card">
        <div class="card-header">P99 延迟趋势</div>
        <div class="card-content">
          <div class="chart-container" style="height:220px">
            <canvas baseChart [type]="'line'" [data]="latencyChart" [options]="chartOptions"></canvas>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">PID 控制器</div>
        <div class="card-content">
          <div class="form-row" *ngIf="pid">
            <div class="form-field">
              <div class="stat-label">Kp (比例)</div>
              <div class="stat-value" style="font-size:18px;">{{ pid.kp }}</div>
            </div>
            <div class="form-field">
              <div class="stat-label">Ki (积分)</div>
              <div class="stat-value" style="font-size:18px;">{{ pid.ki }}</div>
            </div>
            <div class="form-field">
              <div class="stat-label">Kd (微分)</div>
              <div class="stat-value" style="font-size:18px;">{{ pid.kd }}</div>
            </div>
          </div>
          <div class="form-row" style="margin-top:24px;" *ngIf="pid">
            <div class="form-field">
              <div class="stat-label">积分项</div>
              <div class="stat-value" style="font-size:16px;color:#1565c0;">{{ pid.integral.toFixed(2) }}</div>
            </div>
            <div class="form-field">
              <div class="stat-label">上次误差</div>
              <div class="stat-value" style="font-size:16px;color:#1565c0;">{{ pid.lastError.toFixed(2) }}</div>
            </div>
            <div class="form-field">
              <div class="stat-label">输出值</div>
              <div class="stat-value" [style.color]="pid.output >= 0 ? '#2e7d32' : '#c62828'" style="font-size:16px;">
                {{ pid.output.toFixed(2) }}
              </div>
            </div>
          </div>
          <div *ngIf="status?.stableSince" style="margin-top:16px;padding:12px;background:#e8f5e9;border-radius:6px;font-size:13px;color:#2e7d32;">
            <mat-icon style="vertical-align:middle;font-size:16px;">check_circle</mat-icon>
            指标已稳定，自 {{ status.stableSince.slice(0, 19) }} 起逐步恢复配额
          </div>
        </div>
      </div>
    </div>

    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin-top: 24px;">
      <div class="card">
        <div class="card-header">错误率趋势</div>
        <div class="card-content">
          <div class="chart-container" style="height:200px">
            <canvas baseChart [type]="'line'" [data]="errorChart" [options]="errorChartOptions"></canvas>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">调节系数变化</div>
        <div class="card-content">
          <div class="chart-container" style="height:200px">
            <canvas baseChart [type]="'line'" [data]="coeffChart" [options]="coeffChartOptions"></canvas>
          </div>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:24px;">
      <div class="card-header">手动控制</div>
      <div class="card-content">
        <form [formGroup]="overrideForm" class="form-row" style="align-items:end;">
          <mat-form-field appearance="outline" class="form-field" style="max-width:200px;">
            <mat-label>手动覆盖系数 (30% - 100%)</mat-label>
            <input matInput type="number" formControlName="coefficient" min="0.3" max="1" step="0.05">
          </mat-form-field>
          <button mat-raised-button color="primary" (click)="applyOverride()"
            [disabled]="!overrideForm.valid || status?.manualOverride">
            应用覆盖
          </button>
          <button mat-stroked-button color="warn" (click)="clearOverride()"
            [disabled]="!status?.manualOverride">
            清除覆盖
          </button>
          <span style="flex:1;"></span>
          <span style="color:#666;font-size:13px;">
            <mat-icon style="vertical-align:middle;font-size:16px;">info</mat-icon>
            手动覆盖后PID自动调节暂停
          </span>
        </form>
      </div>
    </div>

    <div class="card" style="margin-top:24px;">
      <div class="card-header">PID 参数配置</div>
      <div class="card-content">
        <form [formGroup]="configForm">
          <div class="form-row">
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>目标 P99 延迟 (ms)</mat-label>
              <input matInput type="number" formControlName="targetP99LatencyMs">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>错误率阈值 (0~1)</mat-label>
              <input matInput type="number" step="0.01" formControlName="errorRateThreshold">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>最小系数</mat-label>
              <input matInput type="number" step="0.1" formControlName="minCoefficient">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>最大系数</mat-label>
              <input matInput type="number" step="0.1" formControlName="maxCoefficient">
            </mat-form-field>
          </div>
          <div class="form-row">
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>Kp (比例增益)</mat-label>
              <input matInput type="number" step="0.01" formControlName="kp">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>Ki (积分增益)</mat-label>
              <input matInput type="number" step="0.01" formControlName="ki">
            </mat-form-field>
            <mat-form-field appearance="outline" class="form-field">
              <mat-label>Kd (微分增益)</mat-label>
              <input matInput type="number" step="0.01" formControlName="kd">
            </mat-form-field>
            <button mat-raised-button color="primary" (click)="updateConfig()">
              保存配置
            </button>
          </div>
        </form>
      </div>
    </div>
  `
})
export class AdaptiveStatusComponent implements OnInit, OnDestroy {
  private destroy$ = new Subject<void>();
  status: AdaptiveStatus | null = null;
  pid: PIDState | null = null;

  overrideForm: FormGroup;
  configForm: FormGroup;

  latencyChart: any = { labels: [], datasets: [] };
  errorChart: any = { labels: [], datasets: [] };
  coeffChart: any = { labels: [], datasets: [] };

  chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    scales: {
      y: { beginAtZero: true, title: { display: true, text: '延迟(ms)' } }
    },
    plugins: { legend: { display: false } }
  };

  errorChartOptions = {
    ...this.chartOptions,
    scales: {
      y: {
        beginAtZero: true,
        ticks: { callback: (v: number) => (v * 100).toFixed(0) + '%' },
        title: { display: true, text: '错误率' }
      }
    }
  };

  coeffChartOptions = {
    ...this.chartOptions,
    scales: {
      y: {
        min: 0.3,
        max: 1,
        ticks: { callback: (v: number) => (v * 100).toFixed(0) + '%' },
        title: { display: true, text: '系数' }
      }
    }
  };

  constructor(private api: ApiService, private fb: FormBuilder) {
    this.overrideForm = fb.group({ coefficient: [0.7] });
    this.configForm = fb.group({
      targetP99LatencyMs: [200],
      errorRateThreshold: [0.05],
      minCoefficient: [0.3],
      maxCoefficient: [1.0],
      kp: [0.5], ki: [0.1], kd: [0.2]
    });
  }

  get coeffColor(): string {
    const c = this.status?.currentCoefficient || 0;
    if (c >= 0.9) return '#2e7d32';
    if (c >= 0.7) return '#f57f17';
    return '#c62828';
  }

  ngOnInit(): void {
    this.refresh();
    interval(10000).pipe(takeUntil(this.destroy$)).subscribe(() => this.refresh());
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  refresh(): void {
    this.api.getAdaptiveStatus().subscribe(s => {
      this.status = s;
      this.pid = s.pidState;
      this.buildCharts(s);
      this.configForm.patchValue({
        targetP99LatencyMs: s.targetP99LatencyMs,
        errorRateThreshold: s.errorRateThreshold
      });
    });
  }

  private buildCharts(s: AdaptiveStatus): void {
    const fmtLabels = (arr: any[]) => arr.map(p => {
      const d = new Date(p.timestamp);
      return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`;
    });

    this.latencyChart = {
      labels: fmtLabels(s.latencyHistory),
      datasets: [{
        data: s.latencyHistory.map(p => p.value),
        borderColor: '#2196f3',
        backgroundColor: 'rgba(33,150,243,0.1)',
        fill: true,
        tension: 0.3
      }]
    };

    this.errorChart = {
      labels: fmtLabels(s.errorRateHistory),
      datasets: [{
        data: s.errorRateHistory.map(p => p.value),
        borderColor: '#f44336',
        backgroundColor: 'rgba(244,67,54,0.1)',
        fill: true,
        tension: 0.3
      }]
    };

    this.coeffChart = {
      labels: fmtLabels(s.coefficientHistory),
      datasets: [{
        data: s.coefficientHistory.map(p => p.value),
        borderColor: '#4caf50',
        backgroundColor: 'rgba(76,175,80,0.1)',
        fill: true,
        tension: 0.3
      }]
    };
  }

  applyOverride(): void {
    const c = parseFloat(this.overrideForm.value.coefficient);
    this.api.overrideAdaptiveCoeff(c).subscribe(() => this.refresh());
  }

  clearOverride(): void {
    this.api.clearAdaptiveOverride().subscribe(() => this.refresh());
  }

  updateConfig(): void {
    const cfg: AdaptiveConfigUpdate = this.configForm.value;
    this.api.updateAdaptiveConfig(cfg).subscribe(() => this.refresh());
  }
}
