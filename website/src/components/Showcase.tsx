import { useState, type ReactNode } from "react";

type View = { id: string; tab: string; title: string; caption: string; body: ReactNode };

export const servers = [
  { name: "survival", meta: "Paper 1.21.4", state: "online" },
  { name: "creative", meta: "Purpur 1.21.4", state: "online" },
  { name: "modded", meta: "NeoForge 1.21.1", state: "crashed" },
  { name: "lobby", meta: "Paper 1.21.4", state: "asleep" },
];

const plugins = [
  { name: "LuckPerms", by: "Permissions for players and groups", version: "5.5.0", source: "Modrinth", action: "Up to date" },
  { name: "EssentialsX", by: "Homes, warps, kits and chat basics", version: "2.20.1", source: "Modrinth", action: "Update to 2.21.0" },
  { name: "Geyser", by: "Lets Bedrock players join", version: "2.6.1", source: "Hangar", action: "Up to date" },
  { name: "Floodgate", by: "Added automatically with Geyser", version: "2.2.4", source: "Hangar", action: "Up to date" },
  { name: "ViaVersion", by: "Lets newer clients connect", version: "5.2.0", source: "Hangar", action: "Update to 5.2.1" },
  { name: "WorldEdit", by: "In-game map editor", version: "7.3.9", source: "CurseForge", action: "Up to date" },
];

function PluginsView() {
  return (
    <>
      <div className="app-toolbar">
        <span className="app-search">Search 60,000 plugins and mods</span>
        <span className="app-filter on">Installed 34</span>
        <span className="app-filter">Updates 6</span>
      </div>
      <ul className="plugin-list">
        {plugins.map((plugin) => (
          <li key={plugin.name}>
            <span className="plugin-icon">{plugin.name[0]}</span>
            <span className="plugin-name">
              {plugin.name}
              <small>{plugin.by}</small>
            </span>
            <span className="plugin-version">{plugin.version}</span>
            <span className="plugin-source">{plugin.source}</span>
            <span className={plugin.action.startsWith("Update") ? "app-button" : "plugin-ok"}>{plugin.action}</span>
          </li>
        ))}
      </ul>
    </>
  );
}

function CrashView() {
  return (
    <div className="crash">
      <div className="crash-banner">
        <strong>modded stopped at 14:02</strong>
        <span>4 minutes after a plugin was added</span>
      </div>
      <pre className="log">
        <span>[14:02:11 INFO]: Loading plugins...</span>
        <mark>[14:02:11 ERROR]: Could not load 'plugins/EssentialsXChat-2.21.0.jar'</mark>
        <mark>org.bukkit.plugin.UnknownDependencyException:</mark>
        <mark>{"  "}Unknown/missing dependency plugins: [Essentials]</mark>
        <span>{"    "}at org.bukkit.plugin.SimplePluginManager.loadPlugins(SimplePluginManager.java:219)</span>
        <span>{"    "}at org.bukkit.craftbukkit.CraftServer.loadPlugins(CraftServer.java:507)</span>
      </pre>
      <div className="crash-answer">
        <p className="crash-kicker">What happened</p>
        <p>
          <strong>EssentialsX Chat</strong> didn't load because it needs <strong>EssentialsX</strong>, which
          isn't installed on this server.
        </p>
        <div className="crash-actions">
          <span className="app-button">Install EssentialsX 2.21.0</span>
          <span className="app-button ghost">Remove EssentialsX Chat</span>
        </div>
      </div>
    </div>
  );
}

function ConfigView() {
  return (
    <div className="config">
      <div className="config-form">
        <p className="app-heading">EssentialsX · Homes</p>
        <label>
          Homes per player
          <span className="field">5</span>
        </label>
        <label>
          Teleport delay
          <span className="field">
            3 <small>seconds</small>
          </span>
        </label>
        <label>
          Allow homes in the Nether
          <span className="toggle on" />
        </label>
        <label>
          Respawn at home
          <span className="toggle" />
        </label>
        <p className="field-error">Teleport delay must be a whole number.</p>
      </div>
      <div className="config-diff">
        <p className="app-heading">2 lines will change in config.yml</p>
        <pre className="log">
          <span>sethome-multiple:</span>
          <del>{"  "}default: 3</del>
          <ins>{"  "}default: 5</ins>
          <del>teleport-delay: 5</del>
          <ins>teleport-delay: 3</ins>
          <span>respawn-at-home: false</span>
        </pre>
        <span className="app-button">Save and reload</span>
      </div>
    </div>
  );
}

function VersionsView() {
  return (
    <div className="versions">
      <p className="app-heading">Server software</p>
      <div className="software">
        <span className="on">
          Paper<small>Plugins, fast</small>
        </span>
        <span>
          Purpur<small>Paper with extras</small>
        </span>
        <span>
          Fabric<small>Lightweight mods</small>
        </span>
        <span>
          NeoForge<small>Large modpacks</small>
        </span>
      </div>
      <dl className="version-facts">
        <div>
          <dt>Minecraft version</dt>
          <dd>1.21.4 → 1.21.5</dd>
        </div>
        <div>
          <dt>Build</dt>
          <dd>Latest stable</dd>
        </div>
        <div>
          <dt>Java</dt>
          <dd>21, downloaded for you</dd>
        </div>
        <div>
          <dt>Backup</dt>
          <dd>Taken before the switch</dd>
        </div>
      </dl>
      <span className="app-button">Switch to Paper 1.21.5</span>
    </div>
  );
}

function Spark({ points }: { points: string }) {
  return (
    <svg className="spark" viewBox="0 0 100 22" preserveAspectRatio="none">
      <polyline points={points} />
    </svg>
  );
}

function ConsoleView() {
  return (
    <div className="console">
      <dl className="console-stats">
        <div>
          <dt>CPU</dt>
          <dd>
            34% <small>of 4 cores</small>
          </dd>
          <Spark points="0,16 9,14 18,17 27,12 36,13 45,9 54,12 63,6 72,10 81,8 90,12 100,9" />
        </div>
        <div>
          <dt>Memory</dt>
          <dd>
            3.1 <small>of 6 GB</small>
          </dd>
          <Spark points="0,15 9,15 18,14 27,13 36,13 45,12 54,12 63,11 72,11 81,10 90,10 100,10" />
        </div>
        <div>
          <dt>Ticks per second</dt>
          <dd>
            19.9 <small>of 20</small>
          </dd>
          <Spark points="0,4 9,4 18,5 27,4 36,4 45,4 54,13 63,6 72,4 81,4 90,5 100,4" />
        </div>
        <div>
          <dt>Network</dt>
          <dd>
            1.2 <small>MB/s out</small>
          </dd>
          <Spark points="0,18 9,15 18,16 27,11 36,14 45,10 54,12 63,7 72,9 81,11 90,6 100,8" />
        </div>
      </dl>
      <p className="console-meta">
        <span>12 / 40 players</span>
        <span>0.0.0.0:25565/tcp</span>
        <span>Disk 4.2 of 20 GB</span>
        <span>Up 6 d 4 h</span>
        <span>Last backup 18:43</span>
      </p>
      <pre className="log">
        <span>[18:40:02 INFO]: Mossy_Kat joined the game</span>
        <span>[18:40:31 INFO]: &lt;Mossy_Kat&gt; anyone at spawn?</span>
        <em>[18:41:10 WARN]: Can't keep up! Running 2100ms behind</em>
        <span>[18:42:55 INFO]: brickwren left the game</span>
        <span>[18:43:00 INFO]: Backup finished in 14s (212 MB)</span>
        <span>[18:44:19 INFO]: Mossy_Kat has made the advancement [Stone Age]</span>
      </pre>
      <div className="console-input">
        <span>&gt;</span> whitelist add brickwren<i />
      </div>
    </div>
  );
}

export const views: View[] = [
  {
    id: "plugins",
    tab: "Plugins",
    title: "Install and update plugins without the file manager",
    caption:
      "Search Modrinth, Hangar and CurseForge from inside the panel. Dependencies come along automatically, and every update is backed up first.",
    body: <PluginsView />,
  },
  {
    id: "crash",
    tab: "Crash explainer",
    title: "Read a crash in one sentence",
    caption:
      "Consolry matches the stack trace to the plugin it came from and tells you what to do. It runs on your machine, so the log is never uploaded.",
    body: <CrashView />,
  },
  {
    id: "config",
    tab: "Config forms",
    title: "Change settings in a form, not a YAML file",
    caption:
      "Plugin settings become a page that checks what you type. Before saving, you see exactly which lines will change.",
    body: <ConfigView />,
  },
  {
    id: "versions",
    tab: "Versions",
    title: "Switch server software without guessing",
    caption:
      "Move between Paper, Purpur, Fabric and NeoForge. Consolry fetches the build and the Java version it needs.",
    body: <VersionsView />,
  },
  {
    id: "console",
    tab: "Console",
    title: "And the basics, done properly",
    caption:
      "A console with search and history, a file manager, backups, schedules and roles. Everything a panel should already have.",
    body: <ConsoleView />,
  },
];

export default function Showcase() {
  const [active, setActive] = useState(views[0].id);
  const view = views.find((item) => item.id === active)!;

  return (
    <div className="showcase">
      <div className="showcase-tabs" role="tablist" aria-label="Panel features">
        {views.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            id={`tab-${item.id}`}
            aria-selected={item.id === active}
            aria-controls="showcase-panel"
            onClick={() => setActive(item.id)}
          >
            {item.tab}
          </button>
        ))}
      </div>

      <div className="showcase-caption">
        <h3>{view.title}</h3>
        <p>{view.caption}</p>
      </div>

      <div id="showcase-panel" role="tabpanel" aria-labelledby={`tab-${view.id}`}>
        <AppWindow viewId={view.id}>{view.body}</AppWindow>
      </div>
    </div>
  );
}

const appTabs = ["Console", "Files", "Plugins", "Config", "Versions", "Backups", "Schedules", "Users"];
const activeTab: Record<string, string> = {
  plugins: "Plugins",
  crash: "Console",
  config: "Config",
  versions: "Versions",
  console: "Console",
};

export function AppWindow({ viewId, children }: { viewId: string; children: ReactNode }) {
  const selected = viewId === "crash" ? "modded" : "survival";
  return (
    <div className="app">
      <div className="app-bar">
        <strong>Consolry</strong>
        <span>
          Servers / <b>{selected}</b>
        </span>
        <span className="app-node">node-01 · Linux · 25565/tcp</span>
        <span className="app-tag">Preview</span>
      </div>
      <div className="app-tabs">
        {appTabs.map((tab) => (
          <span key={tab} className={tab === activeTab[viewId] ? "on" : undefined}>
            {tab}
          </span>
        ))}
      </div>
      <div className="app-body">
        <aside className="app-side">
          <p>Servers</p>
          <ul>
            {servers.map((server) => (
              <li key={server.name} className={server.name === selected ? "on" : undefined}>
                <span className={`dot ${server.state}`} />
                <span>
                  {server.name}
                  <small>{server.meta}</small>
                </span>
              </li>
            ))}
          </ul>
        </aside>
        <div className="app-main">{children}</div>
      </div>
    </div>
  );
}
