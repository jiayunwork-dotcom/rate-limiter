import { Injectable, OnDestroy } from '@angular/core';
import { Subject, Observable } from 'rxjs';
import { WebSocketMessage, AlertPushMessage, AlertEvent } from '../models/models';

@Injectable({ providedIn: 'root' })
export class WebSocketService implements OnDestroy {
  private ws: WebSocket | null = null;
  private messageSubject = new Subject<WebSocketMessage>();
  private alertSubject = new Subject<AlertPushMessage>();
  private reconnectTimer: any = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 3000;

  messages$: Observable<WebSocketMessage> = this.messageSubject.asObservable();
  alerts$: Observable<AlertPushMessage> = this.alertSubject.asObservable();

  connect(): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/alerts`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.reconnectAttempts = 0;
      };

      this.ws.onmessage = (event) => {
        try {
          const data: WebSocketMessage = JSON.parse(event.data);
          this.messageSubject.next(data);

          if (data.type === 'alert_firing') {
            this.alertSubject.next(data.payload as AlertPushMessage);
          }
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e);
        }
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.scheduleReconnect();
      };
    } catch (e) {
      console.error('Failed to create WebSocket:', e);
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.warn('Max reconnection attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.min(this.reconnectAttempts, 5);

    this.reconnectTimer = setTimeout(() => {
      console.log(`Reconnecting... attempt ${this.reconnectAttempts}`);
      this.connect();
    }, delay);
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  ngOnDestroy(): void {
    this.disconnect();
    this.messageSubject.complete();
    this.alertSubject.complete();
  }
}
