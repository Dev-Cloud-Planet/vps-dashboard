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
import type { LoginEvent, LoginStats, BannedIP, PaginatedResponse } from "@/lib/types";
import {
  ShieldCheck,
  ShieldX,
  UserPlus,
  UserMinus,
  Terminal,
  Activity,
  Globe,
  Clock,
  Ban,
  BarChart3,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  PieChart,
  Pie,
  Cell,
  Legend,
} from "recharts";

// -------------------------------------------------------------------
// Country flag helper
// -------------------------------------------------------------------
function countryFlag(code: string): string {
  if (!code || code.length !== 2) return "";
  const codePoints = code
    .toUpperCase()
    .split("")
    .map((char) => 127397 + char.charCodeAt(0));
  return String.fromCodePoint(...codePoints);
}

// -------------------------------------------------------------------
// Login timeline event icon
// -------------------------------------------------------------------
function getEventIcon(eventType: string) {
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

function getEventColor(eventType: string): string {
  switch (eventType) {
    case "LOGIN_OK":
      return "border-green-500/30";
    case "LOGIN_FAIL":
      return "border-red-500/30";
    case "NEW_USER":
      return "border-blue-500/30";
    case "USER_DELETED":
      return "border-yellow-500/30";
    case "SUDO_DANGER":
      return "border-orange-500/30";
    case "SESSION":
      return "border-cyan-500/30";
    default:
      return "border-zinc-700";
  }
}

function getEventDotColor(eventType: string): string {
  switch (eventType) {
    case "LOGIN_OK":
      return "bg-green-500";
    case "LOGIN_FAIL":
      return "bg-red-500";
    case "NEW_USER":
      return "bg-blue-500";
    case "USER_DELETED":
      return "bg-yellow-500";
    case "SUDO_DANGER":
      return "bg-orange-500";
    case "SESSION":
      return "bg-cyan-500";
    default:
      return "bg-zinc-500";
  }
}

// -------------------------------------------------------------------
// Tab: Login Timeline
// -------------------------------------------------------------------
function LoginTimeline() {
  const ws = useWS();
  const [page, setPage] = useState(1);
  const { data, isLoading } = useSWR<PaginatedResponse<LoginEvent>>(
    `/logins?page=${page}&per_page=50`,
    swrFetcher,
    { refreshInterval: 30000 }
  );

  // Merge WS data with API data, deduplicating by id
  const events = useMemo(() => {
    const apiEvents = data?.data ?? [];
    const wsEvents = ws.recentLogins;

    if (page === 1 && wsEvents.length > 0) {
      const existingIds = new Set(apiEvents.map((e) => e.id));
      const newFromWs = wsEvents.filter((e) => !existingIds.has(e.id));
      return [...newFromWs, ...apiEvents];
    }
    return apiEvents;
  }, [data, ws.recentLogins, page]);

  const totalPages = data ? Math.ceil(data.total / data.per_page) : 1;

  if (isLoading && events.length === 0) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 8 }).map((_, i) => (
          <Skeleton key={i} className="h-16 rounded-lg" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Timeline */}
      <div className="relative space-y-0">
        {/* Vertical line */}
        <div className="absolute left-[19px] top-2 bottom-2 w-px bg-zinc-800" />

        {events.map((event, i) => (
          <div key={event.id || i} className="relative flex gap-4 py-2">
            {/* Dot on the line */}
            <div className="relative z-10 flex h-10 w-10 shrink-0 items-center justify-center">
              <div
                className={cn(
                  "h-3 w-3 rounded-full ring-4 ring-zinc-950",
                  getEventDotColor(event.event_type)
                )}
              />
            </div>

            {/* Event content */}
            <div
              className={cn(
                "flex-1 rounded-lg border bg-zinc-900/50 p-3",
                getEventColor(event.event_type)
              )}
            >
              <div className="flex flex-wrap items-center gap-2">
                {getEventIcon(event.event_type)}
                <span className="font-medium text-zinc-200">
                  {event.username}
                </span>
                <Badge
                  variant={
                    event.event_type === "LOGIN_OK"
                      ? "success"
                      : event.event_type === "LOGIN_FAIL"
                        ? "destructive"
                        : event.event_type === "SUDO_DANGER"
                          ? "warning"
                          : "secondary"
                  }
                >
                  {event.event_type.replace(/_/g, " ")}
                </Badge>
              </div>
              <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-400">
                {event.ip && (
                  <span className="flex items-center gap-1">
                    <Globe className="h-3 w-3" />
                    {event.ip}
                  </span>
                )}
                {event.geo_country && (
                  <span>
                    {countryFlag(event.geo_country)} {event.geo_country}
                    {event.geo_city ? `, ${event.geo_city}` : ""}
                  </span>
                )}
                {event.method && <span>via {event.method}</span>}
                <span className="flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  {formatRelativeTime(event.timestamp)}
                </span>
              </div>
              {event.command && (
                <div className="mt-1">
                  <code className="text-xs text-orange-300 bg-orange-950/30 px-1.5 py-0.5 rounded">
                    {event.command}
                  </code>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

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

// -------------------------------------------------------------------
// Tab: Banned IPs
// -------------------------------------------------------------------
function BannedIPsTab() {
  const { data: bannedIPs, isLoading } = useSWR<BannedIP[]>(
    "/banned-ips",
    swrFetcher,
    { refreshInterval: 60000 }
  );

  // Sort by banned_at desc
  const sorted = useMemo(() => {
    if (!bannedIPs) return [];
    return [...bannedIPs].sort(
      (a, b) => new Date(b.banned_at).getTime() - new Date(a.banned_at).getTime()
    );
  }, [bannedIPs]);

  if (isLoading) {
    return <Skeleton className="h-96 rounded-xl" />;
  }

  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-zinc-800">
              <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                IP Address
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                Country
              </th>
              <th className="hidden px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500 md:table-cell">
                City
              </th>
              <th className="hidden px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500 lg:table-cell">
                ISP
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                Jail
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                Banned At
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                Status
              </th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((ip) => (
              <tr
                key={`${ip.ip}-${ip.jail}-${ip.banned_at}`}
                className="border-b border-zinc-800/50"
              >
                <td className="px-4 py-3 font-mono text-sm text-zinc-200">
                  {ip.ip}
                </td>
                <td className="px-4 py-3 text-sm text-zinc-300">
                  {countryFlag(ip.country)} {ip.country}
                </td>
                <td className="hidden px-4 py-3 text-sm text-zinc-400 md:table-cell">
                  {ip.city || "-"}
                </td>
                <td className="hidden px-4 py-3 text-sm text-zinc-400 lg:table-cell">
                  <span className="max-w-[200px] truncate block">{ip.isp || "-"}</span>
                </td>
                <td className="px-4 py-3">
                  <Badge variant="outline">{ip.jail}</Badge>
                </td>
                <td className="px-4 py-3 text-xs text-zinc-400">
                  {formatDate(ip.banned_at)}
                </td>
                <td className="px-4 py-3">
                  <Badge variant={ip.is_active ? "destructive" : "secondary"}>
                    {ip.is_active ? "Active" : "Expired"}
                  </Badge>
                </td>
              </tr>
            ))}
            {sorted.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-12 text-center text-zinc-500">
                  No banned IPs found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

// -------------------------------------------------------------------
// Tab: Stats
// -------------------------------------------------------------------
function StatsTab() {
  const { data: stats, isLoading } = useSWR<LoginStats>(
    "/logins/stats",
    swrFetcher,
    { refreshInterval: 60000 }
  );

  if (isLoading || !stats) {
    return (
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Skeleton className="h-80 rounded-xl" />
        <Skeleton className="h-80 rounded-xl" />
        <Skeleton className="h-80 rounded-xl lg:col-span-2" />
      </div>
    );
  }

  // Pie chart data from by_type
  const successCount = stats.success || 0;
  const failedCount = stats.failed || 0;
  const pieData = [
    { name: "Success", value: successCount },
    { name: "Failed", value: failedCount },
  ];
  const PIE_COLORS = ["#22c55e", "#ef4444"];

  // Bar chart: by_type breakdown
  const barData = stats.by_type?.map((item) => ({
    type: item.type.replace(/_/g, " "),
    count: item.count,
  })) ?? [];

  return (
    <div className="space-y-4">
      {/* Summary row */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card>
          <CardContent className="p-4 text-center">
            <p className="text-3xl font-bold text-zinc-100">{stats.total}</p>
            <p className="text-xs text-zinc-400">Total Events</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4 text-center">
            <p className="text-3xl font-bold text-green-400">{stats.success}</p>
            <p className="text-xs text-zinc-400">Successful</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4 text-center">
            <p className="text-3xl font-bold text-red-400">{stats.failed}</p>
            <p className="text-xs text-zinc-400">Failed</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4 text-center">
            <p className="text-3xl font-bold text-blue-400">{stats.unique_ips}</p>
            <p className="text-xs text-zinc-400">Unique IPs</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* Pie chart: Success vs Failed */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Success vs Failed</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={60}
                    outerRadius={90}
                    paddingAngle={3}
                    dataKey="value"
                    label={({ name, percent }: // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    any) =>
                      `${name || ""} ${((percent ?? 0) * 100).toFixed(0)}%`
                    }
                  >
                    {pieData.map((_, i) => (
                      <Cell key={i} fill={PIE_COLORS[i]} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "#18181b",
                      border: "1px solid #27272a",
                      borderRadius: "8px",
                      color: "#e4e4e7",
                    }}
                  />
                  <Legend
                    formatter={(value) => (
                      <span className="text-zinc-300">{value}</span>
                    )}
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Bar chart: by event type */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Events by Type</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={barData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                  <XAxis
                    dataKey="type"
                    tick={{ fill: "#71717a", fontSize: 10 }}
                    angle={-20}
                    textAnchor="end"
                    height={50}
                  />
                  <YAxis tick={{ fill: "#71717a", fontSize: 11 }} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "#18181b",
                      border: "1px solid #27272a",
                      borderRadius: "8px",
                      color: "#e4e4e7",
                    }}
                  />
                  <Bar dataKey="count" fill="#3b82f6" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Top attacking IPs table */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Top Attacking IPs</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-zinc-800">
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-zinc-500">
                    #
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-zinc-500">
                    IP Address
                  </th>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-zinc-500">
                    Country
                  </th>
                  <th className="px-4 py-2 text-right text-xs font-medium uppercase text-zinc-500">
                    Attempts
                  </th>
                </tr>
              </thead>
              <tbody>
                {(stats.top_ips ?? []).slice(0, 10).map((item, i) => (
                  <tr
                    key={item.ip}
                    className="border-b border-zinc-800/50"
                  >
                    <td className="px-4 py-2 text-sm text-zinc-500">
                      {i + 1}
                    </td>
                    <td className="px-4 py-2 font-mono text-sm text-zinc-200">
                      {item.ip}
                    </td>
                    <td className="px-4 py-2 text-sm text-zinc-300">
                      {countryFlag(item.country)} {item.country}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <Badge variant="destructive">{item.count}</Badge>
                    </td>
                  </tr>
                ))}
                {(!stats.top_ips || stats.top_ips.length === 0) && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-zinc-500">
                      No data available
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

// -------------------------------------------------------------------
// Security page with tabs
// -------------------------------------------------------------------
export default function SecurityPage() {
  const [activeTab, setActiveTab] = useState<"timeline" | "banned" | "stats">(
    "timeline"
  );

  const tabs = [
    { id: "timeline" as const, label: "Login Timeline", icon: Activity },
    { id: "banned" as const, label: "Banned IPs", icon: Ban },
    { id: "stats" as const, label: "Stats", icon: BarChart3 },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-zinc-100">Security</h1>

      {/* Tab bar */}
      <div className="flex gap-1 rounded-lg border border-zinc-800 bg-zinc-900/50 p-1">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              "flex items-center gap-2 rounded-md px-4 py-2 text-sm font-medium transition-colors",
              activeTab === tab.id
                ? "bg-zinc-800 text-zinc-100"
                : "text-zinc-400 hover:text-zinc-200"
            )}
          >
            <tab.icon className="h-4 w-4" />
            <span className="hidden sm:inline">{tab.label}</span>
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === "timeline" && <LoginTimeline />}
      {activeTab === "banned" && <BannedIPsTab />}
      {activeTab === "stats" && <StatsTab />}
    </div>
  );
}
