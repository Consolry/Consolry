export type User = { id: number; username: string };
export type PanelState = { setupNeeded: boolean; version: string; user: User | null };
export type NodeInfo = { id: number; name: string; url: string; online: boolean; os?: string; version?: string };
export type ServerState = "offline" | "running" | "stopping" | "crashed" | "unreachable";
export type ServerInfo = {
  id: string;
  name: string;
  nodeId: number;
  nodeName: string;
  state: ServerState;
  command: string;
  args: string[];
  stopCommand: string;
};

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(`/api${path}`, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error ?? `Request failed (${response.status})`);
  return data as T;
}

export const api = {
  state: () => request<PanelState>("GET", "/state"),
  setup: (username: string, password: string) => request<User>("POST", "/setup", { username, password }),
  login: (username: string, password: string) => request<User>("POST", "/login", { username, password }),
  logout: () => request<void>("POST", "/logout", {}),

  nodes: () => request<NodeInfo[]>("GET", "/nodes"),
  createNode: (name: string, url: string, token: string) => request<NodeInfo>("POST", "/nodes", { name, url, token }),
  deleteNode: (id: number) => request<void>("DELETE", `/nodes/${id}`),

  servers: () => request<ServerInfo[]>("GET", "/servers"),
  createServer: (input: { name: string; nodeId: number; startCommand: string; stopCommand: string }) =>
    request<{ id: string }>("POST", "/servers", input),
  deleteServer: (id: string) => request<void>("DELETE", `/servers/${id}`),
  power: (id: string, action: "start" | "stop" | "kill") => request<{ state: string }>("POST", `/servers/${id}/power`, { action }),
};

export function consoleSocket(serverId: string) {
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  return new WebSocket(`${scheme}://${location.host}/api/servers/${serverId}/console`);
}
