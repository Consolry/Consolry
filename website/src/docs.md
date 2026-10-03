=== | Start here | Getting started | What Consolry is, what you need, and where to begin.

Consolry is a game server panel you install on your own computer. It runs Minecraft servers and gives you a web page to manage them: a console, files, plugins, players, backups and schedules.

> **Consolry is a pre-release.** It works, but it is not finished. Expect bugs and missing features, and keep your own copy of any world you care about.

## What you need

- A computer running **Windows 10 or 11**, or **64-bit Linux** (Intel, AMD or ARM).
- About 2 GB of free memory for each Minecraft server, and a few GB of disk space.
- An internet connection, to download Minecraft and plugins.

You do **not** need to install Java, Docker or a database. Consolry downloads the right Java for each server and keeps its own data in one folder.

## The short version

1. Install Consolry: [on Windows](/docs/install-windows) or [on Linux](/docs/install-linux).
2. Open the panel in your browser and create your admin account.
3. [Create your first server](/docs/first-server).
4. [Open a port](/docs/network) if friends outside your home should be able to join.

## How it fits together

Consolry is one program. It serves the panel at `http://127.0.0.1:8700` and runs your servers on the same machine. Everything stays on that computer: your account, your worlds and your backups.

=== install-windows | Start here | Install on Windows | Download one file, run it, and create your account.

## Install

1. Download **Consolry.exe** from the [download page](https://www.consolry.com/download).
2. Run it. Your browser opens the panel at `http://127.0.0.1:8700`.
3. Create your admin account. The password needs at least 10 characters.

Consolry has no installer and no setup wizard. The file you downloaded is the whole program.

## "Windows protected your PC"

Windows shows this for programs that are not code-signed, and the pre-release is not signed yet. Choose **More info**, then **Run anyway**.

## Where it runs

Consolry has no window. It sits in the notification area, next to the clock, as a blue icon. You may need to click the `^` arrow to see it. Right-click the icon for:

- **Open Consolry**: shows the panel in your browser.
- **Start when I sign in**: tick this so your servers come back after a restart.
- **Quit and stop servers**: saves and closes every server, then exits.

Running the file a second time does not start a second copy. It just opens the panel.

## Where your data is kept

In `%LOCALAPPDATA%\Consolry`, which is usually `C:\Users\you\AppData\Local\Consolry`. See [Your data](/docs/data).

Keep `Consolry.exe` somewhere it can stay, such as that same folder. If you tick "Start when I sign in" and later move the file, untick it and tick it again.

=== install-linux | Start here | Install on Linux | One command installs it. A second sets it up to run in the background.

## Install

Run this in a terminal:

```
curl -fsSL https://www.consolry.com/install.sh | sh
```

It downloads the newest pre-release for your processor, checks it against its published fingerprint, and installs a single program called `consolry`. It does not need root: without root it installs to `~/.local/bin`.

## Start it

```
consolry
```

Then open `http://127.0.0.1:8700` in a browser on that machine and create your admin account.

## Run it in the background

To keep Consolry running after you close the terminal, and start it when you sign in:

```
consolry -autostart on
```

This sets up a systemd service for your user. To keep it running while you are signed out, which you want on a server, also run once:

```
sudo loginctl enable-linger $USER
```

Turn it off again with `consolry -autostart off`. Read its log with `journalctl --user -u consolry -f`.

## No screen on that machine?

The panel only answers on the machine it runs on. From your own computer, open a tunnel:

```
ssh -L 8700:127.0.0.1:8700 you@your-server
```

Leave that running and open `http://127.0.0.1:8700` on your own computer.

Do not put the panel directly on the internet in this pre-release. It has no HTTPS of its own yet.

=== first-server | Start here | Your first server | Pick the software, a version and how much memory. Consolry does the rest.

## Create it

1. In the panel, choose **New server**.
2. Give it a name.
3. Pick the **server software**:
   - **Paper**: runs plugins and is fast. The usual choice.
   - **Purpur**: Paper with extra settings.
   - **Fabric**: runs mods instead of plugins.
4. Pick the **Minecraft version** and how much **memory** it may use.
5. Tick the box to accept the Minecraft EULA. A Minecraft server will not start without it.
6. Choose **Create server**.

Consolry downloads the server, checks the download, and installs a suitable Java if your computer does not have one.

## Start it

Open the server and press **Start**. The status reads **Starting** with the latest line from the log, then **Running** once Minecraft is ready. A new server usually takes 20 to 40 seconds; one with many plugins takes longer.

## Join it

On the same computer, add a server in Minecraft with the address `localhost`. For other devices and for friends, see [Network](/docs/network).

## How much memory?

2 GB is enough for a few friends on a plain server. Add more for many plugins or players, but leave a few GB free for the rest of your computer. You can change it later on the [Startup](/docs/startup) tab.

=== import | Start here | Moving from Pterodactyl | Copy servers from a Pterodactyl panel, with their worlds, plugins and settings.

Consolry can copy servers from a Pterodactyl panel. Nothing on Pterodactyl is changed, so you can check the copy before switching over.

## Before you start

- In Pterodactyl, open **Account**, then **API Credentials**, and create a key. It starts with `ptlc_`. It is used only for the import and is not saved.
- **Stop the server on Pterodactyl**, so its world is saved and not changing while it is copied.
- Make sure this computer has room for the server's files.

## Import

1. In Consolry, open **New server** and choose **Import a server**, or go to `#/import` in the panel.
2. Type your Pterodactyl address and the API key, then press **Show my servers**.
3. Press **Import** next to a server.

Pterodactyl packs the server's files, Consolry downloads and unpacks them, then the packed copy is deleted from Pterodactyl again. A big world can take several minutes.

## What is carried over

| From Pterodactyl | In Consolry |
| --- | --- |
| Every file: world, plugins, settings | The same files in the new server's folder |
| Memory limit | The server's memory |
| Java version of the Docker image | The Java version Consolry uses |
| Paper, Purpur or Fabric egg | A Minecraft server with plugin tools and the version switcher |
| Any other egg | Pterodactyl's start command, which may need changing on the [Startup](/docs/startup) tab |

Not carried over: Pterodactyl's sub-users, schedules, databases and backups. Share the server again from the [Users tab](/docs/users), and add [schedules](/docs/schedules) here.

## After importing

Start the server and check the console. If the egg said "latest" for the version, Consolry reads the real version from the server's log; when it cannot, it says so and you can pick it under **Settings**.

=== console | Managing a server | Console | Watch the server's log live and type commands.

The Console tab shows what the server prints, as it happens, and lets you send commands.

## Sending commands

Type a command and press Enter. Leave out the `/` you would type in the game: `say hello`, not `/say hello`. The up and down arrows step through commands you sent before.

## Colours

- **Time** is dimmed, and the level is coloured: INFO blue, WARN yellow, ERROR red.
- **Plugin names in square brackets** show how that plugin is doing in this run: green once it has enabled, yellow if it has warned, red if it has logged an error or failed to load, grey if it was disabled.
- Where the server colours text itself, such as the plugin list from `plugins`, those colours are kept.

## Following and searching

The console stays on the newest line. Scroll up to read back and it stops following; a yellow button shows how many new lines are waiting and jumps back down. The search box filters the log to lines containing your text.

The console keeps the last 2,000 lines. Older lines are in the server's `logs` folder, under [Files](/docs/files).

## When a server stops unexpectedly

A **What went wrong** box appears above the console when Consolry recognises the cause, with what to do about it. It covers common cases: the EULA not accepted, the port already in use, running out of memory, Java too old, a plugin missing something it depends on, and a plugin failing as it starts. It only recognises known patterns, so no box does not mean nothing went wrong.

=== files | Managing a server | Files | Browse, edit, upload and download a server's files.

The Files tab shows the server's own folder.

- **Open a folder** by clicking it. The path at the top takes you back up.
- **Edit a text file** by clicking it. Change it, then **Save**.
- **Upload** files from your computer into the folder you are looking at.
- **New file** and **New folder** create empty ones.
- **Download**, **Rename** and **Delete** are on each row. Deleting a folder deletes everything in it.

Large files and files that are not text, such as `.jar` files and world data, download instead of opening in the editor.

Most settings files are only read when the server starts, so restart the server after changing one. For the common Minecraft settings, the [Game settings](/docs/game-settings) tab is easier than editing `server.properties` by hand.

A server can only reach its own folder. Nothing in the Files tab can read or change files anywhere else on your computer.

=== plugins | Managing a server | Plugins and mods | Find, install, update and remove plugins from inside the panel.

On Paper and Purpur servers this tab is called **Plugins**. On Fabric servers it is **Mods**.

## Browse and install

**Browse** shows the most downloaded plugins that have a version for your server, 16 to a page. Use the search box to find one by name. Press **Install** on a card.

- Anything the plugin requires is installed with it.
- A [backup](/docs/backups) is taken first.
- If the server is running, restart it to load the new plugin.

## Where plugins come from

Pick a site above the results:

| Site | What it has |
| --- | --- |
| [Modrinth](https://modrinth.com) | Plugins and Fabric mods. The default. |
| [Hangar](https://hangar.papermc.io) | PaperMC's own plugin site. Paper and Purpur only. |
| [CurseForge](https://www.curseforge.com/minecraft) | Plugins and mods. Needs an API key, see below. |

Every download is checked against the fingerprint the site publishes, and refused if it does not match. A few authors only allow downloads from their own page; those cannot be installed from Consolry. Only full releases are installed or offered as updates, never test builds.

A plugin from anywhere else can be uploaded into the `plugins` folder on the [Files](/docs/files) tab.

## Connecting CurseForge

CurseForge only answers programs that have a key. Keys are free:

1. Sign in at [console.curseforge.com](https://console.curseforge.com) and open **API keys**.
2. Copy your key.
3. In Consolry, open any server's **Plugins** tab, choose **CurseForge**, paste the key and press **Save key**. Only the admin can do this, and it is needed once for the whole panel.

## Installed

**Installed** lists what is in the server's plugins folder.

- **Verified** means the file is exactly what its author published, and shows which site it came from.
- **Unverified** means Consolry cannot match it. That is normal for plugins uploaded by hand. It is not a sign that the file is harmful, only that it could not be checked.
- **Update** appears when a newer version exists for your server.
- **Remove** deletes the plugin's file. Its settings folder stays.

=== players | Managing a server | Players | See who is online, and manage the whitelist, operators and bans.

The Players tab is available on Minecraft servers.

- **Online** lists who is connected. Each player has **Make operator**, **Kick** and **Ban**.
- **Whitelist**: when it is on, only listed players can join. Add a name and turn it on before you share your address with anyone.
- **Operators** can run every command in the game. Give this to people you trust.
- **Banned** lists banned players, with **Unban**.

The lists are read from the server's files, so you can see them while it is off. Adding, removing, kicking and banning go through the game itself, so they need the server to be running.

=== game-settings | Managing a server | Game settings | Change the common Minecraft settings in a form.

This tab edits `server.properties` for you. It shows the settings most people change: the message in the server list, player limit, game mode, difficulty, whitelist, view distance, port and more.

Change what you want. Before you save, a box lists exactly which lines will change. Press **Save game settings**, then restart the server.

Only settings your version of Minecraft has are shown. Everything else in the file is left exactly as it is; edit those on the [Files](/docs/files) tab.

If the tab says to start the server once, do that. Minecraft creates its settings file on the first run.

Leave **Check players own Minecraft** on unless you know why you need it off. Turning it off lets anyone join under any name.

=== backups | Managing a server | Backups | Keep copies of a server and restore one when something goes wrong.

A backup is a copy of every file in the server's folder: the world, plugins and settings.

## Taking one

Press **Back up now**. For a clean copy, stop the server first. A backup taken while it runs may catch the world mid-save, and files the running server has locked are left out; the panel tells you which.

## Automatic backups

- **Before a change.** Consolry takes one before installing or updating a plugin and before switching version. The newest 5 are kept.
- **On a schedule.** Add a backup [schedule](/docs/schedules). The newest 7 are kept.

Backups you take by hand are never deleted automatically.

## Restoring

Stop the server, then press **Restore** on a backup. Every file on the server is replaced with the backup's copy, so anything made since then is lost.

## Where they are kept

On the same computer, in Consolry's [data folder](/docs/data). That protects you from a bad plugin or a mistake, but not from a failed disk. Press **Download** on a backup to keep a copy somewhere else, or copy them off-site automatically.

## Off-site copies

Consolry can copy backups to storage that works like Amazon S3: [Backblaze B2](https://www.backblaze.com/cloud-storage), [Cloudflare R2](https://www.cloudflare.com/developer-platform/r2/), Wasabi, Amazon S3, or your own MinIO.

1. The admin opens **Storage** in the menu, fills in the bucket's address, name and keys, and presses **Save and check**. Consolry writes a small test file to make sure the details work.
2. On a server's **Backups** tab, tick **Copy every backup off-site**.

From then on, every backup taken by hand or by a schedule is copied as soon as it is made, and the oldest copies beyond the number to keep are deleted. Backups taken automatically before a change stay on this computer only. A failed copy is written in the server's activity log, and sent as an [alert](/docs/alerts) if task alerts are on.

The **Off-site copies** list has **Download** and **Restore** for each copy. Restore brings the copy back to this computer, then restores it like any other backup.

> Most providers charge for storage by the gigabyte. A Minecraft world is usually a few hundred megabytes; check how big your backups are before choosing how many to keep.

Folders that Minecraft downloads again by itself, such as `libraries` and `cache`, are left out to keep backups small.

=== schedules | Managing a server | Schedules | Restart, back up or run a command automatically.

A schedule does something to a server at set times.

## What it can do

- Restart, start or stop the server.
- Take a backup.
- Run a console command, such as `say The server restarts in 5 minutes`.

## When

Every day at a time, every week on a day and time, or every few hours. Times use the clock of the computer Consolry runs on.

## Things to know

- Schedules only run while Consolry is running. Set it to [run in the background](/docs/background).
- A daily or weekly run that was missed because the computer was off is skipped, not run late.
- A console command only works while the server is running.
- **Run now** tries a schedule straight away. **Pause** keeps it without running it.

A nightly restart and a daily backup are a good start.

=== network | Managing a server | Network and ports | Let other devices and friends outside your home join.

The Network tab shows how players reach the server.

## On your home network

Other devices on the same Wi-Fi join with the address shown under **On your home network**, such as `192.168.1.20`. If the port is not 25565, they add it: `192.168.1.20:25566`.

## Friends outside your home

Your router blocks connections from the internet until you open the server's port. Press **Open the port for me**. Consolry asks the router to forward the port and shows the address your friends should use.

That address is your home's public address. Share it only with people you trust, and turn on the whitelist on the [Players](/docs/players) tab first.

Consolry reopens the port each time the server starts. **Close the port** removes it.

## If the router refuses

- **"Your router already has a rule for port 25565"**: a rule was added by hand in the router's own settings. Open the router's port forwarding page. If the rule points at this computer, the port is already open and you need nothing else. Otherwise change or delete it, or give the server a different port.
- **"Your router did not answer"**: the router has UPnP switched off. Turn it on in the router's settings, or forward the port there by hand: TCP, the server's port, to this computer's address.
- **"Its own address is not a public one"**: your internet provider shares one address between customers. Forwarding cannot work; you would need a tunnel service.

## Changing the port

Stop the server, enter a new port, and save. Use a different port for each server you run at the same time.

=== alerts | Managing a server | Alerts | Get a message in Discord or by email when something happens to a server.

Each server's owner chooses its alerts on the server's **Alerts** tab.

## Where alerts go

- **Discord:** in your Discord server, open **Server settings**, **Integrations**, **Webhooks**, make a **New webhook** for a channel, and **Copy webhook URL**. Paste it into Consolry.
- **Email:** type up to five addresses. Email needs the panel's mail server, which the admin sets up once (see below).

Press **Save and send a test** to check it works.

## What you can be told about

| Choice | When |
| --- | --- |
| It crashes | The server stops without being asked to. The message includes what went wrong when the [crash explainer](/docs/troubleshooting) recognises it. |
| A scheduled task fails | A backup, restart or command from a [schedule](/docs/schedules) did not work, or a backup could not be copied off-site. |
| It starts | The server is ready for players. |
| It is stopped | Someone, or a schedule, stopped it. |

Crashes and failed tasks are on from the start; the other two are off.

## Setting up email (admin)

Open **Alerts** in the menu and fill in **Mail server** with the details of an email account you already have. For Gmail, use `smtp.gmail.com`, port `587`, your address, and an [app password](https://myaccount.google.com/apppasswords) rather than your normal password. Services such as Brevo or Mailgun also work.

## Alerts about the panel (admin)

The same page has alerts for the panel itself: when a new version of Consolry is out, and when a machine stops answering for more than a minute.

=== startup | Managing a server | Startup | Memory, Java options and how the server is launched.

## Memory

How much memory Java may use. Stop the server to change it. Leave a few GB free for the rest of your computer.

## Extra Java options

Advanced. Consolry sets the memory and the server file itself; anything you add here goes between them.

- **Add the faster-startup option** makes Java remember what it loaded and reuse it next time. It measured about 8% faster. It takes effect after one clean stop.
- **Use Paper's recommended options** fills in the set Paper suggests for your version.

## Stop command

The command typed into the console to shut the server down cleanly. For Minecraft it is `stop`.

## Which Java is used

The page says which Java the server needs and which one will run it. If your computer's own Java is new enough, that is used. Otherwise Consolry uses a copy it downloaded for itself, kept in its data folder. It never changes the Java installed on your computer.

=== users | Managing a server | Users and permissions | Share a server with other people and choose what each one can do.

The **Users** tab lets other people help run a server without giving them your password. Only the server's owner and the panel's admin see this tab.

## Invite someone

1. Ask them to open your panel and choose **Create an account** on the sign-in page. They need to be able to reach the panel, so see [Remote access](/docs/remote-access) first.
2. Ask for the username they picked.
3. Open the server, go to **Users**, type their username and tick what they may do.
4. Press **Invite**. The server appears in their panel straight away.

## What each permission allows

| Permission | Lets them |
| --- | --- |
| Console | Read the console and type commands |
| Start and stop | Start, stop and kill the server |
| Files | Open, change, upload and delete the server's files |
| Plugins and mods | Install and update them |
| Players | Kick, ban, whitelist and make operators |
| Settings | Change game settings, the version, memory and start-up options |
| Backups | Make, download, restore and delete backups |
| Schedules | Add, change and run scheduled tasks |
| Network | See the address and open or close the port |
| Activity | See who did what on the server |

Everyone you invite can see the server's dashboard. With nothing ticked, that is all they can do.

> **Console** is a powerful permission: someone who can type commands can make themselves an operator. **Files** is too, because it includes the server's settings files. Give both only to people you trust.

## Change or remove someone

Press **Change** next to their name to tick different boxes, or **Remove** to take the server away from them. Both take effect at once.

## What invited people cannot do

They cannot delete the server, invite others, add machines or change the panel's own settings. Those stay with the owner and the admin. Creating servers is separate: see [Accounts and their servers](/docs/accounts).

## Stop new sign-ups

Once everyone has an account, the admin can untick **Let people create their own account** on the **Remote access** page. A new account can do nothing until a server is shared with it.

=== account-security | Running Consolry | Passwords and two-factor login | Change your password, and protect your account with a code from your phone.

Click your name at the top right of the panel to open **Your account**.

## Change your password

Type your current password and a new one of at least 10 characters. Every other device signed in to your account is signed out.

## Two-factor login

With two-factor login on, signing in needs your password and a six-digit code from an app on your phone. Someone who learns your password still cannot get in. Turn it on if your panel can be reached from the internet.

1. Install an authenticator app, such as Google Authenticator, Microsoft Authenticator or 2FAS.
2. On **Your account**, press **Set it up** and scan the QR code with the app.
3. Type the code the app shows and press **Switch on**.
4. Save the eight backup codes somewhere safe. Each one signs you in once if you lose your phone.

## Locked out?

- **Lost your phone:** sign in with your password and one of your backup codes.
- **Someone else is locked out:** the admin opens **Accounts**, presses **Locked out?** next to their name, and sets a new password, switches off their two-factor login, or both. They are signed out everywhere.
- **The admin is locked out:** reset the password on the computer itself; see [Troubleshooting](/docs/troubleshooting).

Eight wrong passwords or codes from one address lock that address out for 15 minutes.

=== background | Running Consolry | Running in the background | Keep your servers up after you close the window or restart.

Your servers only run while Consolry runs.

## Windows

Consolry runs from an icon in the notification area and has no window to close by accident. Right-click the icon and tick **Start when I sign in** to bring it back after a restart.

## Linux

```
consolry -autostart on
sudo loginctl enable-linger $USER
```

The first line installs a background service for your user. The second keeps it running while you are signed out.

## Stopping safely

Use **Quit and stop servers** on Windows, or stop the service on Linux. Consolry tells each server to stop and waits for it to save before exiting. Ending the program from Task Manager, or pulling the power, skips that and can lose recent changes to a world.

## After a restart

Consolry comes back, but your servers stay off until you start them. To start one automatically, add a [schedule](/docs/schedules) or start it from the panel.

=== data | Running Consolry | Your data | Where Consolry keeps everything, and how to back it up, move it and remove it.

## Where it is

- **Windows:** `%LOCALAPPDATA%\Consolry`
- **Linux:** `~/.local/share/consolry`

Inside:

| Item | What it holds |
| --- | --- |
| `consolry.db` | Your account, server list, schedules and activity log |
| `daemon-data/servers` | Each server's files, including its world |
| `daemon-data/backups` | Backups |
| `daemon-data/java` | Any Java that Consolry downloaded |
| `consolry.log` | Consolry's own log |

To use a different folder, start Consolry with `-dir "D:\Consolry"`.

Do not keep this folder inside OneDrive, Dropbox or similar. They upload world files while the server is using them, which slows the server and can damage the world.

## Backing it all up

Quit Consolry, then copy the whole folder.

## Moving to another computer

Quit Consolry, copy the folder to the same place on the new computer, and start Consolry there.

## Updating

Quit Consolry, replace the program file with the newer one, and start it again. Your data is not touched. On Linux, run the install command again.

## Removing it

Quit Consolry and untick **Start when I sign in**, or run `consolry -autostart off` on Linux. Then delete the program file and the data folder. Deleting the data folder deletes your worlds and backups.

=== remote-access | Running Consolry | Remote access | Open the panel from your phone, another computer, or outside your home.

A new install only answers on the computer it runs on, at `http://127.0.0.1:8700`. The **Remote access** page, in the left menu, changes that. Only the admin sees it.

## The three choices

| Choice | Who can open the panel |
| --- | --- |
| Only this computer | A browser on the same machine. This is the default. |
| My home network | Phones and computers on the same Wi-Fi or network. |
| Anywhere | Any device on the internet. Consolry asks your router to open the panel's port. |

After you choose, the page shows the address to use from other devices, such as `http://192.168.1.20:8700` at home or `http://203.0.113.5:8700` from anywhere.

## The Windows firewall question

The first time you choose something other than "Only this computer", Windows asks whether to allow Consolry through the firewall. Choose **Allow**. If you closed the question, or other devices cannot connect:

1. Open **Windows Security**, then **Firewall & network protection**.
2. Choose **Allow an app through firewall**.
3. Find **Consolry** and tick **Private**.

## Before you choose "Anywhere"

> The connection is not encrypted, and anyone who finds the address can reach your sign-in page. Consolry is also a pre-release. Open it to the internet only if you need to.

- Use a long password that you use nowhere else.
- Eight wrong passwords from one address lock that address out for 15 minutes.
- Switch back to "My home network" when you do not need it.

## If "Anywhere" does not work

The page says why in a sentence. The common reasons are the same as for a game server's port, and so are the fixes: see [Network and ports](/docs/network).

- **Your router did not answer:** switch on UPnP in the router's settings.
- **Your provider shares one public address:** the port opens on your router, but nothing outside your home can reach it. Ask your provider for a public address.

Your home's public address can change when the router restarts. If the address stops working, open the Remote access page at home to see the new one.

## On Linux

It works the same way. If the machine has its own firewall, allow the port, for example `sudo ufw allow 8700/tcp`.

=== phone | Running Consolry | On your phone | Put Consolry on your Android phone or iPhone's home screen.

Consolry does not have an app in the App Store or Google Play. Instead, the panel itself can be added to your phone's home screen. It gets its own icon and opens full screen, without the browser's address bar.

## 1. Let your phone reach the panel

On the computer running Consolry, open **Remote access** and choose **My home network**, or **Anywhere** to use it away from home. The page shows the address to use. See [Remote access](/docs/remote-access).

## 2. Open it and sign in

Type that address into your phone's browser and sign in.

## 3. Add it to your home screen

**iPhone or iPad**, in Safari:

1. Tap the Share button.
2. Tap **Add to Home Screen**, then **Add**.

**Android**, in Chrome:

1. Tap the three-dot menu.
2. Tap **Add to Home screen**, then **Add**.

## Things to know

- The icon opens the address you added. If you added the home-network address, it only works on your home Wi-Fi.
- If your home's public address changes, remove the icon and add it again with the new address.
- Everything the panel does on a computer works on the phone, including the console and file editor.

=== accounts | Running Consolry | Accounts and their servers | See who has an account, and let people create servers of their own.

The **Accounts** page, in the left menu, lists everyone with an account on your panel. Only the admin sees it.

## Let someone create their own servers

A new account cannot create servers. To allow it:

1. Open **Accounts** and find the person.
2. Set **Servers they may create** to a number above 0.
3. Choose the most memory each of their servers may use.
4. Press **Save**. They may need to reload the panel to see **New server**.

They own the servers they create: they can do everything on them, including sharing them from the [Users tab](/docs/users) and deleting them. You, as admin, can still open and manage every server.

## What they can and cannot do

- They can create Minecraft servers only, up to their number, on the first machine.
- They cannot go above their memory allowance, when creating a server or later.
- They cannot create custom-command servers, change a start command, add machines or change panel settings.

> Servers run on your computer and use its memory and disk. A server's plugins can run any program there. Only let people you trust create servers.

## New accounts

At the bottom of the page, **New accounts** sets what an account gets when it signs up. Leave servers at 0 to decide for each person yourself.

## Remove an account

Press **Remove** next to the account. The person is signed out and can no longer sign in. Servers they owned are kept and pass to you.

=== updating | Running Consolry | Updating | Install a new version from inside the panel.

From version 0.3.0, Consolry updates itself. You do not need to download the program again.

## How to update

When a newer version is out, the admin sees a notice at the top of the panel. Press **Update now**, then **Update and restart**.

Consolry then:

1. Downloads the new version and checks it against its published fingerprint.
2. Saves and stops your running servers.
3. Swaps the program and starts again.
4. Starts the servers that were running.

This takes about a minute, and players are disconnected while it happens. The page reloads by itself when Consolry is back. Your servers, worlds, backups and accounts are not touched.

## Good to know

- Consolry looks for a new version about once an hour. It never updates without you pressing the button.
- Updating from 0.1.0 or 0.2.0 has to be done by hand one last time: download the newest version from the [download page](https://www.consolry.com/download) and replace the old file, or run the install command again on Linux.
- If the new version does not come back, start Consolry again yourself. The previous program is kept next to the new one as a file ending in `.old` until the new one has started.

=== licence | Running Consolry | Licence and paid plans | What a licence key is, how to add one, and how to manage a subscription.

Consolry is free to run, with no limit on servers. Paid plans add extra features on top. You always host Consolry yourself; a plan never rents you a server.

> Paid plans are not on sale yet, because their features are still being built. This page explains how it will work.

## Adding a licence key

After paying on the [pricing page](https://www.consolry.com/pricing), you are shown a licence key starting with `CONSOLRY-`. In your panel, open **Licence** in the menu (admin only), paste it and press **Save**.

The key carries the plan and the date it is paid until. Consolry checks it with a signature built into the program, so it works without contacting anyone. Once a day the panel asks consolry.com for a fresh key, which extends the date while the subscription is paid.

## Managing your subscription

Go to [consolry.com/license](https://www.consolry.com/license), paste your licence key under **Manage your subscription**, and press **Open billing**. Stripe's page lets you change your card, download invoices or cancel.

If a subscription ends, the panel keeps working on the free plan a week after the last paid period. Your servers and data are not touched.

=== second-machine | Running Consolry | A second machine | Run servers on another computer from the same panel.

One panel can manage servers on more than one computer. Each extra computer runs a small program, the daemon, and appears in the panel as a **node**.

1. Download `consolry-daemon` for that computer from the release files.
2. Start it there. The first time, it prints a **token**.
3. In the panel, open **Nodes** and choose **Add another machine**. Enter a name, the daemon's address and the token.
4. When you create a server, choose which node it runs on.

The daemon listens on `127.0.0.1:8750` by default, which only the same computer can reach. To reach it from the panel's computer, start it with `-listen` and an address on your private network.

The link between the panel and a daemon is not encrypted in this pre-release. Only use it across a network you trust, never across the internet.

Most people do not need this. The panel already runs servers on the computer it is installed on.

=== troubleshooting | Help | Troubleshooting | Fixes for the problems people hit most.

## The panel will not open

Consolry is probably not running. On Windows, look for its icon by the clock; if it is missing, run `Consolry.exe`. On Linux, run `consolry`, or check the service with `systemctl --user status consolry`.

## A server will not start

Open its Console tab. If a **What went wrong** box appears, follow it. The usual causes:

- **The port is already in use.** Another server is using the same port. Stop it, or change this server's port on the [Network](/docs/network) tab.
- **Java is too old or missing.** Consolry normally installs what is needed. If that failed, the message says which version to install.
- **A plugin is missing something it depends on.** Install the plugin it names, or remove the one that needs it.

## A server takes a long time to start

It shows **Starting** with its latest log line while it loads. Plugins are usually what takes the time: each one adds to it. A server with 30 plugins can take a minute. See also the faster-startup option on the [Startup](/docs/startup) tab, and keep the [data folder](/docs/data) out of OneDrive.

## Friends cannot join

Work through the [Network](/docs/network) page. Check the server is running, the port is open on your router, and they are using your public address, with the port added if it is not 25565.

## The console stops updating

Refresh the page with Ctrl+F5. If it still stops, note whether the light at the top of the console says **Live** or **Reconnecting**, and report it.

## I forgot my password

If you are not the admin, ask the admin: on **Accounts** they press **Locked out?** next to your name and set a new password.

If you are the admin, reset it on the computer Consolry runs on. This also switches off two-factor login for the account.

- **Windows:** open PowerShell in the folder with `Consolry.exe` (usually `%LOCALAPPDATA%\Consolry`) and run `.\Consolry.exe -reset-password yourname`
- **Linux:** run `consolry -reset-password yourname`

It prints a new password. Sign in with it, then change it on **Your account**.

## Reporting a problem

Open an issue on [GitHub](https://github.com/Consolry/Consolry/issues). Include what you did, what happened, and the last lines of `consolry.log` from the [data folder](/docs/data).
