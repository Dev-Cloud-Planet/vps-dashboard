"use client";

import { useState, useMemo } from "react";
import useSWR from "swr";
import { swrFetcher } from "@/lib/api";
import { useWS } from "@/providers/websocket-provider";
import { cn, formatDate, formatRelativeTime } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { AlertEvent, AlertStats, PaginatedResponse } from "@/lib/types";
import {
  Bell,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Cpu,
  Container,
  Shield,
  Activity,
  ChevronLeft,
  ChevronRight,
  Send,
  Ban,
  Clock as ClockIcon,
} from "lucide-react";

// -------------------------------------------------------------------
// Alert type icon
// -------------------------------------------------------------------
function getAlertTypeIcon(type: string) {
  switch (type) {
    case "resource":
      return <Cpu className="h-5 w-5 text-blue-400" />;
    case "container":
      return <Container className="h-5 w-5 text-purple-400" />;
    case "login":
      return <Shield className="h-5 w-5 text-orange-400" />;
    case "service":
      return <Activity className="h-5 w-5 text-cyan-400" />;
    default:
      return <Bell className="h-5 w-5 text-zinc-400" />;
  }
}

function getStatusIcon(status: string) {
  switch (status) {
    case "sent":
      return <CheckCircle className="h-4 w-4 text-green-400" />;
    case "failed":
      return <XCircle className="h-4 w-4 text-red-400" />;
    case "rate_limited":
      return <AlertTriangle className="h-4 w-4 text-yellow-400" />;
    default:
      return <Bell className="h-4 w-4 text-zinc-400" />;
  }
}

// -------------------------------------------------------------------
// Alerts page
// -------------------------------------------------------------------
export default function AlertsPage() {
  const ws = useWS();
  const [page, setPage] = useState(1);
  const [typeFilter, setTypeFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  // Build query string
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("per_page", "50");
  if (typeFilter !== "all") params.set("type", typeFilter);
  if (statusFilter !== "all") params.set("status", statusFilter);

  const { data: alertsData, isLoading } = useSWR<PaginatedResponse<AlertEvent>>(
    `/alerts?${params.toString()}`,
    swrFetcher,
    { refreshInterval: 30000 }
  );

  const { data: stats } = useSWR<AlertStats>("/alerts/stats", swrFetcher, {
    refreshInterval: 60000,
  });

  // Merge WS alerts with API data on first page
  const alerts = useMemo(() => {
    const apiAlerts = alertsData?.data ?? [];
    if (page === 1 && typeFilter === "all" && statusFilter === "all" && ws.recentAlerts.length > 0) {
      const existingIds = new Set(apiAlerts.map((a) => a.id));
      const newFromWs = ws.recentAlerts.filter((a) => !existingIds.has(a.id));
      return [...newFromWs, ...apiAlerts];
    }
    return apiAlerts;
  }, [alertsData, ws.recentAlerts, page, typeFilter, statusFilter]);

  const totalPages = alertsData
    ? Math.ceil(alertsData.total / alertsData.per_page)
    : 1;

  // Compute stats from API response or WS
  const totalSent = stats?.count_by_status?.["sent"] ?? 0;
  const totalFailed = stats?.count_by_status?.["failed"] ?? 0;
  const totalRateLimited = stats?.count_by_status?.["rate_limited"] ?? 0;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-zinc-100">Alerts</h1>

      {/* Stats cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-green-600/20">
              <Send className="h-5 w-5 text-green-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-green-400">{totalSent}</p>
              <p className="text-xs text-zinc-400">Total Sent</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-red-600/20">
              <XCircle className="h-5 w-5 text-red-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-red-400">{totalFailed}</p>
              <p className="text-xs text-zinc-400">Failed</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-yellow-600/20">
              <Ban className="h-5 w-5 text-yellow-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-yellow-400">
                {totalRateLimited}
              </p>
              <p className="text-xs text-zinc-400">Rate Limited</p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="space-y-1">
          <label className="text-xs font-medium text-zinc-500">Type</label>
          <select
            value={typeFilter}
            onChange={(e) => {
              setTypeFilter(e.target.value);
              setPage(1);
            }}
            className="block w-full rounded-md border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:outline-none focus:ring-1 focus:ring-zinc-400"
          >
            <option value="all">All Types</option>
            <option value="resource">Resource</option>
            <option value="container">Container</option>
            <option value="login">Login</option>
            <option value="service">Service</option>
          </select>
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-zinc-500">Status</label>
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setPage(1);
            }}
            className="block w-full rounded-md border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 focus:outline-none focus:ring-1 focus:ring-zinc-400"
          >
            <option value="all">All Status</option>
            <option value="sent">Sent</option>
            <option value="failed">Failed</option>
            <option value="rate_limited">Rate Limited</option>
          </select>
        </div>
      </div>

      {/* Alert list */}
      {isLoading && alerts.length === 0 ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-20 rounded-xl" />
          ))}
        </div>
      ) : (
        <div className="space-y-3">
          {alerts.map((alert, i) => (
            <Card key={alert.id || i}>
              <CardContent className="flex items-start gap-4 p-4">
                {/* Type icon */}
                <div className="mt-1 shrink-0">
                  {getAlertTypeIcon(alert.type)}
                </div>

                {/* Content */}
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium text-zinc-200">
                      {alert.alert_key}
                    </span>
                    <Badge variant="outline" className="text-xs">
                      {alert.type}
                    </Badge>
                    <Badge
                      variant={
                        alert.status === "sent"
                          ? "success"
                          : alert.status === "failed"
                            ? "destructive"
                            : "warning"
                      }
                    >
                      <span className="flex items-center gap-1">
                        {getStatusIcon(alert.status)}
                        {alert.status.replace("_", " ")}
                      </span>
                    </Badge>
                  </div>
                  <p className="mt-1 text-sm text-zinc-400 line-clamp-2">
                    {alert.message}
                  </p>
                  <div className="mt-2 flex items-center gap-1 text-xs text-zinc-500">
                    <ClockIcon className="h-3 w-3" />
                    {formatDate(alert.timestamp)}
                    <span className="ml-1 text-zinc-600">
                      ({formatRelativeTime(alert.timestamp)})
                    </span>
                  </div>
                  {alert.details && (
                    <p className="mt-1 text-xs text-zinc-600 font-mono">
                      {alert.details}
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}

          {alerts.length === 0 && (
            <div className="flex h-48 items-center justify-center rounded-xl border border-zinc-800 text-zinc-500">
              No alerts found
            </div>
          )}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
          >
            <ChevronLeft className="h-4 w-4" />
            Prev
          </Button>
          <span className="text-sm text-zinc-400">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage(page + 1)}
          >
            Next
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      )}
    </div>
  );
}
