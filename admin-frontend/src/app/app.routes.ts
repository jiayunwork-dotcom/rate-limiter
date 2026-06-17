import { Routes } from '@angular/router';
import { RulesComponent } from './pages/rules/rules.component';
import { QuotaHierarchyComponent } from './pages/quota-hierarchy/quota-hierarchy.component';
import { DashboardComponent } from './pages/dashboard/dashboard.component';
import { AdaptiveStatusComponent } from './pages/adaptive-status/adaptive-status.component';
import { EventsLogComponent } from './pages/events-log/events-log.component';

export const routes: Routes = [
  { path: '', redirectTo: '/dashboard', pathMatch: 'full' },
  { path: 'dashboard', component: DashboardComponent },
  { path: 'rules', component: RulesComponent },
  { path: 'quota-hierarchy', component: QuotaHierarchyComponent },
  { path: 'adaptive-status', component: AdaptiveStatusComponent },
  { path: 'events-log', component: EventsLogComponent }
];
