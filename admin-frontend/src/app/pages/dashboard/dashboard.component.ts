import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatCardModule } from '@angular/material/card';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService } from '../../services/api.service';
import { TrafficSeriesPoint, TenantShareData, HeatmapData } from '../../models/models';
import { Chart, registerables } from 'chart.js';
import { NgChartsModule } from 'ng2-charts';
import { interval, Subject, takeUntil } from 'rxjs';

Chart.register(...registerables);

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, MatCardModule, MatProgressBarModule, NgChartsModule, MatTooltipModule],
  template: `
    <div class="page-header">
      <h1 class="page-title">实时流量大盘</h1>
      <span class="tag tag-green">每5秒自动刷新</span>
    </div>

    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">总请求数(近1小时)</div>
        <div class="stat-value">{{ totalRequests | number }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">通过请求数</div>
        <div class="stat-value" style="color:#2e7d32">{{ totalAllowed | number }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">拒绝请求数</div>
        <div class="stat-value" style="color:#c62828">{{ totalRejected | number }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">通过率</div>
        <div class="stat-value" [style.color]="passRate >= 0.95 ? '#2e7d32' : passRate >= 0.8 ? '#f57f17' : '#c62828'">
          {{ (passRate * 100).toFixed(2) }}%
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">各API请求趋势（近1小时）</div>
      <div class="card-content">
        <div class="chart-container" style="height:350px">
          <canvas baseChart
            [type]="'line'"
            [data]="trafficChartData"
            [options]="trafficChartOptions">
          </canvas>
        </div>
      </div>
    </div>

    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 24px;">
      <div class="card">
        <div class="card-header">租户流量占比</div>
        <div class="card-content">
          <div class="chart-container" style="height:300px">
            <canvas baseChart
              [type]="'pie'"
              [data]="tenantChartData"
              [options]="tenantChartOptions">
            </canvas>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">各时段流量密度热力图</div>
        <div class="card-content">
          <div style="display: flex; align-items: flex-start; gap: 16px;">
            <div style="display: flex; flex-direction: column; gap: 4px; padding-top: 20px; font-size: 11px; color: #666;">
              <div *ngFor="let day of weekdays">{{ day }}</div>
            </div>
            <div class="heatmap-grid" [style.grid-template-columns]="'repeat(24, 1fr)'">
              <div *ngFor="let cell of heatmapCells"
                class="heatmap-cell"
                [style.background]="getHeatColor(cell.count)"
                [matTooltip]="weekdays[cell.weekday] + ' ' + cell.hour + ':00 - ' + cell.count + ' 次'">
              </div>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 8px; margin-top: 16px; font-size: 12px; color: #666;">
            <span>低</span>
            <div *ngFor="let c of heatLegend" [style.background]="c.color" style="width:20px;height:16px;border-radius:2px;"></div>
            <span>高</span>
          </div>
        </div>
      </div>
    </div>
  `
})
export class DashboardComponent implements OnInit, OnDestroy {
  private destroy$ = new Subject<void>();
  weekdays = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];

  totalRequests = 0;
  totalAllowed = 0;
  totalRejected = 0;
  passRate = 0;

  trafficPoints: TrafficSeriesPoint[] = [];
  tenantShares: TenantShareData[] = [];
  heatmapData: HeatmapData[] = [];

  trafficChartData: any = { datasets: [] };
  trafficChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    scales: {
      x: { title: { display: true, text: '时间' } },
      y: { title: { display: true, text: '请求数' }, beginAtZero: true }
    },
    plugins: { legend: { position: 'top' as const } }
  };

  tenantChartData: any = { labels: [], datasets: [] };
  tenantChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { position: 'right' as const } }
  };

  heatmapCells: Array<{ hour: number; weekday: number; count: number }> = [];
  heatLegend = [
    { color: '#ebedf0' },
    { color: '#9be9a8' },
    { color: '#40c463' },
    { color: '#30a14e' },
    { color: '#216e39' }
  ];

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadData();
    interval(5000).pipe(takeUntil(this.destroy$)).subscribe(() => this.loadData());
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  private loadData(): void {
    this.api.getTrafficSeries().pipe(takeUntil(this.destroy$)).subscribe(data => {
      this.trafficPoints = data;
      this.calcStats();
      this.buildTrafficChart();
    });
    this.api.getTenantShare().pipe(takeUntil(this.destroy$)).subscribe(data => {
      this.tenantShares = data;
      this.buildTenantChart();
    });
    this.api.getHeatmap().pipe(takeUntil(this.destroy$)).subscribe(data => {
      this.heatmapData = data;
      this.buildHeatmap();
    });
  }

  private calcStats(): void {
    this.totalAllowed = this.trafficPoints.reduce((s, p) => s + p.allowed, 0);
    this.totalRejected = this.trafficPoints.reduce((s, p) => s + p.rejected, 0);
    this.totalRequests = this.totalAllowed + this.totalRejected;
    this.passRate = this.totalRequests > 0 ? this.totalAllowed / this.totalRequests : 0;
  }

  private buildTrafficChart(): void {
    const apiMap = new Map<string, Map<string, { allowed: number; rejected: number }>>();
    const labelsSet = new Set<string>();

    for (const p of this.trafficPoints) {
      if (!apiMap.has(p.apiPath)) apiMap.set(p.apiPath, new Map());
      const m = apiMap.get(p.apiPath)!;
      const key = p.timestamp;
      m.set(key, { allowed: p.allowed, rejected: p.rejected });
      labelsSet.add(key);
    }

    const labels = Array.from(labelsSet).sort();
    const apis = Array.from(apiMap.keys()).slice(0, 5);
    const colors = [
      { bg: 'rgba(33,150,243,0.1)', border: '#2196f3' },
      { bg: 'rgba(76,175,80,0.1)', border: '#4caf50' },
      { bg: 'rgba(255,152,0,0.1)', border: '#ff9800' },
      { bg: 'rgba(156,39,176,0.1)', border: '#9c27b0' },
      { bg: 'rgba(0,188,212,0.1)', border: '#00bcd4' }
    ];

    const datasets: any[] = [];
    apis.forEach((api, idx) => {
      const c = colors[idx % colors.length];
      const m = apiMap.get(api)!;
      datasets.push({
        label: `${api} (通过)`,
        data: labels.map(l => m.get(l)?.allowed || 0),
        borderColor: c.border,
        backgroundColor: c.bg,
        fill: true,
        tension: 0.3
      });
      datasets.push({
        label: `${api} (拒绝)`,
        data: labels.map(l => m.get(l)?.rejected || 0),
        borderColor: '#f44336',
        backgroundColor: 'rgba(244,67,54,0.05)',
        borderDash: [5, 5],
        fill: false,
        tension: 0.3
      });
    });

    this.trafficChartData = {
      labels: labels.map(l => {
        const d = new Date(l);
        return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`;
      }),
      datasets
    };
  }

  private buildTenantChart(): void {
    const top = this.tenantShares.slice(0, 8);
    const colors = ['#2196f3', '#4caf50', '#ff9800', '#9c27b0', '#00bcd4', '#e91e63', '#607d8b', '#795548'];
    this.tenantChartData = {
      labels: top.map(t => t.tenantName || t.tenantId),
      datasets: [{
        data: top.map(t => t.requestCount),
        backgroundColor: colors
      }]
    };
  }

  private buildHeatmap(): void {
    const cells: any[] = [];
    for (let w = 0; w < 7; w++) {
      for (let h = 0; h < 24; h++) {
        const found = this.heatmapData.find(d => d.weekday === w && d.hour === h);
        cells.push({ hour: h, weekday: w, count: found?.count || 0 });
      }
    }
    this.heatmapCells = cells;
  }

  getHeatColor(count: number): string {
    if (count === 0) return '#ebedf0';
    if (count < 10) return '#9be9a8';
    if (count < 50) return '#40c463';
    if (count < 200) return '#30a14e';
    return '#216e39';
  }
}
