"use client";

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  useCallback,
} from "react";
import { getToken } from "@/lib/auth";
import type {
  SystemMetrics,
  Container,
  LoginEvent,
  AlertEvent,
  PortStatus,
  WSMessage,
} from "@/lib/types";

const WS_BASE_URL =
  process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8080";

const MAX_RECENT_ITEMS = 50;
const INITIAL_RECONNECT_DELAY = 3000;
const MAX_RECONNECT_DELAY = 30000;

interface WSContextValue {
  connected: boolean;
  lastMetrics: SystemMetrics | null;
  lastContainers: Container[];
  recentLogins: LoginEvent[];
  recentAlerts: AlertEvent[];
  lastPorts: PortStatus[];
  bannedIPsVersion: number;
}

const WSContext = createContext<WSContextValue | undefined>(undefined);

export function WebSocketProvider({ children }: { children: React.ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [lastMetrics, setLastMetrics] = useState<SystemMetrics | null>(null);
  const [lastContainers, setLastContainers] = useState<Container[]>([]);
  const [recentLogins, setRecentLogins] = useState<LoginEvent[]>([]);
  const [recentAlerts, setRecentAlerts] = useState<AlertEvent[]>([]);
  const [lastPorts, setLastPorts] = useState<PortStatus[]>([]);
  const [bannedIPsVersion, setBannedIPsVersion] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectDelayRef = useRef(INITIAL_RECONNECT_DELAY);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);

  const handleMessage = useCallback((event: MessageEvent) => {
    try {
      const msg: WSMessage = JSON.parse(event.data);

      switch (msg.type) {
        case "system_metrics":
          setLastMetrics(msg.data as SystemMetrics);
          break;

        case "containers": {
          // Backend sends {containers: [...], metrics: [...]}
          const cdata = msg.data as { containers?: Container[]; metrics?: unknown } | Container[];
          if (Array.isArray(cdata)) {
            setLastContainers(cdata);
          } else if (cdata && cdata.containers) {
            setLastContainers(cdata.containers);
          }
          break;
        }

        case "login_event":
          setRecentLogins((prev) => {
            const updated = [msg.data as LoginEvent, ...prev];
            return updated.slice(0, MAX_RECENT_ITEMS);
          });
          break;

        case "alert":
          setRecentAlerts((prev) => {
            const updated = [msg.data as AlertEvent, ...prev];
            return updated.slice(0, MAX_RECENT_ITEMS);
          });
          break;

        case "monitor_event":
          // Monitor events are treated as alerts
          setRecentAlerts((prev) => {
            const updated = [msg.data as AlertEvent, ...prev];
            return updated.slice(0, MAX_RECENT_ITEMS);
          });
          break;

        case "ports":
          setLastPorts(msg.data as PortStatus[]);
          break;

        case "banned_ip_update":
          setBannedIPsVersion((v) => v + 1);
          break;
      }
    } catch (err) {
      console.error("[WS] Failed to parse message:", err);
    }
  }, []);

  const connect = useCallback(() => {
    const token = getToken();
    if (!token) return;

    // Clean up any existing connection
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const url = `${WS_BASE_URL}/api/ws?token=${encodeURIComponent(token)}`;

    try {
      const ws = new WebSocket(url);

      ws.onopen = () => {
        if (!mountedRef.current) return;
        console.log("[WS] Connected");
        setConnected(true);
        reconnectDelayRef.current = INITIAL_RECONNECT_DELAY;
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;
        handleMessage(event);
      };

      ws.onclose = (event) => {
        if (!mountedRef.current) return;
        console.log("[WS] Disconnected:", event.code, event.reason);
        setConnected(false);
        wsRef.current = null;

        // Auto-reconnect with exponential backoff
        const delay = reconnectDelayRef.current;
        console.log(`[WS] Reconnecting in ${delay / 1000}s...`);
        reconnectTimerRef.current = setTimeout(() => {
          if (mountedRef.current) {
            reconnectDelayRef.current = Math.min(
              delay * 2,
              MAX_RECONNECT_DELAY
            );
            connect();
          }
        }, delay);
      };

      ws.onerror = (error) => {
        console.error("[WS] Error:", error);
      };

      wsRef.current = ws;
    } catch {
      // Connection failed, schedule reconnect
      const delay = reconnectDelayRef.current;
      reconnectTimerRef.current = setTimeout(() => {
        if (mountedRef.current) {
          reconnectDelayRef.current = Math.min(
            delay * 2,
            MAX_RECONNECT_DELAY
          );
          connect();
        }
      }, delay);
    }
  }, [handleMessage]);

  useEffect(() => {
    mountedRef.current = true;
    connect();

    return () => {
      mountedRef.current = false;

      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }

      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect]);

  return (
    <WSContext.Provider
      value={{
        connected,
        lastMetrics,
        lastContainers,
        recentLogins,
        recentAlerts,
        lastPorts,
        bannedIPsVersion,
      }}
    >
      {children}
    </WSContext.Provider>
  );
}

export function useWS(): WSContextValue {
  const context = useContext(WSContext);
  if (context === undefined) {
    throw new Error("useWS must be used within a WebSocketProvider");
  }
  return context;
}
