"use client";

import { useWS } from "@/providers/websocket-provider";
import useSWR from "swr";
import { swrFetcher } from "@/lib/api";
import { cn, formatUptime, formatRelativeTime, getPercentColor, getPercentBgColor } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import type { OverviewData, SystemMetrics, LoginEvent, AlertEvent, Container } from "@/lib/types";
import {
  Cpu,
  MemoryStick,
  HardDrive,
  Container as ContainerIcon,
  Activity,
  Clock,
  ShieldCheck,
  ShieldX,
  UserPlus,
  UserMinus,
  Terminal,
  AlertTriangle,
  Bell,
  CheckCircle,
  XCircle,
} from "lucide-react";

// -------------------------------------------------------------------
// Circular gauge SVG component
// -------------------------------------------------------------------
function CircularGauge({
  value,
  label,
  size = 120,
  strokeWidth = 8,
}: {
  value: number;
  label: string;
  size?: number;
  strokeWidth?: number;
}) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (value / 100) * circumference;
  const colorClass = getPercentColor(value);

  // Map text color class to stroke color
  const strokeColor =
    value >= 90
      ? "#ef4444"
      : value >= 75
        ? "#f97316"
        : value >= 50
          ? "#eab308"
          : "#22c55e";

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative" style={{ width: size, height: size }}>
        <svg width={size} height={size} className="-rotate-90">
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            stroke="#27272a"
            strokeWidth={strokeWidth}
            fill="none"
          />
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            stroke={strokeColor}
            strokeWidth={strokeWidth}
            fill="none"
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            className="transition-all duration-700 ease-out"
          />
        </svg>
        <div className="absolute inset-0 flex items-center justify-center">
          <span className={cn("text-xl font-bold", colorClass)}>
            {value.toFixed(1)}%
          </span>
        </div>
      </div>
      <span className="text-sm font-medium text-zinc-400">{label}</span>
    </div>
  );
}

// -------------------------------------------------------------------
// Stat card component
// -------------------------------------------------------------------
function StatCard({
  title,
  value,
  icon: Icon,
  percent,
  color,
  subtitle,
}: {
  title: string;
  value: string;
  icon: React.ElementType;
  percent?: number;
  color: string;
  subtitle?: string;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <p className="text-sm text-zinc-400">{title}</p>
            <p className="text-2xl font-bold text-zinc-100">{value}</p>
            {subtitle && (
              <p className="text-xs text-zinc-500">{subtitle}</p>
            )}
          </div>
          <div
            className={cn(
              "flex h-12 w-12 items-center justify-center rounded-xl",
              color
            )}
          >
            <Icon className="h-6 w-6" />
          </div>
        </div>
        {percent !== undefined && (
          <div className="mt-3">
            <div className="h-2 w-full rounded-full bg-zinc-800">
              <div
                className={cn(
                  "h-2 rounded-full transition-all duration-700",
                  getPercentBgColor(percent)
                )}
                style={{ width: `${Math.min(percent, 100)}%` }}
              />
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// -------------------------------------------------------------------
// Login event icon/color helpers
// -------------------------------------------------------------------
function getLoginEventIcon(eventType: string) {
  switch (eventType) {
    case "LOGIN_OK":
      return <ShieldCheck className="h-4 w-4 text-green-400" />;
    case "LOGIN_FAIL":
      return <ShieldX className="h-4 w-4 text-red-400" />;
    case "NEW_USER":
      return <UserPlus className="h-4 w-4 text-blue-400" />;
    case "USER_DELETED":
      return <UserMinus className="h-4 w-4 text-yellow-400" />;
    case "SUDO_DANGER":
      return <Terminal className="h-4 w-4 text-orange-400" />;
    case "SESSION":
      return <Activity className="h-4 w-4 text-cyan-400" />;
    default:
      return <Activity className="h-4 w-4 text-zinc-400" />;
  }
}

function getAlertStatusBadge(status: string) {
  switch (status) {
    case "sent":
      return <Badge variant="success">Sent</Badge>;
    case "failed":
      return <Badge variant="destructive">Failed</Badge>;
    case "rate_limited":
      return <Badge variant="warning">Rate Limited</Badge>;
    default:
      return <Badge variant="secondary">{status}</Badge>;
  }
}

// -------------------------------------------------------------------
// Overview page
// -------------------------------------------------------------------
export default function DashboardOverview() {
  const ws = useWS();
  const { data: overview, isLoading } = useSWR<OverviewData>(
    "/overview",
    swrFetcher,
    { refreshInterval: 30000 }
  );

  // Use WebSocket data when available, fall back to SWR
  const metrics: SystemMetrics | null = ws.lastMetrics || overview?.metrics || null;
  const containers: Container[] = ws.lastContainers.length > 0 ? ws.lastContainers : [];
  const recentAlerts: AlertEvent[] = ws.recentAlerts;
  const recentLogins: LoginEvent[] = ws.recentLogins;

  const containersRunning =
    containers.length > 0
      ? containers.filter((c) => c.status === "running").length
      : overview?.containers_running ?? 0;
  const containersTotal =
    containers.length > 0 ? containers.length : overview?.containers_total ?? 0;
  const uptimeSeconds = overview?.uptime_seconds ?? 0;

  if (isLoading && !metrics) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">Overview</h1>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-32 rounded-xl" />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Skeleton className="h-64 rounded-xl" />
          <Skeleton className="h-64 rounded-xl" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-zinc-100">Overview</h1>

      {/* Top row: 4 stat cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="CPU Usage"
          value={`${(metrics?.cpu_percent ?? 0).toFixed(1)}%`}
          icon={Cpu}
          percent={metrics?.cpu_percent ?? 0}
          color="bg-blue-600/20 text-blue-400"
        />
        <StatCard
          title="RAM Usage"
          value={`${(metrics?.ram_percent ?? 0).toFixed(1)}%`}
          icon={MemoryStick}
          percent={metrics?.ram_percent ?? 0}
          color="bg-purple-600/20 text-purple-400"
          subtitle={
            metrics
              ? `${metrics.ram_used_mb.toFixed(0)} / ${metrics.ram_total_mb.toFixed(0)} MB`
              : undefined
          }
        />
        <StatCard
          title="Disk Usage"
          value={`${(metrics?.disk_percent ?? 0).toFixed(1)}%`}
          icon={HardDrive}
          percent={metrics?.disk_percent ?? 0}
          color="bg-amber-600/20 text-amber-400"
          subtitle={
            metrics
              ? `${metrics.disk_used_gb.toFixed(1)} / ${metrics.disk_total_gb.toFixed(1)} GB`
              : undefined
          }
        />
        <StatCard
          title="Containers"
          value={`${containersRunning} / ${containersTotal}`}
          icon={ContainerIcon}
          percent={containersTotal > 0 ? (containersRunning / containersTotal) * 100 : 0}
          color="bg-green-600/20 text-green-400"
          subtitle={`${containersRunning} running`}
        />
      </div>

      {/* Second row: Gauges + Container grid */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* System gauges */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">System Resources</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap items-center justify-around gap-4 py-4">
              <CircularGauge value={metrics?.cpu_percent ?? 0} label="CPU" />
              <CircularGauge value={metrics?.ram_percent ?? 0} label="RAM" />
              <CircularGauge value={metrics?.disk_percent ?? 0} label="Disk" />
            </div>
          </CardContent>
        </Card>

        {/* Container status grid */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Containers</CardTitle>
          </CardHeader>
          <CardContent>
            {containers.length > 0 ? (
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                {containers.map((c) => (
                  <div
                    key={c.id}
                    className="flex items-center gap-2 rounded-lg border border-zinc-800 bg-zinc-900 p-3"
                  >
                    <div
                      className={cn(
                        "h-2.5 w-2.5 rounded-full",
                        c.status === "running"
                          ? "bg-green-500"
                          : c.status === "exited"
                            ? "bg-red-500"
                            : "bg-yellow-500"
                      )}
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-zinc-200">
                        {c.name}
                      </p>
                      <p className="text-xs text-zinc-500">{c.status}</p>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="flex h-40 items-center justify-center text-zinc-500">
                No container data available
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Third row: Recent alerts + Recent logins */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* Recent alerts */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base">
              <Bell className="h-4 w-4" />
              Recent Alerts
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-80 space-y-2 overflow-y-auto pr-2">
              {recentAlerts.length > 0 ? (
                recentAlerts.slice(0, 20).map((alert, i) => (
                  <div
                    key={alert.id || i}
                    className="flex items-start gap-3 rounded-lg border border-zinc-800 bg-zinc-900/50 p-3"
                  >
                    <div className="mt-0.5">
                      {alert.status === "sent" ? (
                        <CheckCircle className="h-4 w-4 text-green-400" />
                      ) : alert.status === "failed" ? (
                        <XCircle className="h-4 w-4 text-red-400" />
                      ) : (
                        <AlertTriangle className="h-4 w-4 text-yellow-400" />
                      )}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <p className="truncate text-sm font-medium text-zinc-200">
                          {alert.alert_key}
                        </p>
                        {getAlertStatusBadge(alert.status)}
                      </div>
                      <p className="mt-0.5 truncate text-xs text-zinc-400">
                        {alert.message}
                      </p>
                      <p className="mt-0.5 text-xs text-zinc-600">
                        {formatRelativeTime(alert.timestamp)}
                      </p>
                    </div>
                  </div>
                ))
              ) : (
                <div className="flex h-40 items-center justify-center text-zinc-500">
                  No recent alerts
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Recent logins */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base">
              <ShieldCheck className="h-4 w-4" />
              Recent Logins
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-80 space-y-2 overflow-y-auto pr-2">
              {recentLogins.length > 0 ? (
                recentLogins.slice(0, 10).map((login, i) => (
                  <div
                    key={login.id || i}
                    className="flex items-start gap-3 rounded-lg border border-zinc-800 bg-zinc-900/50 p-3"
                  >
                    <div className="mt-0.5">
                      {getLoginEventIcon(login.event_type)}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-zinc-200">
                          {login.username}
                        </span>
                        <Badge
                          variant={
                            login.event_type === "LOGIN_OK"
                              ? "success"
                              : login.event_type === "LOGIN_FAIL"
                                ? "destructive"
                                : "secondary"
                          }
                        >
                          {login.event_type.replace("_", " ")}
                        </Badge>
                      </div>
                      <p className="mt-0.5 text-xs text-zinc-400">
                        {login.ip}
                        {login.geo_country ? ` - ${login.geo_country}` : ""}
                        {login.method ? ` via ${login.method}` : ""}
                      </p>
                      <p className="mt-0.5 text-xs text-zinc-600">
                        {formatRelativeTime(login.timestamp)}
                      </p>
                    </div>
                  </div>
                ))
              ) : (
                <div className="flex h-40 items-center justify-center text-zinc-500">
                  No recent login events
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Bottom: Load average + Uptime */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <Activity className="h-5 w-5 text-blue-400" />
              <div>
                <p className="text-sm text-zinc-400">Load Average</p>
                <div className="flex gap-4 text-lg font-bold text-zinc-100">
                  <span className="text-red-400">
                    {(metrics?.load_1m ?? 0).toFixed(2)}
                  </span>
                  <span className="text-amber-400">
                    {(metrics?.load_5m ?? 0).toFixed(2)}
                  </span>
                  <span className="text-green-400">
                    {(metrics?.load_15m ?? 0).toFixed(2)}
                  </span>
                </div>
                <p className="text-xs text-zinc-500">1m / 5m / 15m</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <Clock className="h-5 w-5 text-green-400" />
              <div>
                <p className="text-sm text-zinc-400">Uptime</p>
                <p className="text-lg font-bold text-zinc-100">
                  {formatUptime(uptimeSeconds)}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
