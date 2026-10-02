export const site = {
  url: "https://www.consolry.com",
  name: "Consolry",
  // The Buttondown account name, from buttondown.com/<name>. Empty means the waitlist is closed.
  buttondownUsername: "",
};

export const routes = [
  {
    path: "/",
    nav: "",
    title: "Consolry: the game server panel that knows your game",
    description:
      "Consolry is an open source game server panel that manages Minecraft plugins, updates, crashes and configs for you, on Windows or Linux.",
  },
  {
    path: "/features",
    nav: "Features",
    title: "Features | Consolry",
    description:
      "Plugin manager, crash explainer, config forms and version switcher, on top of a proper console, file manager and backups.",
  },
  {
    path: "/pricing",
    nav: "Pricing",
    title: "Pricing | Consolry",
    description:
      "Consolry is free to self-host with no server limit. Paid plans from $6 a month add upgrade planning, networks and hosting tools.",
  },
  {
    path: "/roadmap",
    nav: "Roadmap",
    title: "Roadmap | Consolry",
    description: "What is being built, in what order, and what the first release of Consolry will include.",
  },
  {
    path: "/faq",
    nav: "FAQ",
    title: "Questions and answers | Consolry",
    description: "Is Consolry free? Does it run on Windows? Is it a Pterodactyl fork? Answers to the common questions.",
  },
];

export const checks = ["Free open source core", "Windows and Linux", "No Docker needed on Windows"];

export const problems = [
  {
    title: "Your panel can't see the game",
    body: "It starts a container and shows a console. It has no idea there are 34 plugins inside, six of them out of date.",
  },
  {
    title: "Every update is a coin toss",
    body: "You learn which plugin breaks by updating and watching the server fall over, usually with players online.",
  },
  {
    title: "It only runs on Linux",
    body: "Most people who want a server for friends own a Windows PC. Most panels ask them to learn Docker first.",
  },
];

export const comparisons = [
  {
    task: "Update a plugin",
    before: "Find the download page, upload the jar over SFTP, delete the old one, restart, hope.",
    after: "Click Update. A backup is taken first.",
  },
  {
    task: "Work out a crash",
    before: "Scroll through 400 lines of stack trace, then paste it into a Discord help channel.",
    after: "Read one sentence that names the plugin.",
  },
  {
    task: "Change a setting",
    before: "Edit YAML by hand and find out on restart whether the indentation was right.",
    after: "Fill in a form that checks what you type.",
  },
  {
    task: "Move to a new version",
    before: "Download the server jar and work out which Java it wants this time.",
    after: "Pick the version. Java is chosen for you.",
  },
  {
    task: "Host on your own PC",
    before: "Install Docker and WSL, or rent a Linux box instead.",
    after: "Run one Windows installer.",
  },
];

export const basics = [
  { name: "Console", detail: "Search, filters by log level, command history and scrollback that survives a refresh." },
  { name: "Files", detail: "File manager and editor, drag-and-drop uploads, archive extract, and SFTP." },
  { name: "Backups", detail: "Manual and scheduled, local or off-site, and automatic before every change." },
  { name: "Schedules", detail: "Plain-language timing, such as every day at 4am." },
  { name: "Users and roles", detail: "Accounts, sub-users, built-in roles and two-factor login." },
  { name: "API", detail: "Everything the interface does, available to scripts with scoped keys." },
];

export const audiences = [
  {
    who: "Server owners",
    what: "Fewer broken updates, crash reports you can read, and settings pages instead of YAML.",
    when: "First release",
  },
  {
    who: "Home users",
    what: "A Windows installer and a server your friends can join, with ports opened for you.",
    when: "First release, Consolry Home after",
  },
  {
    who: "Hosting companies",
    what: "Fewer support tickets, your own branding, and billing integrations.",
    when: "Later",
  },
  {
    who: "Developers",
    what: "A full API from day one, then a module SDK for adding games and pages.",
    when: "API first, SDK later",
  },
];

export const platforms = [
  {
    name: "Windows",
    how: "Servers run as normal programs. One installer sets up the panel, the node software and the database.",
    when: "First release",
  },
  {
    name: "Linux",
    how: "Servers run in containers, sealed off from each other. Safe for hosting other people's servers.",
    when: "First release",
  },
  { name: "macOS", how: "For running a server at home or testing plugins.", when: "Later" },
  { name: "ARM", how: "Raspberry Pi and ARM cloud servers.", when: "Later" },
];

export type Plan = {
  name: string;
  monthly: number;
  perNode?: number;
  featured?: boolean;
  audience: string;
  includes: string;
  features: string[];
  support: string;
};

export const plans: Plan[] = [
  {
    name: "Community",
    monthly: 0,
    featured: true,
    audience: "Home users and small servers",
    includes: "The whole panel:",
    features: [
      "Console, files, schedules and roles",
      "Plugin manager and version switcher",
      "Crash explainer and config forms",
      "Local and off-site backups, taken before every change",
      "Two-factor login and single sign-on",
      "Windows and Linux",
      "Full API",
    ],
    support: "Docs and Discord",
  },
  {
    name: "Plus",
    monthly: 6,
    audience: "Serious owners and communities",
    includes: "Everything in Community, and:",
    features: [
      "Upgrade planner and staging clones",
      "Scheduled plugin updates with rollback",
      "Sleep mode for empty servers",
      "Performance view that names what is lagging",
      "Restore a single player or file",
      "World pruning and size reports",
      "Resource pack tools",
    ],
    support: "Discord, with a supporter role",
  },
  {
    name: "Pro",
    monthly: 19,
    audience: "Networks and teams",
    includes: "Everything in Plus, and:",
    features: [
      "Networks: a proxy and its servers as one",
      "Server templates",
      "Git sync and deploy hooks",
      "Public status pages",
      "Share links for temporary access",
      "Audit log export",
    ],
    support: "Email, reply within 2 business days",
  },
  {
    name: "Host",
    monthly: 49,
    perNode: 8,
    audience: "Companies selling servers",
    includes: "Everything in Pro, and:",
    features: [
      "Billing integrations",
      "Resource pools customers split themselves",
      "Your branding and your domain",
      "Support view and staff roles",
      "Abuse controls and node monitoring",
    ],
    support: "Priority email, reply within 1 business day",
  },
];

export const pricingNotes = [
  "The first release is Community only. Paid plans open as their features are built.",
  "Try Pro for 14 days without a card.",
  "If a subscription ends, paid features switch off. Your servers keep running and nothing is deleted.",
  "Prices are in US dollars and may change before the paid plans open.",
];

export const stages = [
  { name: "Foundations", detail: "This site, the repository and the build setup.", status: "In progress" },
  { name: "Daemon and core panel", detail: "Create, start and manage a server on Windows and Linux.", status: "Next" },
  { name: "Everyday basics", detail: "Backups, schedules, notifications and two-factor login.", status: "Planned" },
  { name: "Minecraft module", detail: "Plugin manager, version switcher, crash explainer, config forms.", status: "First release" },
  { name: "Consolry Home", detail: "Single-PC mode, automatic port forwarding, Pterodactyl importer.", status: "Planned" },
  { name: "Safe upgrades", detail: "Upgrade planner and staging clones. The Plus plan opens.", status: "Planned" },
  { name: "Networks and teams", detail: "Networks, templates, sleep mode, status pages. The Pro plan opens.", status: "Planned" },
  { name: "Hosting", detail: "Billing, resource pools, white-label. The Host plan opens.", status: "Planned" },
];

export const firstRelease = [
  { area: "Console", now: "Live log, command input, history, search", later: "Pop-out, split view, saved commands" },
  { area: "Files", now: "File manager, editor, uploads, SFTP", later: "Content search, version history" },
  { area: "Backups", now: "Manual, scheduled, off-site, before changes", later: "Single-file and single-player restore" },
  { area: "Plugins", now: "Install, update and remove with dependencies", later: "Upgrade planner, staging clones" },
  { area: "Crashes", now: "Plain-language explanation and a suggested fix", later: "Lag sources and performance view" },
  { area: "Settings", now: "Forms for the most-used plugins", later: "Forms for more plugins, from shared schemas" },
  { area: "Platforms", now: "Windows and Linux", later: "macOS and ARM" },
];

export const faqs = [
  {
    q: "Is it really free?",
    a: "Yes. The Community edition is free, has no server or node limit, and its source is open under the AGPL-3.0 licence. Paid plans add things that save time or help teams and hosts. Anything you need to run a server safely, like backups and two-factor login, stays free.",
  },
  {
    q: "Can I use it yet?",
    a: "No. Consolry is in development and nothing has been released. Join the waitlist and you'll get one email when the first release is ready.",
  },
  {
    q: "Can I run it on Windows without Docker?",
    a: "Yes. On Windows, Consolry runs game servers as normal programs, so there is nothing extra to install. That mode is meant for your own servers, not for selling hosting to strangers. On Linux, servers run in containers.",
  },
  {
    q: "Which games does it support?",
    a: "Minecraft first: Paper, Purpur, Fabric and NeoForge. Other games will arrive as separate modules after the first release.",
  },
  {
    q: "Is this a fork of Pterodactyl?",
    a: "No. Consolry is its own panel with its own node software, written from scratch. A one-time importer for existing Pterodactyl servers is planned for after the first release.",
  },
  {
    q: "Where do plugins and mods come from?",
    a: "Modrinth, Hangar and CurseForge, installed from inside the panel. Plugins from anywhere else, including SpigotMC, can be uploaded by hand.",
  },
  {
    q: "What happens if I stop paying?",
    a: "The paid features switch off. Your servers keep running and nothing is deleted.",
  },
  {
    q: "Does my server data leave my machine?",
    a: "No. Consolry runs on your own hardware. Nothing is sent anywhere unless you turn on a feature that needs it, and that feature says what it sends before you enable it.",
  },
];
