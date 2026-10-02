import { useState, type FormEvent } from "react";
import { api, type User } from "./api";
import { ErrorNote, useAction } from "./ui";

function PasswordForm() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [done, setDone] = useState(false);
  const { error, busy, run } = useAction();

  function submit(event: FormEvent) {
    event.preventDefault();
    setDone(false);
    run("password", () => api.changePassword(current, next), () => {
      setCurrent("");
      setNext("");
      setDone(true);
    });
  }

  return (
    <form className="card form wide" onSubmit={submit}>
      <h2>Password</h2>
      <label>
        Current password
        <input type="password" value={current} onChange={(event) => setCurrent(event.target.value)} autoComplete="current-password" required />
      </label>
      <label>
        New password
        <input type="password" value={next} onChange={(event) => setNext(event.target.value)} autoComplete="new-password" minLength={10} required />
        <small>At least 10 characters. Every other device signed in to your account is signed out.</small>
      </label>
      <ErrorNote message={error} />
      {done && <p className="note">Your password has been changed.</p>}
      <button className="primary" disabled={busy !== ""}>
        Change password
      </button>
    </form>
  );
}

function TwoFactor({ user, onChanged }: { user: User; onChanged: () => void }) {
  const [setup, setSetup] = useState<{ secret: string; qr: string } | null>(null);
  const [code, setCode] = useState("");
  const [codes, setCodes] = useState<string[] | null>(null);
  const [password, setPassword] = useState("");
  const { error, busy, run } = useAction();

  if (codes) {
    return (
      <section className="card form wide">
        <h2>Two-factor login is on</h2>
        <p>
          Keep these backup codes somewhere safe, such as a password manager or on paper. Each one signs you in once if you lose your phone. They are not shown
          again.
        </p>
        <ul className="recovery">
          {codes.map((item) => (
            <li key={item}>
              <code>{item}</code>
            </li>
          ))}
        </ul>
        <button
          className="primary"
          onClick={() => {
            setCodes(null);
            onChanged();
          }}
        >
          I have saved them
        </button>
      </section>
    );
  }

  if (user.twoFactor) {
    return (
      <form className="card form wide" onSubmit={(event) => (event.preventDefault(), run("off", () => api.disableTwoFactor(password), onChanged))}>
        <h2>Two-factor login</h2>
        <p className="note">On. Signing in asks for a code from your authenticator app.</p>
        <label>
          Your password, to switch it off
          <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required />
        </label>
        <ErrorNote message={error} />
        <button className="danger" disabled={busy !== ""}>
          Switch off two-factor login
        </button>
      </form>
    );
  }

  return (
    <section className="card form wide">
      <h2>Two-factor login</h2>
      <p className="dim">
        Off. With it on, signing in also needs a six-digit code from an app on your phone, such as Google Authenticator, Microsoft Authenticator or 2FAS, so a
        stolen password is not enough.
      </p>
      {!setup ? (
        <button className="primary" disabled={busy !== ""} onClick={() => run("start", async () => setSetup(await api.startTwoFactor()))}>
          Set it up
        </button>
      ) : (
        <form
          className="two-factor"
          onSubmit={(event) => {
            event.preventDefault();
            run("enable", async () => setCodes((await api.enableTwoFactor(setup.secret, code)).recoveryCodes));
          }}
        >
          <img src={setup.qr} alt="QR code for your authenticator app" width={192} height={192} />
          <div className="form">
            <p>1. In your authenticator app, add an account and scan this code.</p>
            <p className="dim">
              Can't scan it? Type this key instead: <code className="secret">{setup.secret}</code>
            </p>
            <label>
              2. Type the six-digit code the app shows
              <input
                value={code}
                onChange={(event) => setCode(event.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9 ]{6,7}"
                required
                autoFocus
              />
            </label>
            <ErrorNote message={error} />
            <button className="primary" disabled={busy !== ""}>
              Switch on
            </button>
          </div>
        </form>
      )}
    </section>
  );
}

/** The signed-in person's own account: their password and two-factor login. */
export default function Account({ user, onChanged }: { user: User; onChanged: () => void }) {
  return (
    <>
      <header className="page-head">
        <h1>Your account</h1>
      </header>
      <p className="dim lead">
        Signed in as <strong>{user.username}</strong>
        {user.admin ? ", the panel's admin." : "."}
      </p>
      <section className="stack">
        <PasswordForm />
        <TwoFactor user={user} onChanged={onChanged} />
      </section>
    </>
  );
}
