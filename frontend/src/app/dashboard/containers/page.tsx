"use client";

import { useState, useMemo } from "react";
import useSWR from "swr";
import { swrFetcher } from "@/lib/api";
import { useWS } from "@/providers/websocket-provider";
import { cn, formatRelativeTime } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import type { Container, SystemMetrics } from "@/lib/types";
import {
  Container as ContainerIcon,
  Search,
  Play,
  Square,
  AlertTriangle,
  X,
  Cpu,
  MemoryStick,
  Clock,
  Image as ImageIcon,
  Activity,
} from "lucide-react";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";

// -------------------------------------------------------------------
// Container detail modal
// -------------------------------------------------------------------
function ContainerDetailModal({
  container,
  onClose,
}: {
  container: Container;
  onClose: () => void;
}) {
  const { data: metricsData } = useSWR<SystemMetrics[]>(
    `/containers/${container.id}/metrics?range=24h`,
    swrFetcher
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-black/60" onClick={onClose} />
      <div className="relative z-10 w-full max-w-2xl rounded-xl border border-zinc-800 bg-zinc-900 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-zinc-800 p-4">
          <div className="flex items-center gap-3">
            <div
              className={cn(
                "h-3 w-3 rounded-full",
                container.status === "running"
                  ? "bg-green-500"
                  : container.status === "exited"
                    ? "bg-red-500"
                    : "bg-yellow-500"
              )}
            />
            <h2 className="text-lg font-bold text-zinc-100">
              {container.name}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="text-zinc-400 hover:text-zinc-100"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="space-y-4 p-4">
          {/* Info grid */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <p className="text-xs text-zinc-500">Image</p>
              <p className="flex items-center gap-1 text-sm text-zinc-200">
                <ImageIcon className="h-3.5 w-3.5" />
                {container.image}
              </p>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-zinc-500">Status</p>
              <Badge
                variant={
                  container.status === "running"
                    ? "success"
                    : container.status === "exited"
                      ? "destructive"
                      : "warning"
                }
              >
                {container.status}
              </Badge>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-zinc-500">Health</p>
              <Badge
                variant={
                  container.health === "healthy"
                    ? "success"
                    : container.health === "unhealthy"
                      ? "destructive"
                      : "secondary"
                }
              >
                {container.health || "N/A"}
              </Badge>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-zinc-500">Started</p>
              <p className="text-sm text-zinc-200">
                {container.started_at
                  ? formatRelativeTime(container.started_at)
                  : "N/A"}
              </p>
            </div>
          </div>

          {/* Resource usage */}
          <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg border border-zinc-800 bg-zinc-950 p-3">
              <div className="flex items-center gap-2">
                <Cpu className="h-4 w-4 text-blue-400" />
                <span className="text-xs text-zinc-400">CPU</span>
              </div>
              <p className="mt-1 text-xl font-bold text-zinc-100">
                {container.cpu_percent.toFixed(1)}%
              </p>
            </div>
            <div className="rounded-lg border border-zinc-800 bg-zinc-950 p-3">
              <div className="flex items-center gap-2">
                <MemoryStick className="h-4 w-4 text-purple-400" />
                <span className="text-xs text-zinc-400">Memory</span>
              </div>
              <p className="mt-1 text-xl font-bold text-zinc-100">
                {container.mem_percent.toFixed(1)}%
              </p>
              <p className="text-xs text-zinc-500">
                {container.mem_usage_mb.toFixed(0)} / {container.mem_limit_mb.toFixed(0)} MB
              </p>
            </div>
          </div>

          {/* Metrics chart */}
          {metricsData && metricsData.length > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-medium text-zinc-300">
                CPU Usage (24h)
              </h3>
              <div className="h-48 w-full">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={metricsData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                    <XAxis
                      dataKey="timestamp"
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => {
                        try {
                          return new Date(v).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                          });
                        } catch {
                          return v;
                        }
                      }}
                    />
                    <YAxis
                      domain={[0, 100]}
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => `${v}%`}
                    />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "#18181b",
                        border: "1px solid #27272a",
                        borderRadius: "8px",
                        color: "#e4e4e7",
                      }}
                    />
                    <Area
                      type="monotone"
                      dataKey="cpu_percent"
                      stroke="#3b82f6"
                      fill="#3b82f6"
                      fillOpacity={0.15}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// -------------------------------------------------------------------
// Containers page
// -------------------------------------------------------------------
export default function ContainersPage() {
  const ws = useWS();
  const { data: apiResp, isLoading } = useSWR<{ data: Container[]; total: number }>(
    "/containers",
    swrFetcher,
    { refreshInterval: 15000 }
  );
  const apiContainers = apiResp?.data;

  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"all" | "running" | "exited" | "unhealthy">("all");
  const [selectedContainer, setSelectedContainer] = useState<Container | null>(null);

  // WebSocket data takes priority
  const containers = ws.lastContainers.length > 0 ? ws.lastContainers : apiContainers ?? [];

  // Summary counts
  const total = containers.length;
  const isRunning = (s: string) => s === "running" || s.startsWith("Up");
  const isExited = (s: string) => s === "exited" || s.startsWith("Exited");
  const running = containers.filter((c) => isRunning(c.status)).length;
  const stopped = containers.filter((c) => isExited(c.status)).length;
  const unhealthy = containers.filter((c) => c.health === "unhealthy").length;

  // Filter and search
  const filtered = useMemo(() => {
    let result = containers;

    if (filter === "running") result = result.filter((c) => isRunning(c.status));
    else if (filter === "exited") result = result.filter((c) => isExited(c.status));
    else if (filter === "unhealthy") result = result.filter((c) => c.health === "unhealthy");

    if (search.trim()) {
      const term = search.toLowerCase();
      result = result.filter(
        (c) =>
          c.name.toLowerCase().includes(term) ||
          c.image.toLowerCase().includes(term)
      );
    }

    return result;
  }, [containers, filter, search]);

  if (isLoading && containers.length === 0) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">Containers</h1>
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-xl" />
          ))}
        </div>
        <Skeleton className="h-96 rounded-xl" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-zinc-100">Containers</h1>

      {/* Summary cards */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-zinc-800">
              <ContainerIcon className="h-5 w-5 text-zinc-300" />
            </div>
            <div>
              <p className="text-2xl font-bold text-zinc-100">{total}</p>
              <p className="text-xs text-zinc-400">Total</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-green-600/20">
              <Play className="h-5 w-5 text-green-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-green-400">{running}</p>
              <p className="text-xs text-zinc-400">Running</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-red-600/20">
              <Square className="h-5 w-5 text-red-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-red-400">{stopped}</p>
              <p className="text-xs text-zinc-400">Stopped</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-yellow-600/20">
              <AlertTriangle className="h-5 w-5 text-yellow-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-yellow-400">{unhealthy}</p>
              <p className="text-xs text-zinc-400">Unhealthy</p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Search and filter */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
          <Input
            placeholder="Search containers..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <div className="flex gap-2">
          {(["all", "running", "exited", "unhealthy"] as const).map((f) => (
            <Button
              key={f}
              variant={filter === f ? "secondary" : "ghost"}
              size="sm"
              onClick={() => setFilter(f)}
              className="capitalize"
            >
              {f === "exited" ? "Stopped" : f}
            </Button>
          ))}
        </div>
      </div>

      {/* Container table */}
      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-zinc-800">
                <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                  Name
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500">
                  Health
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium uppercase text-zinc-500">
                  CPU %
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium uppercase text-zinc-500">
                  MEM %
                </th>
                <th className="hidden px-4 py-3 text-left text-xs font-medium uppercase text-zinc-500 lg:table-cell">
                  Image
                </th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((c) => (
                <tr
                  key={c.id}
                  onClick={() => setSelectedContainer(c)}
                  className="cursor-pointer border-b border-zinc-800/50 transition-colors hover:bg-zinc-800/30"
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div
                        className={cn(
                          "h-2 w-2 rounded-full",
                          c.status === "running"
                            ? "bg-green-500"
                            : c.status === "exited"
                              ? "bg-red-500"
                              : "bg-yellow-500"
                        )}
                      />
                      <span className="font-medium text-zinc-200">
                        {c.name}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <Badge
                      variant={
                        c.status === "running"
                          ? "success"
                          : c.status === "exited"
                            ? "destructive"
                            : "warning"
                      }
                    >
                      {c.status}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">
                    <Badge
                      variant={
                        c.health === "healthy"
                          ? "success"
                          : c.health === "unhealthy"
                            ? "destructive"
                            : "secondary"
                      }
                    >
                      {c.health || "none"}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-sm text-zinc-300">
                    {c.cpu_percent.toFixed(1)}%
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-sm text-zinc-300">
                    {c.mem_percent.toFixed(1)}%
                  </td>
                  <td className="hidden px-4 py-3 text-sm text-zinc-400 lg:table-cell">
                    <span className="max-w-xs truncate block">{c.image}</span>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-zinc-500">
                    No containers found
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </Card>

      {/* Detail modal */}
      {selectedContainer && (
        <ContainerDetailModal
          container={selectedContainer}
          onClose={() => setSelectedContainer(null)}
        />
      )}
    </div>
  );
}
