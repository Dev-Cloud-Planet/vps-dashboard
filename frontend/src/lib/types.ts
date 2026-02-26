// ============================================================
// VPS Dashboard - TypeScript type definitions
// ============================================================

export interface SystemMetrics {
  timestamp: string;
  cpu_percent: number;
  ram_percent: number;
  ram_used_mb: number;
  ram_total_mb: number;
  disk_percent: number;
  disk_used_gb: number;
  disk_total_gb: number;
  swap_percent: number;
  load_1m: number;
  load_5m: number;
  load_15m: number;
}

export type ContainerStatus =
  | "running"
  | "exited"
  | "paused"
  | "restarting"
  | "dead";

export type ContainerHealth = "healthy" | "unhealthy" | "none" | null;

export interface Container {
  id: string;
  name: string;
  image: string;
  status: ContainerStatus;
  health: ContainerHealth;
  started_at: string;
  cpu_percent: number;
  mem_percent: number;
  mem_usage_mb: number;
  mem_limit_mb: number;
  is_critical: boolean;
  last_updated: string;
}

export type LoginEventType =
  | "LOGIN_OK"
  | "LOGIN_FAIL"
  | "SESSION"
  | "NEW_USER"
  | "USER_DELETED"
  | "SUDO_DANGER";

export interface LoginEvent {
  id: number;
  timestamp: string;
  event_type: LoginEventType;
  username: string;
  ip: string;
  method: string;
  attempts: number;
  command: string;
  by_user: string;
  geo_country: string;
  geo_city: string;
  geo_isp: string;
  geo_lat: number;
  geo_lon: number;
}

export type AlertStatus = "sent" | "failed" | "rate_limited";

export interface AlertEvent {
  id: number;
  timestamp: string;
  type: string;
  alert_key: string;
  message: string;
  status: AlertStatus;
  http_code: number;
  details: string;
}

export interface BannedIP {
  id: number;
  ip: string;
  jail: string;
  banned_at: string;
  unbanned_at: string;
  country: string;
  city: string;
  isp: string;
  lat: number;
  lon: number;
  is_proxy: boolean;
  is_active: boolean;
}

export interface PortStatus {
  name: string;
  port: number;
  open: boolean;
}

export interface OverviewData {
  metrics: SystemMetrics;
  containers_total: number;
  containers_running: number;
  containers_stopped: number;
  containers_unhealthy: number;
  alerts_24h: number;
  logins_24h: number;
  uptime_seconds: number;
  active_sessions: number;
}

export interface LoginStats {
  total: number;
  success: number;
  failed: number;
  unique_ips: number;
  top_ips: { ip: string; count: number; country: string }[];
  by_type: { type: string; count: number }[];
}

export interface AlertStats {
  total: number;
  by_type: { type: string; count: number }[];
  by_status: { status: string; count: number }[];
}

// WebSocket message types
export type WSMessageType =
  | "metrics"
  | "containers"
  | "login_event"
  | "alert"
  | "monitor_event"
  | "ports";

export interface WSMessage {
  type: WSMessageType;
  data: unknown;
}

// API response wrapper
export interface APIResponse<T> {
  data: T;
  error?: string;
}

// Pagination
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

// Auth
export interface AuthTokenResponse {
  token: string;
  expires_at: string;
}

export interface User {
  username: string;
}
