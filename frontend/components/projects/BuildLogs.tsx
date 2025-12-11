"use client";

import { useEffect, useRef, useState } from "react";
import { BuildLog } from "@/lib/api/builds";
import { buildsApi } from "@/lib/api/builds";
import { BuildLogsWebSocket } from "@/lib/websocket/buildLogs";
import { buildLogsCache } from "@/lib/cache/buildLogsCache";
import { Loader2 } from "lucide-react";

interface BuildLogsProps {
  buildId: string;
  projectId: string;
  token: string;
  buildStatus?: string; // Build status to determine if we should stream
  wsUrl?: string;
}

export function BuildLogs({
  buildId,
  projectId,
  token,
  buildStatus,
  wsUrl,
}: BuildLogsProps) {
  // Get WebSocket URL from env variable or use provided wsUrl
  // Default to API Gateway WebSocket proxy (ws://localhost:8000/ws)
  const defaultWsUrl =
    typeof window !== "undefined"
      ? process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8000"
      : "ws://localhost:8000";
  const finalWsUrl = wsUrl || defaultWsUrl;
  const [logs, setLogs] = useState<BuildLog[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isStreaming, setIsStreaming] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<BuildLogsWebSocket | null>(null);
  const wsLogCounterRef = useRef<number>(0);
  const maxHistoricalIdRef = useRef<number>(0);

  const lastBuildIdRef = useRef<string | null>(null);

  // Fetch all historical logs
  useEffect(() => {
    if (!buildId || !token) {
      setIsLoading(false);
      return;
    }

    // Only clear if buildId actually changed
    const buildIdChanged = lastBuildIdRef.current !== buildId;
    if (buildIdChanged) {
      console.log(`[BuildLogs] BuildId changed from ${lastBuildIdRef.current} to ${buildId}`);
      setLogs([]);
      setError(null);
      setIsStreaming(false);
      wsLogCounterRef.current = 0;
      maxHistoricalIdRef.current = 0;
      lastBuildIdRef.current = buildId;
    }

    // Check cache first (only if buildId hasn't changed)
    // But always fetch from API to ensure we have latest data
    // Cache is mainly for preventing re-fetch when component remounts for same build
    // if (!buildIdChanged) {
    //   const cachedLogs = buildLogsCache.get(buildId);
    //   if (cachedLogs && cachedLogs.length > 0) {
    //     console.log(`[BuildLogs] Using cached logs for build ${buildId}: ${cachedLogs.length} logs`);
    //     setLogs([...cachedLogs]); // Create new array to trigger re-render
    //     maxHistoricalIdRef.current = Math.max(...cachedLogs.map(l => l.id));
    //     setIsLoading(false);
    //     return;
    //   }
    // }

    const fetchAllLogs = async () => {
      const allLogs: BuildLog[] = [];
      try {
        setIsLoading(true);
        let afterId: number | undefined = undefined;
        let hasMore = true;
        const limit = 1000; // Max limit per request
        let pageCount = 0;

        // Fetch all logs using pagination
        while (hasMore && pageCount < 100) { // Safety limit to prevent infinite loops
          pageCount++;
          const response = await buildsApi.getBuildLogs(
            token,
            buildId,
            limit,
            afterId
          );
          
          console.log(`[BuildLogs] Fetch page ${pageCount} for build ${buildId}:`, {
            response,
            logsCount: response?.logs?.length || 0,
            hasMore: response?.has_more,
            afterId,
          });

          // Handle response - check for both response.logs and response structure
          const logs = response?.logs || [];
          if (logs.length > 0) {
            allLogs.push(...logs);
            // Get the last log ID for next pagination
            const lastLog = logs[logs.length - 1];
            afterId = lastLog.id;
            hasMore = response?.has_more === true;
          } else {
            hasMore = false;
          }
        }

        console.log(`[BuildLogs] Fetched ${allLogs.length} total logs for build ${buildId}`);

        // Sort logs by ID to ensure correct order (ascending)
        allLogs.sort((a, b) => a.id - b.id);
        // Store max ID for WebSocket logs to ensure they come after historical logs
        maxHistoricalIdRef.current = allLogs.length > 0 
          ? Math.max(...allLogs.map(l => l.id)) 
          : 0;
        
        // Cache the logs (module-level cache persists across unmounts)
        if (buildId) {
          buildLogsCache.set(buildId, allLogs);
        }
        
        console.log(`[BuildLogs] Setting ${allLogs.length} logs, maxHistoricalId: ${maxHistoricalIdRef.current}`);
        // Use functional update to ensure state is set correctly
        setLogs(() => [...allLogs]); // Create new array to trigger re-render
      } catch (err: any) {
        console.error("[BuildLogs] Failed to fetch build logs:", err);
        // Don't set error as fatal - show warning but still try to display any logs we have
        setError(err.message || "Failed to fetch logs");
        // Set logs even if there was an error - might have partial data
        if (allLogs.length > 0) {
          allLogs.sort((a, b) => a.id - b.id);
          if (buildId) {
            buildLogsCache.set(buildId, allLogs);
          }
          setLogs(() => [...allLogs]);
        } else {
          setLogs([]);
        }
      } finally {
        setIsLoading(false);
      }
    };

    fetchAllLogs();
  }, [buildId, token]);

  // Check if build is still running (non-terminal states)
  const isBuildRunning = (status?: string): boolean => {
    if (!status) return false;
    const terminalStates = ["success", "failed", "deploy_failed"];
    return !terminalStates.includes(status.toLowerCase());
  };

  // Setup WebSocket for real-time logs (only for running builds)
  useEffect(() => {
    // Only stream if build is running
    if (!isBuildRunning(buildStatus)) {
      setIsStreaming(false);
      return;
    }

    // Wait for historical logs to load before starting WebSocket
    if (isLoading) {
      return;
    }

    let mounted = true;
    const ws = new BuildLogsWebSocket(finalWsUrl, buildId, projectId);
    wsRef.current = ws;

    ws
      .connect()
      .then(() => {
        if (!mounted) {
          ws.disconnect();
          return;
        }
        setIsStreaming(true);
        const channel = `build_logs:${projectId}:${buildId}`;
        ws.subscribe(channel, (message) => {
          if (!mounted) return;
          // Only add logs if build is still running
          if (!isBuildRunning(buildStatus)) {
            return;
          }
          // Add new log line with unique ID
          // Use timestamp + counter to ensure uniqueness
          wsLogCounterRef.current++;
          // Ensure WebSocket logs have IDs greater than historical logs
          const uniqueId = Math.max(
            maxHistoricalIdRef.current + wsLogCounterRef.current,
            Date.now() * 10000 + wsLogCounterRef.current
          );
          const newLog: BuildLog = {
            id: uniqueId,
            build_id: buildId,
            timestamp: new Date().toISOString(),
            log_line: message.message,
          };
          setLogs((prev) => {
            // Ensure logs are sorted by ID (ascending)
            const updated = [...prev, newLog];
            const sorted = updated.sort((a, b) => a.id - b.id);
            // Update cache (module-level cache persists across unmounts)
            if (buildId) {
              buildLogsCache.set(buildId, sorted);
            }
            return sorted;
          });
        });
      })
      .catch((err) => {
        if (!mounted) return;
        console.error("Failed to connect WebSocket:", err);
        // Don't show error for completed builds
        if (isBuildRunning(buildStatus)) {
          setError("Failed to connect to real-time logs");
        }
      });

    return () => {
      mounted = false;
      wsLogCounterRef.current = 0; // Reset counter on unmount
      if (wsRef.current) {
        wsRef.current.disconnect();
        wsRef.current = null;
      }
    };
  }, [buildId, projectId, finalWsUrl, buildStatus, isLoading]);

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (logs.length > 0) {
      logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs]);

  // Debug: log current state
  useEffect(() => {
    console.log(`[BuildLogs] Render state for build ${buildId}:`, {
      logsCount: logs.length,
      isLoading,
      error,
      isStreaming,
      buildStatus,
      lastBuildId: lastBuildIdRef.current,
    });
  }, [logs.length, isLoading, error, isStreaming, buildId, buildStatus]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="h-6 w-6 animate-spin text-surface-400" />
        <span className="ml-2 text-sm text-surface-400">Loading logs...</span>
      </div>
    );
  }

  return (
    <div className="relative">
      {error && (
        <div className="mb-2 rounded-lg border border-yellow-500/20 bg-yellow-500/10 p-2 text-xs text-yellow-500">
          Warning: {error}
        </div>
      )}
      {isStreaming && (
        <div className="mb-2 flex items-center gap-2 text-xs text-surface-400">
          <div className="h-2 w-2 animate-pulse rounded-full bg-green-500" />
          <span>Streaming logs...</span>
        </div>
      )}
      <div className="max-h-96 overflow-y-auto rounded-lg border border-surface-800 bg-surface-950 p-4 font-mono text-xs">
        {logs.length === 0 && !isLoading ? (
          <div className="py-8 text-center text-sm text-surface-500">
            No logs available
          </div>
        ) : (
          <div className="space-y-1">
            {logs.map((log, index) => {
              // Ensure log has required fields
              if (!log || !log.id || !log.timestamp || !log.log_line) {
                console.warn(`[BuildLogs] Invalid log at index ${index}:`, log);
                return null;
              }
              return (
                <div
                  key={`${log.id}-${index}-${log.timestamp}`}
                  className="flex items-start gap-2 text-surface-300"
                >
                  <span className="shrink-0 text-surface-600">
                    {new Date(log.timestamp).toLocaleTimeString("vi-VN", {
                      hour: "2-digit",
                      minute: "2-digit",
                      second: "2-digit",
                    })}
                  </span>
                  <span className="flex-1 whitespace-pre-wrap break-words">
                    {log.log_line}
                  </span>
                </div>
              );
            })}
            <div ref={logsEndRef} />
          </div>
        )}
      </div>
    </div>
  );
}

