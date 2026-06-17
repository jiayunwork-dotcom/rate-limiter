import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';

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
    MatIconModule
  ],
  template: `
    <mat-toolbar color="primary" class="app-toolbar">
      <mat-icon>speed</mat-icon>
      <span class="toolbar-title">速率限制网关管理平台</span>
      <span class="spacer"></span>
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
        </mat-nav-list>
      </mat-sidenav>

      <mat-sidenav-content class="content">
        <div class="container">
          <router-outlet></router-outlet>
        </div>
      </mat-sidenav-content>
    </mat-sidenav-container>
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
  `]
})
export class AppComponent {
  title = 'Rate Limiter Admin';
}
