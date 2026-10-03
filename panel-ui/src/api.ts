export type Limits = {
  /** How many servers of their own the account may create. */
  serverLimit: number;
  /** The most memory the account may give one server. */
  memoryLimitMb: number;
};
export type User = { id: number; username: string; admin: boolean; twoFactor: boolean } & Limits;
/** A sign-in answer: either the account, or a request for the code from an authenticator app. */
export type LoginResult = User | { twoFactor: true; ticket: string };
export type Account = User & { servers: number };
export type AlertSettings = { discord: string; email: string; events: string[] };
export type SmtpSettings = { host: string; port: number; username: string; password?: string; from: string };
export type UpdateStatus = { current: string; latest: string; available: boolean; notes: string; problem: string };
export type PanelState = { setupNeeded: boolean; version: string; user: User | null; signupAllowed: boolean };
export type Permission = "console" | "power" | "files" | "plugins" | "players" | "settings" | "backups" | "schedules" | "network" | "activity";
/** What a server's owner can let an invited person do, in the order it is offered. */
export const permissionList: { id: Permission; label: string; about: string; minecraftOnly?: boolean }[] = [
  { id: "console", label: "Console", about: "Read the console and type commands." },
  { id: "power", label: "Start and stop", about: "Start, stop and kill the server." },
  { id: "files", label: "Files", about: "Open, change, upload and delete the server's files." },
  { id: "plugins", label: "Plugins and mods", about: "Install and update them.", minecraftOnly: true },
  { id: "players", label: "Players", about: "Kick, ban, whitelist and make operators.", minecraftOnly: true },
  { id: "settings", label: "Settings", about: "Game settings, version, memory and start-up options." },
  { id: "backups", label: "Backups", about: "Make, download, restore and delete backups." },
  { id: "schedules", label: "Schedules", about: "Add, change and run scheduled tasks." },
  { id: "network", label: "Network", about: "See the address and open or close the port." },
  { id: "activity", label: "Activity", about: "See who did what on the server." },
];
export type Member = { userId: number; username: string; permissions: Permission[] };
export type RemoteMode = "off" | "network" | "internet";
export type PanelSettings = {
  signup: boolean;
  remote: { mode: RemoteMode; port: number; networkAddress: string; internetAddress: string; problem: string };
};
export type NodeInfo = { id: number; name: string; url: string; online: boolean; os?: string; version?: string };
export type ServerState = "offline" | "starting" | "running" | "stopping" | "crashed" | "unreachable";

/** True while the server's process exists: starting, running or shutting down. */
export const isLive = (state: ServerState) => state === "starting" || state === "running" || state === "stopping";
export type PowerAction = "start" | "stop" | "kill";
export type ServerInfo = {
  id: string;
  name: string;
  nodeId: number;
  nodeName: string;
  state: ServerState;
  command: string;
  args: string[];
  stopCommand: string;
  /** Unix seconds when the running process started, or 0. */
  startedAt: number;
  kind: "generic" | "minecraft";
  software: string;
  mcVersion: string;
  /** Percentage of the whole machine's CPU. */
  cpu: number;
  /** Bytes of memory in use. */
  memory: number;
  memoryLimitMb: number;
  /** The server's newest output line while it is starting. */
  progress: string;
  /** True for the server's owner and for admins. */
  owner: boolean;
  /** What the signed-in user may do on this server. */
  permissions: Permission[];
};
export type Activity = { id: number; at: number; user: string; text: string };
export type Forward = { port: number; externalIp: string; reachable: boolean };
export type Network = { port: number; addresses: string[]; listening: boolean; forward: Forward | null };
export type Startup = {
  command: string;
  args: string[];
  stopCommand: string;
  memoryMb: number;
  javaOptions: string[];
  javaNeeded: number;
  systemJava: string;
  managedJava: number[];
  recommended: string[];
  fastStart: string[];
};
export type Software = { id: string; name: string; about: string; folder: string };
export type FileEntry = { name: string; dir: boolean; size: number; modified: number };
export type Backup = { name: string; size: number; created: number; kind: "manual" | "auto" | "scheduled"; skipped?: string[] };
export type ScheduleAction = "start" | "stop" | "restart" | "backup" | "command";
export type ScheduleMode = "interval" | "daily" | "weekly";
export type Schedule = {
  id: number;
  action: ScheduleAction;
  command: string;
  mode: ScheduleMode;
  minutes: number;
  at: string;
  weekday: number;
  enabled: boolean;
  lastRun: number;
  lastResult: string;
  nextRun: number;
};
export type NewSchedule = Pick<Schedule, "action" | "command" | "mode" | "minutes" | "at" | "weekday">;
export type Players = {
  running: boolean;
  online: string[];
  max: number;
  whitelist: string[];
  whitelistEnabled: boolean;
  operators: string[];
  banned: { name: string; reason: string }[];
};
export type ServerPatch = { name?: string; memoryMb?: number; startCommand?: string; stopCommand?: string; javaOptions?: string };
export type Plugin = {
  file: string;
  size: number;
  verified: boolean;
  projectId?: string;
  title?: string;
  iconUrl?: string;
  version?: string;
  update?: string;
  source?: PluginSource;
};
export type PluginList = { folder: string; items: Plugin[]; lookupFailed: boolean };
export type PluginSource = "modrinth" | "hangar" | "curseforge";
export type Project = { projectId: string; title: string; description: string; author: string; downloads: number; iconUrl: string; source: PluginSource };
export type PluginSearch = { projects: Project[]; total: number; pageSize: number; sources: PluginSource[]; curseforgeReady: boolean };
export type Finding = { title: string; detail: string; fix: string; line: string };
export type NewServerInput = {
  name: string;
  nodeId: number;
  startCommand?: string;
  stopCommand?: string;
  minecraft?: { software: string; version: string; memoryMb: number; acceptEula: boolean };
};

async function fail(response: Response): Promise<never> {
  const data = await response.json().catch(() => ({}));
  throw new Error(data.error ?? `Request failed (${response.status})`);
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(`/api${path}`, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) return fail(response);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

const q = encodeURIComponent;
const node = (id: string) => `/servers/${id}/node`;

export const api = {
  state: () => request<PanelState>("GET", "/state"),
  setup: (username: string, password: string) => request<User>("POST", "/setup", { username, password }),
  login: (username: string, password: string) => request<LoginResult>("POST", "/login", { username, password }),
  loginCode: (ticket: string, code: string) => request<User>("POST", "/login/code", { ticket, code }),
  changePassword: (current: string, next: string) => request<void>("POST", "/account/password", { current, new: next }),
  startTwoFactor: () => request<{ secret: string; qr: string }>("POST", "/account/two-factor/start", {}),
  enableTwoFactor: (secret: string, code: string) => request<{ recoveryCodes: string[] }>("POST", "/account/two-factor", { secret, code }),
  disableTwoFactor: (password: string) => request<void>("DELETE", "/account/two-factor", { password }),
  resetAccount: (userId: number, password: string, clearTwoFactor: boolean) =>
    request<void>("POST", `/accounts/${userId}/reset`, { password, clearTwoFactor }),
  signup: (username: string, password: string) => request<User>("POST", "/signup", { username, password }),
  logout: () => request<void>("POST", "/logout", {}),

  panelSettings: () => request<PanelSettings>("GET", "/panel"),
  updatePanelSettings: (patch: { signup?: boolean; remote?: RemoteMode }) => request<PanelSettings>("POST", "/panel", patch),

  serverAlerts: (id: string) => request<AlertSettings>("GET", `/servers/${id}/alerts`),
  setServerAlerts: (id: string, alerts: AlertSettings, test: boolean) => request<AlertSettings>("POST", `/servers/${id}/alerts`, { ...alerts, test }),
  panelAlerts: () => request<{ alerts: AlertSettings; smtp: SmtpSettings; smtpPasswordSet: boolean }>("GET", "/alerts"),
  setPanelAlerts: (alerts: AlertSettings, test: boolean) => request<AlertSettings>("POST", "/alerts", { ...alerts, test }),
  setSmtp: (smtp: SmtpSettings) => request<void>("POST", "/alerts/smtp", smtp),

  update: () => request<UpdateStatus>("GET", "/update"),
  installUpdate: () => request<void>("POST", "/update", {}),

  accounts: () => request<{ accounts: Account[]; newAccounts: Limits }>("GET", "/accounts"),
  setAccountLimits: (userId: number, limits: Limits) => request<void>("PATCH", `/accounts/${userId}`, limits),
  setNewAccountLimits: (limits: Limits) => request<void>("POST", "/accounts/defaults", limits),
  deleteAccount: (userId: number) => request<void>("DELETE", `/accounts/${userId}`),

  members: (id: string) => request<Member[]>("GET", `/servers/${id}/users`),
  invite: (id: string, username: string, permissions: Permission[]) => request<Member>("POST", `/servers/${id}/users`, { username, permissions }),
  setMemberPermissions: (id: string, userId: number, permissions: Permission[]) =>
    request<void>("PATCH", `/servers/${id}/users/${userId}`, { permissions }),
  removeMember: (id: string, userId: number) => request<void>("DELETE", `/servers/${id}/users/${userId}`),

  nodes: () => request<NodeInfo[]>("GET", "/nodes"),
  createNode: (name: string, url: string, token: string) => request<NodeInfo>("POST", "/nodes", { name, url, token }),
  deleteNode: (id: number) => request<void>("DELETE", `/nodes/${id}`),

  servers: () => request<ServerInfo[]>("GET", "/servers"),
  createServer: (input: NewServerInput) => request<{ id: string; warning: string }>("POST", "/servers", input),
  deleteServer: (id: string) => request<void>("DELETE", `/servers/${id}`),
  power: (id: string, action: PowerAction) => request<{ state: string }>("POST", `/servers/${id}/power`, { action }),
  diagnosis: (id: string) => request<Finding[]>("GET", `/servers/${id}/diagnosis`),
  updateServer: (id: string, patch: ServerPatch) => request<void>("PATCH", `/servers/${id}`, patch),
  switchVersion: (id: string, software: string, version: string) =>
    request<{ warning: string }>("POST", `/servers/${id}/minecraft/version`, { software, version }),

  players: (id: string) => request<Players>("GET", `/servers/${id}/players`),
  playerAction: (id: string, action: string, name = "", reason = "") => request<void>("POST", `/servers/${id}/players`, { action, name, reason }),

  schedules: (id: string) => request<Schedule[]>("GET", `/servers/${id}/schedules`),
  createSchedule: (id: string, input: NewSchedule) => request<Schedule>("POST", `/servers/${id}/schedules`, input),
  setScheduleEnabled: (scheduleId: number, enabled: boolean) => request<void>("PATCH", `/schedules/${scheduleId}`, { enabled }),
  deleteSchedule: (scheduleId: number) => request<void>("DELETE", `/schedules/${scheduleId}`),
  runSchedule: (scheduleId: number) => request<{ result: string }>("POST", `/schedules/${scheduleId}/run`, {}),

  software: () => request<Software[]>("GET", "/minecraft/software"),
  versions: (software: string) => request<string[]>("GET", `/minecraft/versions?software=${q(software)}`),

  files: (id: string, path: string) => request<FileEntry[]>("GET", `${node(id)}/files?path=${q(path)}`),
  fileUrl: (id: string, path: string) => `/api${node(id)}/files/content?path=${q(path)}`,
  readFile: async (id: string, path: string) => {
    const response = await fetch(api.fileUrl(id, path));
    if (!response.ok) return fail(response);
    return response.text();
  },
  writeFile: async (id: string, path: string, content: Blob | string) => {
    const response = await fetch(api.fileUrl(id, path), { method: "PUT", body: content });
    if (!response.ok) return fail(response);
  },
  makeFolder: (id: string, path: string) => request<void>("POST", `${node(id)}/files/mkdir`, { path }),
  rename: (id: string, from: string, to: string) => request<void>("POST", `${node(id)}/files/rename`, { from, to }),
  remove: (id: string, path: string) => request<void>("DELETE", `${node(id)}/files?path=${q(path)}`),

  backups: (id: string) => request<Backup[]>("GET", `${node(id)}/backups`),
  createBackup: (id: string) => request<Backup>("POST", `${node(id)}/backups`, {}),
  restoreBackup: (id: string, name: string) => request<void>("POST", `${node(id)}/backups/${name}/restore`, {}),
  deleteBackup: (id: string, name: string) => request<void>("DELETE", `${node(id)}/backups/${name}`),
  backupUrl: (id: string, name: string) => `/api${node(id)}/backups/${name}`,

  plugins: (id: string) => request<PluginList>("GET", `/servers/${id}/plugins`),
  searchPlugins: (id: string, query: string, page: number, source: PluginSource) =>
    request<PluginSearch>("GET", `/servers/${id}/plugins/search?q=${q(query)}&page=${page}&source=${source}`),
  setCurseForgeKey: (key: string) => request<void>("POST", "/plugin-sources", { curseforgeKey: key }),
  activity: (id: string, limit: number) => request<Activity[]>("GET", `/servers/${id}/activity?limit=${limit}`),
  network: (id: string) => request<Network>("GET", `/servers/${id}/network`),
  setForward: (id: string, enabled: boolean) => request<Forward | void>("POST", `/servers/${id}/network/forward`, { enabled }),
  startup: (id: string) => request<Startup>("GET", `/servers/${id}/startup`),
  installPlugin: (id: string, project: Project) =>
    request<{ installed: string[] }>("POST", `/servers/${id}/plugins/install`, {
      projectId: project.projectId,
      source: project.source,
      title: project.title,
      iconUrl: project.iconUrl,
    }),
  updatePlugin: (id: string, file: string) => request<{ file: string }>("POST", `/servers/${id}/plugins/update`, { file }),
};

export function consoleSocket(serverId: string) {
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  return new WebSocket(`${scheme}://${location.host}/api/servers/${serverId}/console`);
}

const stateLabels: Record<ServerState, string> = {
  running: "Running",
  starting: "Starting",
  offline: "Offline",
  stopping: "Stopping",
  crashed: "Crashed",
  unreachable: "Node offline",
};

export const stateLabel = (state: ServerState) => stateLabels[state];

/** How long a server has been up, such as "3 h 12 min". */
export function uptime(startedAt: number, now: number) {
  if (!startedAt) return "–";
  const seconds = Math.max(0, Math.floor(now / 1000) - startedAt);
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days) return `${days} d ${hours} h`;
  if (hours) return `${hours} h ${minutes} min`;
  if (minutes) return `${minutes} min ${seconds % 60} s`;
  return `${seconds} s`;
}

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`;
}

export function formatTime(unixSeconds: number) {
  return new Date(unixSeconds * 1000).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}
