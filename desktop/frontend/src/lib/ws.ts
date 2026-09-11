// WebSocket 客户端 — 单例，JSON-RPC 风格协议，支持请求-响应与流式推送。

type PendingRequest = {
  resolve: (value: unknown) => void;
  reject: (reason: Error) => void;
};

type StreamCallbacks<M = Record<string, unknown>, D = Record<string, unknown>> = {
  onMeta?: (data: M) => void;
  /** token 数据：chat.send 为 string，dispatch.run 为 {expert_id, content} */
  onToken?: (data: unknown) => void;
  onDone?: (data: D) => void;
  onError?: (error: string) => void;
  /** 自定义事件类型（如 expert_start, expert_done）。 */
  onEvent?: (type: string, data: unknown) => void;
};

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

type StateListener = (state: ConnectionState) => void;

class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private reqId = 0;
  private pending = new Map<string, PendingRequest>();
  private streamHandlers = new Map<string, StreamCallbacks>();
  private state: ConnectionState = 'disconnected';
  private listeners = new Set<StateListener>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectAttempts = 0;
  private maxReconnectDelay = 30000;
  private intentionalClose = false;

  constructor(url: string) {
    this.url = url;
  }

  /** 获取当前连接状态。 */
  getState(): ConnectionState {
    return this.state;
  }

  /** 注册连接状态变化监听器。 */
  onStateChange(fn: StateListener): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  private setState(s: ConnectionState) {
    this.state = s;
    this.listeners.forEach((fn) => fn(s));
  }

  /** 建立 WebSocket 连接。 */
  connect(): Promise<void> {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) return Promise.resolve();

    return new Promise((resolve, reject) => {
      this.intentionalClose = false;
      this.setState('connecting');

      try {
        this.ws = new WebSocket(this.url);
      } catch (e) {
        this.setState('disconnected');
        reject(e);
        return;
      }

      this.ws.onopen = () => {
        this.reconnectAttempts = 0;
        this.setState('connected');
        this.startHeartbeat();
        resolve();
      };

      this.ws.onclose = () => {
        this.stopHeartbeat();
        if (!this.intentionalClose) {
          this.setState('reconnecting');
          this.scheduleReconnect();
        } else {
          this.setState('disconnected');
        }
        // 拒绝所有等待中的请求
        this.pending.forEach((p) => p.reject(new Error('connection closed')));
        this.pending.clear();
      };

      this.ws.onerror = () => {
        // onclose 会紧随其后触发
      };

      this.ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          this.handleMessage(msg);
        } catch {
          // 忽略无法解析的消息
        }
      };
    });
  }

  /** 主动断开连接。 */
  disconnect() {
    this.intentionalClose = true;
    this.stopHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.setState('disconnected');
  }

  /** 请求-响应模式调用。 */
  call<T = unknown>(method: string, params?: Record<string, unknown>): Promise<T> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.log('[WS] call rejected — not connected:', method);
      return Promise.reject(new Error('not connected'));
    }

    const id = String(++this.reqId);
    console.log('[WS] call -> id=%s method=%s params=%o', id, method, params);
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (value: unknown) => void, reject });

      this.ws!.send(JSON.stringify({ id, method, params: params || {} }));

      // 30 秒超时
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          console.log('[WS] call TIMEOUT id=%s method=%s', id, method);
          reject(new Error(`request timeout: ${method}`));
        }
      }, 30000);
    });
  }

  /** 流式调用（仅 chat.send）。返回 cancel 函数。 */
  stream<M = Record<string, unknown>, D = Record<string, unknown>>(
    method: string,
    params: Record<string, unknown>,
    callbacks: StreamCallbacks<M, D>
  ): () => void {
    console.log('[WS] stream -> method=%s params=%o wsReadyState=%d', method, params, this.ws?.readyState);

    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.log('[WS] stream REJECTED — not connected, ws=%s readyState=%d', !!this.ws, this.ws?.readyState ?? -1);
      callbacks.onError?.('not connected');
      return () => {};
    }

    const id = String(++this.reqId);
    console.log('[WS] stream SEND id=%s method=%s', id, method);
    this.streamHandlers.set(id, callbacks as StreamCallbacks);

    this.ws.send(JSON.stringify({ id, method, params }));

    return () => {
      console.log('[WS] stream CANCEL id=%s', id);
      this.streamHandlers.delete(id);
    };
  }

  /** 取消流式请求。 */
  cancelStream(id: string) {
    this.streamHandlers.delete(id);
  }

  private handleMessage(msg: Record<string, unknown>) {
    const id = msg.id as string;
    const type = msg.type as string | undefined;

    // 流式帧（有 type 字段）
    if (type) {
      const handlers = this.streamHandlers.get(id);
      console.log('[WS] stream frame id=%s type=%s hasHandler=%s data=%o', id, type, !!handlers, msg.data);
      if (!handlers) return;

      switch (type) {
        case 'meta':
          handlers.onMeta?.(msg.data as Record<string, unknown>);
          break;
        case 'token':
          handlers.onToken?.(msg.data);
          break;
        case 'done':
          handlers.onDone?.(msg.data as Record<string, unknown>);
          this.streamHandlers.delete(id);
          break;
        case 'error':
          console.log('[WS] stream ERROR routing to onError callback');
          handlers.onError?.(typeof msg.data === 'string' ? msg.data : (msg.data as Record<string, unknown>)?.message as string || 'stream error');
          this.streamHandlers.delete(id);
          break;
        default:
          // 转发自定义事件类型（如 expert_start, expert_done）
          handlers.onEvent?.(type, msg.data);
      }
      return;
    }

    // 普通响应（有 id + result/error）—— 同时检查 pending 和 streamHandlers
    if (id) {
      // 先检查是否为流式请求的错误响应（chat.send 等流式方法在 prepend 时返回普通 error 帧）
      const streamHandlers = this.streamHandlers.get(id);
      if (streamHandlers && msg.error) {
        console.log('[WS] stream RPC ERROR id=%s error=%o — routing to stream onError', id, msg.error);
        const err = msg.error as { code: number; message: string };
        streamHandlers.onError?.(err.message || `RPC error ${err.code}`);
        this.streamHandlers.delete(id);
        return;
      }

      // 检查普通 call 响应
      if (this.pending.has(id)) {
        const { resolve, reject } = this.pending.get(id)!;
        this.pending.delete(id);

        if (msg.error) {
          const err = msg.error as { code: number; message: string };
          console.log('[WS] call <- ERROR id=%s code=%d message=%s', id, err.code, err.message);
          reject(new Error(err.message || `RPC error ${err.code}`));
        } else {
          console.log('[WS] call <- OK id=%s result=%o', id, msg.result);
          resolve(msg.result);
        }
        return;
      }

      // 未匹配到任何 handler（可能是已被取消的流式请求）
      console.log('[WS] unmatched response id=%s (no pending or stream handler)', id);
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      this.call('system.ping').catch(() => {});
    }, 30000);
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private scheduleReconnect() {
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), this.maxReconnectDelay);
    this.reconnectAttempts++;

    this.reconnectTimer = setTimeout(() => {
      this.connect().catch(() => {
        // 重连失败由 onclose 再次触发
      });
    }, delay);
  }
}

// 单例
const WS_URL = `ws://${window.location.hostname}:8080/ws`;
export const wsClient = new WSClient(WS_URL);