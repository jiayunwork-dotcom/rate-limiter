import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet, RouterLink, RouterLinkActive, Router } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatBadgeModule } from '@angular/material/badge';
import { MatButtonModule } from '@angular/material/button';
import { Subscription } from 'rxjs';
import { WebSocketService } from './services/websocket.service';
import { ApiService } from './services/api.service';
import { AlertPushMessage, AlertSeverity } from './models/models';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    MatToolbarModule,
    MatSidenavModule,
    MatListModule,
    MatIconModule,
    MatBadgeModule,
    MatButtonModule
  ],
  template: `
    <mat-toolbar color="primary" class="app-toolbar">
      <mat-icon>speed</mat-icon>
      <span class="toolbar-title">速率限制网关管理平台</span>
      <span class="spacer"></span>
      <button mat-icon-button class="alert-bell" (click)="goToAlertCenter()" [matBadge]="firingCount"
              [matBadgeColor]="firingCount > 0 ? 'warn' : undefined" matBadgeSize="small">
        <mat-icon>notifications</mat-icon>
      </button>
      <span class="status-badge">
        <span class="status-dot online"></span>
        系统运行中
      </span>
    </mat-toolbar>

    <mat-sidenav-container class="sidenav-container">
      <mat-sidenav mode="side" opened class="sidenav">
        <mat-nav-list>
          <a mat-list-item routerLink="/dashboard" routerLinkActive="active">
            <mat-icon matListItemIcon>dashboard</mat-icon>
            <span matListItemTitle>实时流量大盘</span>
          </a>
          <a mat-list-item routerLink="/rules" routerLinkActive="active">
            <mat-icon matListItemIcon>rule</mat-icon>
            <span matListItemTitle>规则管理</span>
          </a>
          <a mat-list-item routerLink="/rule-templates" routerLinkActive="active">
            <mat-icon matListItemIcon>category</mat-icon>
            <span matListItemTitle>规则模板</span>
          </a>
          <a mat-list-item routerLink="/quota-hierarchy" routerLinkActive="active">
            <mat-icon matListItemIcon>account_tree</mat-icon>
            <span matListItemTitle>配额层级</span>
          </a>
          <a mat-list-item routerLink="/adaptive-status" routerLinkActive="active">
            <mat-icon matListItemIcon>auto_graph</mat-icon>
            <span matListItemTitle>自适应状态</span>
          </a>
          <a mat-list-item routerLink="/events-log" routerLinkActive="active">
            <mat-icon matListItemIcon>warning_amber</mat-icon>
            <span matListItemTitle>告警日志</span>
          </a>
          <a mat-list-item routerLink="/alert-center" routerLinkActive="active">
            <mat-icon matListItemIcon>notifications_active</mat-icon>
            <span matListItemTitle>告警中心</span>
          </a>
        </mat-nav-list>
      </mat-sidenav>

      <mat-sidenav-content class="content">
        <div class="container">
          <router-outlet></router-outlet>
        </div>
      </mat-sidenav-content>
    </mat-sidenav-container>

    <div class="toast-container">
      <div *ngFor="let toast of toasts; trackBy: trackByToastId"
           class="toast" [ngClass]="'toast-' + toast.severity">
        <div class="toast-icon">
          <mat-icon>{{ getToastIcon(toast.severity) }}</mat-icon>
        </div>
        <div class="toast-content">
          <div class="toast-title">{{ toast.ruleName }}</div>
          <div class="toast-desc">{{ toast.dimensionValue }}</div>
        </div>
        <button mat-icon-button class="toast-close" (click)="removeToast(toast.id)">
          <mat-icon>close</mat-icon>
        </button>
      </div>
    </div>
  `,
  styles: [`
    .app-toolbar {
      position: sticky;
      top: 0;
      z-index: 1000;
    }
    .toolbar-title {
      margin-left: 12px;
      font-size: 18px;
    }
    .spacer {
      flex: 1;
    }
    .alert-bell {
      margin-right: 16px;
      color: white !important;
    }
    .status-badge {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;
      opacity: 0.9;
    }
    .status-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
    }
    .status-dot.online {
      background: #4caf50;
      box-shadow: 0 0 8px #4caf50;
    }
    .sidenav-container {
      height: calc(100vh - 64px);
    }
    .sidenav {
      width: 240px;
      background: #fff;
      box-shadow: 2px 0 4px rgba(0,0,0,0.05);
    }
    .content {
      background: #f5f5f5;
    }
    .mat-mdc-list-item.active {
      background: #e3f2fd;
      color: #1976d2;
    }
    .toast-container {
      position: fixed;
      bottom: 24px;
      right: 24px;
      z-index: 9999;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .toast {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px 16px;
      border-radius: 8px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
      min-width: 320px;
      max-width: 400px;
      animation: slideIn 0.3s ease-out;
    }
    @keyframes slideIn {
      from {
        transform: translateX(100%);
        opacity: 0;
      }
      to {
        transform: translateX(0);
        opacity: 1;
      }
    }
    .toast-critical {
      background: #ffebee;
      border-left: 4px solid #f44336;
    }
    .toast-warning {
      background: #fff8e1;
      border-left: 4px solid #ff9800;
    }
    .toast-info {
      background: #e3f2fd;
      border-left: 4px solid #2196f3;
    }
    .toast-icon {
      flex-shrink: 0;
    }
    .toast-critical .toast-icon { color: #f44336; }
    .toast-warning .toast-icon { color: #ff9800; }
    .toast-info .toast-icon { color: #2196f3; }
    .toast-content {
      flex: 1;
      min-width: 0;
    }
    .toast-title {
      font-weight: 500;
      font-size: 14px;
      margin-bottom: 2px;
    }
    .toast-desc {
      font-size: 12px;
      color: #666;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .toast-close {
      flex-shrink: 0;
      opacity: 0.6;
    }
    .toast-close:hover {
      opacity: 1;
    }
  `]
})
export class AppComponent implements OnInit, OnDestroy {
  title = 'Rate Limiter Admin';
  firingCount = 0;
  toasts: Array<{ id: number; ruleName: string; dimensionValue: string; severity: AlertSeverity }> = [];
  private toastId = 0;
  private wsSub: Subscription | null = null;

  constructor(
    private router: Router,
    private wsService: WebSocketService,
    private api: ApiService
  ) {}

  ngOnInit(): void {
    this.wsService.connect();
    this.loadFiringCount();

    this.wsSub = this.wsService.alerts$.subscribe(alert => {
      this.loadFiringCount();
      this.showToast(alert);
    });

    setInterval(() => {
      this.loadFiringCount();
    }, 30000);
  }

  ngOnDestroy(): void {
    this.wsSub?.unsubscribe();
  }

  loadFiringCount(): void {
    this.api.getAlertStats().subscribe(stats => {
      this.firingCount = stats.firingCount;
    }, () => {});
  }

  showToast(alert: AlertPushMessage): void {
    const id = ++this.toastId;
    this.toasts.push({
      id,
      ruleName: alert.ruleName,
      dimensionValue: alert.dimensionValue,
      severity: alert.severity
    });
    setTimeout(() => {
      this.removeToast(id);
    }, 3000);
  }

  removeToast(id: number): void {
    const idx = this.toasts.findIndex(t => t.id === id);
    if (idx > -1) {
      this.toasts.splice(idx, 1);
    }
  }

  trackByToastId(index: number, item: any): number {
    return item.id;
  }

  getToastIcon(severity: AlertSeverity): string {
    switch (severity) {
      case 'critical': return 'error';
      case 'warning': return 'warning';
      default: return 'info';
    }
  }

  goToAlertCenter(): void {
    this.router.navigate(['/alert-center']);
  }
}
