import { onBeforeUnmount } from 'vue';
import { ref, type Ref } from 'vue';

export interface WsIncoming {
  type: 'message' | 'error' | 'ack' | 'subscribed' | 'typing' | 'notification' | 'chat.unread' | 'read';
  conversation_id?: string;
  message?: {
    id: string;
    conversation_id: string;
    author_id: string;
    body: string;
    created_at_unix: number;
  };
  user_id?: string;
  is_typing?: boolean;
  notification?: any;
  error?: string;
}

export interface ChatSocket {
  connected: Ref<boolean>;
  subscribe: (convId: string) => void;
  unsubscribe: (convId: string) => void;
  send: (convId: string, body: string) => boolean;
  sendTyping: (convId: string, isTyping: boolean) => boolean;
  sendRead: (convId: string) => boolean;
  onMessage: (fn: (msg: WsIncoming) => void) => () => void;
  close: () => void;
}

export function useChatSocket(): ChatSocket {
  const connected = ref(false);
  let ws: WebSocket | null = null;
  const listeners: ((msg: WsIncoming) => void)[] = [];
  const pendingSubs = new Set<string>();
  let reconnectTimer: number | null = null;
  let closedByUser = false;
  let reconnectAttempts = 0;
  const maxReconnectAttempts = 5;

  function connect() {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${window.location.host}/api/v1/ws/chat`;

    ws = new WebSocket(url);

    ws.onopen = () => {
      connected.value = true;
      reconnectAttempts = 0;
      
      // CRITICAL FIX: Resubscribe to all rooms we are currently in
      for (const id of pendingSubs) {
        console.log("📡 RESUBSCRIBING TO ROOM:", id);
        ws?.send(JSON.stringify({ type: 'subscribe', conversation_id: id }));
      }
      
      window.dispatchEvent(new CustomEvent('ws-reconnected'));
    };

    ws.onmessage = (evt) => {
      try {
        const msg: WsIncoming = JSON.parse(evt.data);
        for (const fn of listeners) fn(msg);
      } catch {
        // ignore
      }
    };

    ws.onclose = () => {
      connected.value = false;
      ws = null;
      if (!closedByUser && reconnectAttempts < maxReconnectAttempts) {
        reconnectAttempts++;
        const timeout = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
        reconnectTimer = window.setTimeout(connect, timeout);
      }
    };

    ws.onerror = () => {
      ws?.close();
    };
  }

  function subscribe(convId: string) {
    pendingSubs.add(convId);
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'subscribe', conversation_id: convId }));
    }
  }

  function unsubscribe(convId: string) {
    pendingSubs.delete(convId);
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'unsubscribe', conversation_id: convId }));
    }
  }

  function send(convId: string, body: string): boolean {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'message', conversation_id: convId, body }));
      return true;
    }
    return false;
  }

  function sendTyping(convId: string, isTyping: boolean): boolean {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({
          type: 'typing',
          conversation_id: convId,
          is_typing: isTyping,
        })
      );
      return true;
    }
    return false;
  }

  function sendRead(convId: string): boolean {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'read', conversation_id: convId }));
      return true;
    }
    return false;
  }

  function onMessage(fn: (msg: WsIncoming) => void) {
    listeners.push(fn);
    return () => {
      const i = listeners.indexOf(fn);
      if (i >= 0) listeners.splice(i, 1);
    };
  }

  function close() {
    closedByUser = true;
    if (reconnectTimer) window.clearTimeout(reconnectTimer);
    ws?.close();
    ws = null;
    connected.value = false;
  }

  connect();

  onBeforeUnmount(() => {
    close();
  });

  return { connected, subscribe, unsubscribe, send, sendTyping, sendRead, onMessage, close };
}
