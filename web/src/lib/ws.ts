/* ws: tiny typed helper for the per-session terminal socket */

const encoder = new TextEncoder();

// open a binary WebSocket to the session's /ws/<id> endpoint
export function openSessionSocket(id: string): WebSocket {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws/${id}`);
  ws.binaryType = "arraybuffer";
  return ws;
}

// send a {type:"resize"} control message when the socket is open
export function sendResize(ws: WebSocket, cols: number, rows: number): void {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: "resize", cols, rows }));
  }
}

// send raw bytes (encoded from a string) when the socket is open
export function sendBytes(ws: WebSocket, data: string): void {
  if (ws.readyState === WebSocket.OPEN) ws.send(encoder.encode(data));
}
