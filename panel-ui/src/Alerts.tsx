import { useEffect, useState, type FormEvent } from "react";
import { api, type AlertSettings, type ServerInfo, type SmtpSettings } from "./api";
import { ErrorNote, useAction } from "./ui";

type Choice = { id: string; label: string; about: string };

const serverEvents: Choice[] = [
  { id: "crash", label: "It crashes", about: "With what went wrong, when Consolry can tell." },
  { id: "task", label: "A scheduled task fails", about: "Such as a backup or restart that did not work." },
  { id: "start", label: "It starts", about: "When it is ready for players." },
  { id: "stop", label: "It is stopped", about: "When someone or a schedule stops it." },
];

const panelEvents: Choice[] = [
  { id: "update", label: "A new version of Consolry is out", about: "Checked every few hours." },
  { id: "node", label: "A machine stops answering", about: "After a minute without a reply, and again when it is back." },
];

function AlertsForm({
  title,
  intro,
  events,
  load,
  save,
}: {
  title: string;
  intro: string;
  events: Choice[];
  load: () => Promise<AlertSettings>;
  save: (alerts: AlertSettings, test: boolean) => Promise<AlertSettings>;
}) {
  const [alerts, setAlerts] = useState<AlertSettings | null>(null);
  const [note, setNote] = useState("");
  const { error, busy, run } = useAction();

  useEffect(() => {
    load().then(setAlerts, () => {});
  }, [load]);
  if (!alerts) return null;

  function submit(test: boolean) {
    setNote("");
    run(test ? "test" : "save", async () => {
      setAlerts(await save(alerts!, test));
      setNote(test ? "Saved, and a test alert was sent. Check Discord or your inbox." : "Saved.");
    });
  }

  return (
    <form
      className="card form wide"
      onSubmit={(event: FormEvent) => {
        event.preventDefault();
        submit(false);
      }}
    >
      <h2>{title}</h2>
      <p className="dim">{intro}</p>
      <label>
        Discord webhook
        <input
          className="mono"
          value={alerts.discord}
          onChange={(event) => setAlerts({ ...alerts, discord: event.target.value })}
          placeholder="https://discord.com/api/webhooks/…"
          spellCheck={false}
        />
        <small>In Discord: Server settings, Integrations, Webhooks, New webhook, then Copy webhook URL.</small>
      </label>
      <label>
        Email
        <input value={alerts.email} onChange={(event) => setAlerts({ ...alerts, email: event.target.value })} placeholder="you@example.com" />
        <small>Up to five addresses, separated by commas. Needs the panel's mail server, which the admin sets up.</small>
      </label>
      <fieldset className="permissions">
        <legend>Send an alert when</legend>
        {events.map((item) => (
          <label key={item.id} className="check">
            <input
              type="checkbox"
              checked={alerts.events.includes(item.id)}
              onChange={(event) =>
                setAlerts({ ...alerts, events: event.target.checked ? [...alerts.events, item.id] : alerts.events.filter((id) => id !== item.id) })
              }
            />
            <span>
              <strong>{item.label}</strong>
              <small>{item.about}</small>
            </span>
          </label>
        ))}
      </fieldset>
      <ErrorNote message={error} />
      {note && <p className="note">{note}</p>}
      <span className="row-actions">
        <button className="primary" disabled={busy !== ""}>
          Save
        </button>
        <button type="button" disabled={busy !== ""} onClick={() => submit(true)}>
          {busy === "test" ? "Sending…" : "Save and send a test"}
        </button>
      </span>
    </form>
  );
}

/** A server's Alerts tab, for its owner. */
export function ServerAlerts({ server }: { server: ServerInfo }) {
  return (
    <AlertsForm
      title="Alerts"
      intro={`Get a message in Discord or by email when something happens to ${server.name}.`}
      events={serverEvents}
      load={() => api.serverAlerts(server.id)}
      save={(alerts, test) => api.setServerAlerts(server.id, alerts, test)}
    />
  );
}

function MailServer() {
  const [smtp, setSmtp] = useState<SmtpSettings | null>(null);
  const [passwordSet, setPasswordSet] = useState(false);
  const [saved, setSaved] = useState(false);
  const { error, busy, run } = useAction();

  useEffect(() => {
    api.panelAlerts().then((result) => {
      setSmtp({ ...result.smtp, port: result.smtp.port || 587, password: "" });
      setPasswordSet(result.smtpPasswordSet);
    }, () => {});
  }, []);
  if (!smtp) return null;

  return (
    <form
      className="card form wide"
      onSubmit={(event) => {
        event.preventDefault();
        setSaved(false);
        run("smtp", () => api.setSmtp(smtp), () => {
          setSaved(true);
          if (smtp.password) setPasswordSet(true);
          setSmtp({ ...smtp, password: "" });
        });
      }}
    >
      <h2>Mail server</h2>
      <p className="dim">
        Email alerts are sent through a mail account you already have. Gmail works with an app password; so do most email providers and services such as
        Brevo or Mailgun.
      </p>
      <div className="pair">
        <label>
          Server
          <input value={smtp.host} onChange={(event) => setSmtp({ ...smtp, host: event.target.value })} placeholder="smtp.gmail.com" spellCheck={false} />
        </label>
        <label>
          Port
          <input type="number" value={smtp.port} onChange={(event) => setSmtp({ ...smtp, port: Number(event.target.value) })} />
        </label>
      </div>
      <div className="pair">
        <label>
          Username
          <input value={smtp.username} onChange={(event) => setSmtp({ ...smtp, username: event.target.value })} autoComplete="off" spellCheck={false} />
        </label>
        <label>
          Password
          <input
            type="password"
            value={smtp.password ?? ""}
            onChange={(event) => setSmtp({ ...smtp, password: event.target.value })}
            placeholder={passwordSet ? "Saved. Type to change it" : ""}
            autoComplete="new-password"
          />
        </label>
      </div>
      <label>
        Send from
        <input value={smtp.from} onChange={(event) => setSmtp({ ...smtp, from: event.target.value })} placeholder="alerts@example.com" />
      </label>
      <ErrorNote message={error} />
      {saved && <p className="note">Saved. Use "Save and send a test" below to check it.</p>}
      <button className="primary" disabled={busy !== ""}>
        Save mail server
      </button>
    </form>
  );
}

/** The admin's Alerts page: the mail server, and notices about the panel itself. */
export default function PanelAlerts() {
  return (
    <>
      <header className="page-head">
        <h1>Alerts</h1>
      </header>
      <p className="dim lead">Each server's owner chooses that server's alerts on its Alerts tab. Here you set up email, and alerts about the panel itself.</p>
      <section className="stack">
        <MailServer />
        <AlertsForm
          title="About the panel"
          intro="Where to tell you about Consolry itself."
          events={panelEvents}
          load={() => api.panelAlerts().then((result) => result.alerts)}
          save={(alerts, test) => api.setPanelAlerts(alerts, test)}
        />
      </section>
    </>
  );
}
