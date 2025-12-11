type LogMessage = {
  type: string;
  message: string;
  project_id?: string;
  build_id?: string;
};

type SubscribeMessage = {
  action: "subscribe" | "unsubscribe";
  channel: string;
};

export class BuildLogsWebSocket {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private isManualClose = false;
  private connectionState = false;
  private listeners: Map<string, Set<(message: LogMessage) => void>> = new Map();
  private reconnectTimer: NodeJS.Timeout | null = null;

  constructor(
    private wsUrl: string,
    private buildId: string,
    private projectId: string
  ) {}

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        // If already connected, resolve immediately
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
          resolve();
          return;
        }

        // If connection is in progress, wait for it
        if (this.ws && this.ws.readyState === WebSocket.CONNECTING) {
          const checkConnection = setInterval(() => {
            if (this.isManualClose) {
              clearInterval(checkConnection);
              reject(new Error("Connection cancelled"));
              return;
            }
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
              clearInterval(checkConnection);
              resolve();
            } else if (this.ws && this.ws.readyState === WebSocket.CLOSED) {
              clearInterval(checkConnection);
              // Retry connection
              this.connect().then(resolve).catch(reject);
            }
          }, 100);
          return;
        }

        const channel = `build_logs:${this.projectId}:${this.buildId}`;
        const url = `${this.wsUrl}/ws?subscribe=${channel}`;

        this.ws = new WebSocket(url);
        let isResolved = false;
        let isRejected = false;
        let connectionCheckInterval: NodeJS.Timeout | null = null;

        // Cleanup function
        const cleanup = () => {
          if (connectionCheckInterval) {
            clearInterval(connectionCheckInterval);
            connectionCheckInterval = null;
          }
        };

        this.ws.onopen = () => {
          cleanup();
          if (this.isManualClose) {
            // Connection was cancelled, close it
            if (this.ws) {
              this.ws.close();
            }
            return;
          }
          this.reconnectAttempts = 0;
          this.connectionState = true;
          isResolved = true;
          if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
          }
          resolve();
        };

        this.ws.onmessage = (event) => {
          if (this.isManualClose) return;
          try {
            // Handle newline-delimited JSON (multiple JSON objects in one message)
            const text = event.data;
            const lines = text.split('\n').filter((line: string) => line.trim());
            
            for (const line of lines) {
              if (!line.trim()) continue;
              
              try {
                const data = JSON.parse(line);

                // Handle ACK messages
                if (data.type === "ack" && data.status === "subscribed") {
                  continue;
                }

                // Handle log messages
                const logMessage: LogMessage = {
                  type: data.type || "log",
                  message: data.message || data.line || line,
                  project_id: data.project_id || this.projectId,
                  build_id: data.build_id || this.buildId,
                };

                // Notify all listeners for this channel
                const channel = `build_logs:${this.projectId}:${this.buildId}`;
                const channelListeners = this.listeners.get(channel);
                if (channelListeners) {
                  channelListeners.forEach((listener) => listener(logMessage));
                }
              } catch (parseErr) {
                // If single line parse fails, try to handle as plain text log
                if (lines.length === 1) {
                  const logMessage: LogMessage = {
                    type: "log",
                    message: line,
                    project_id: this.projectId,
                    build_id: this.buildId,
                  };
                  const channel = `build_logs:${this.projectId}:${this.buildId}`;
                  const channelListeners = this.listeners.get(channel);
                  if (channelListeners) {
                    channelListeners.forEach((listener) => listener(logMessage));
                  }
                } else {
                  console.warn("Failed to parse WebSocket message line:", line, parseErr);
                }
              }
            }
          } catch (err) {
            console.error("Failed to process WebSocket message:", err, "Data:", event.data);
          }
        };

        this.ws.onerror = (error) => {
          cleanup();
          if (this.isManualClose) return;
          console.error("WebSocket error:", error);
          // Only reject if connection hasn't been established yet
          if (!isResolved && !isRejected && this.reconnectAttempts === 0) {
            isRejected = true;
            reject(error);
          }
        };

        this.ws.onclose = () => {
          cleanup();
          this.connectionState = false;
          // Only attempt reconnect if not manually closed and not resolved (connection was never established)
          if (!this.isManualClose && !isResolved && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            this.reconnectTimer = setTimeout(() => {
              if (!this.isManualClose) {
                console.log(`Reconnecting... (attempt ${this.reconnectAttempts})`);
                this.connect().catch(console.error);
              }
            }, this.reconnectDelay * this.reconnectAttempts);
          }
        };
      } catch (err) {
        reject(err);
      }
    });
  }

  subscribe(channel: string, callback: (message: LogMessage) => void): () => void {
    if (!this.listeners.has(channel)) {
      this.listeners.set(channel, new Set());
    }
    this.listeners.get(channel)!.add(callback);

    // Send subscribe message if WebSocket is open
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const msg: SubscribeMessage = {
        action: "subscribe",
        channel,
      };
      this.ws.send(JSON.stringify(msg));
    }

    // Return unsubscribe function
    return () => {
      const channelListeners = this.listeners.get(channel);
      if (channelListeners) {
        channelListeners.delete(callback);
      }

      // Send unsubscribe message if WebSocket is open
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        const msg: SubscribeMessage = {
          action: "unsubscribe",
          channel,
        };
        this.ws.send(JSON.stringify(msg));
      }
    };
  }

  disconnect(): void {
    this.isManualClose = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      const readyState = this.ws.readyState;
      // Only close if connection is open (not if still connecting or already closed)
      // This prevents "WebSocket is closed before the connection is established" error
      if (readyState === WebSocket.OPEN) {
        try {
          this.ws.close();
        } catch (err) {
          // Ignore errors when closing
        }
        this.ws = null;
      } else if (readyState === WebSocket.CONNECTING) {
        // If still connecting, set up a handler to close it once it opens
        // Don't close it while connecting as that causes the error
        const ws = this.ws;
        const originalOnOpen = ws.onopen;
        ws.onopen = (event) => {
          // Close immediately after opening if we're supposed to disconnect
          if (this.isManualClose && ws) {
            try {
              ws.close();
            } catch (err) {
              // Ignore errors
            }
            this.ws = null;
          } else if (originalOnOpen) {
            originalOnOpen.call(ws, event);
          }
        };
        // Also set up a timeout to clean up if connection takes too long
        setTimeout(() => {
          if (this.ws === ws && ws.readyState === WebSocket.CONNECTING) {
            // Connection is taking too long, just null it out
            // The WebSocket will eventually close on its own
            this.ws = null;
          }
        }, 5000);
      } else {
        // Already closed or closing, just null it out
        this.ws = null;
      }
    }
    this.connectionState = false;
    this.listeners.clear();
  }

  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }
}

