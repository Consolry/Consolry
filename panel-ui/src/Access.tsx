import { useEffect, useState } from "react";
import { api, type PanelSettings, type RemoteMode } from "./api";
import { ErrorNote, useAction } from "./ui";

const modes: { id: RemoteMode; label: string; about: string }[] = [
  { id: "off", label: "Only this computer", about: "The panel opens only in a browser on the machine it runs on." },
  { id: "network", label: "My home network", about: "Phones and computers on the same Wi-Fi can open the panel." },
  {
    id: "internet",
    label: "Anywhere",
    about: "Consolry asks your router to open the panel's port, so you can reach it from outside your home.",
  },
];

/** Panel-wide settings for the admin: where the panel can be opened from, and who may make accounts. */
export default function Access() {
  const [settings, setSettings] = useState<PanelSettings | null>(null);
  const { error, busy, run } = useAction();

  useEffect(() => {
    api.panelSettings().then(setSettings, () => {});
  }, []);

  if (!settings) return null;
  const { remote } = settings;
  const change = (patch: { signup?: boolean; remote?: RemoteMode }) => run("save", async () => setSettings(await api.updatePanelSettings(patch)));
  const best = remote.internetAddress || remote.networkAddress;

  return (
    <>
      <header className="page-head">
        <h1>Remote access</h1>
      </header>
      <p className="dim lead">Choose where the panel can be opened from. It always works on this computer.</p>
      <ErrorNote message={error} />

      <section className="card form wide">
        <h2>Open the panel from</h2>
        <fieldset className="permissions" disabled={busy !== ""}>
          <legend className="sr-only">Where the panel can be opened from</legend>
          {modes.map((mode) => (
            <label key={mode.id} className="check">
              <input type="radio" name="remote" checked={remote.mode === mode.id} onChange={() => change({ remote: mode.id })} />
              <span>
                <strong>{mode.label}</strong>
                <small>{mode.about}</small>
              </span>
            </label>
          ))}
        </fieldset>
        {busy !== "" && <p className="dim">Working… asking the router can take a few seconds.</p>}

        {remote.mode !== "off" && (
          <dl className="addresses">
            {remote.networkAddress && (
              <div>
                <dt>On your home network</dt>
                <dd>
                  <code>{remote.networkAddress}</code>
                </dd>
              </div>
            )}
            {remote.internetAddress && (
              <div>
                <dt>From anywhere</dt>
                <dd>
                  <code>{remote.internetAddress}</code>
                </dd>
              </div>
            )}
          </dl>
        )}
        {remote.problem && <p className="note warn">{remote.problem}</p>}
        {remote.mode !== "off" && (
          <p className="dim">
            The first time, Windows may ask whether to allow Consolry through the firewall. Choose <strong>Allow</strong>, or other devices will not connect.
          </p>
        )}
        {remote.mode === "internet" && (
          <p className="note warn">
            The connection is not encrypted, and anyone who finds the address can reach the sign-in page. Use a long password you use nowhere else, and switch
            this off when you do not need it.
          </p>
        )}
      </section>

      {best && (
        <section className="card form wide">
          <h2>Put Consolry on your phone</h2>
          <p className="dim">
            Open <code>{best}</code> in your phone's browser and sign in. Then:
          </p>
          <ul className="steps">
            <li>
              <strong>iPhone:</strong> in Safari, tap Share, then <strong>Add to Home Screen</strong>.
            </li>
            <li>
              <strong>Android:</strong> in Chrome, tap the menu, then <strong>Add to Home screen</strong>.
            </li>
          </ul>
          <p className="dim">Consolry then has its own icon and opens full screen, like an app.</p>
        </section>
      )}

      <section className="card form wide">
        <h2>Accounts</h2>
        <label className="check">
          <input type="checkbox" checked={settings.signup} disabled={busy !== ""} onChange={(event) => change({ signup: event.target.checked })} />
          <span>
            <strong>Let people create their own account</strong>
            <small>
              A new account can do nothing until a server is shared with it from that server's Users tab. Switch this off once everyone you want has signed up.
            </small>
          </span>
        </label>
      </section>
    </>
  );
}
