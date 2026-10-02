import { useCallback, useEffect, useState } from "react";
import { api, type Account, type Limits } from "./api";
import { ErrorNote, useAction } from "./ui";

const memoryChoices = [1, 2, 4, 6, 8, 12, 16];

function LimitFields({ value, onChange }: { value: Limits; onChange: (next: Limits) => void }) {
  const sizes = memoryChoices.map((gb) => gb * 1024);
  if (!sizes.includes(value.memoryLimitMb)) sizes.push(value.memoryLimitMb);
  sizes.sort((a, b) => a - b);
  return (
    <>
      <label>
        Servers they may create
        <input
          type="number"
          min={0}
          max={100}
          value={value.serverLimit}
          onChange={(event) => onChange({ ...value, serverLimit: Math.max(0, Math.min(100, Number(event.target.value) || 0)) })}
        />
      </label>
      <label>
        Memory per server, at most
        <select value={value.memoryLimitMb} onChange={(event) => onChange({ ...value, memoryLimitMb: Number(event.target.value) })}>
          {sizes.map((mb) => (
            <option key={mb} value={mb}>
              {mb % 1024 === 0 ? `${mb / 1024} GB` : `${mb} MB`}
            </option>
          ))}
        </select>
      </label>
    </>
  );
}

function Row({ account, onChanged }: { account: Account; onChanged: () => void }) {
  const [limits, setLimits] = useState<Limits>({ serverLimit: account.serverLimit, memoryLimitMb: account.memoryLimitMb });
  const [confirming, setConfirming] = useState(false);
  const { error, busy, run } = useAction();
  const changed = limits.serverLimit !== account.serverLimit || limits.memoryLimitMb !== account.memoryLimitMb;

  return (
    <li>
      <div className="member-head">
        <strong>{account.username}</strong>
        <span className="dim grow">
          {account.admin
            ? "Admin · manages the panel and can create any number of servers"
            : `Owns ${account.servers} ${account.servers === 1 ? "server" : "servers"}`}
        </span>
        {!account.admin &&
          (confirming ? (
            <>
              <button className="small danger" disabled={busy !== ""} onClick={() => run("remove", () => api.deleteAccount(account.id), onChanged)}>
                Remove account
              </button>
              <button className="small" onClick={() => setConfirming(false)}>
                Cancel
              </button>
            </>
          ) : (
            <button className="small danger" onClick={() => setConfirming(true)}>
              Remove
            </button>
          ))}
      </div>
      {confirming && <p className="dim">Their servers are kept and pass to you. This cannot be undone.</p>}
      {!account.admin && (
        <div className="limits">
          <LimitFields value={limits} onChange={setLimits} />
          <button className="small primary" disabled={!changed || busy !== ""} onClick={() => run("save", () => api.setAccountLimits(account.id, limits), onChanged)}>
            Save
          </button>
        </div>
      )}
      <ErrorNote message={error} />
    </li>
  );
}

/** The admin's list of accounts, and how many servers each may create for themselves. */
export default function Accounts() {
  const [accounts, setAccounts] = useState<Account[] | null>(null);
  const [defaults, setDefaults] = useState<Limits | null>(null);
  const [saved, setSaved] = useState<Limits | null>(null);
  const { error, busy, run } = useAction();

  const load = useCallback(
    () =>
      api.accounts().then(
        (result) => {
          setAccounts(result.accounts);
          setDefaults(result.newAccounts);
          setSaved(result.newAccounts);
        },
        () => {},
      ),
    [],
  );
  useEffect(() => {
    load();
  }, [load]);

  if (!accounts || !defaults || !saved) return null;
  const changed = defaults.serverLimit !== saved.serverLimit || defaults.memoryLimitMb !== saved.memoryLimitMb;

  return (
    <>
      <header className="page-head">
        <h1>Accounts</h1>
      </header>
      <p className="dim lead">
        Everyone with an account on this panel. Let someone create servers of their own by giving them a number above 0. They own those servers and can
        share them from the Users tab.
      </p>
      <p className="note warn">
        Servers run on this computer and use its memory and disk. A server's plugins can run any program here, so only let people you trust create servers.
      </p>

      <ul className="members">
        {accounts.map((account) => (
          <Row key={`${account.id}-${account.serverLimit}-${account.memoryLimitMb}`} account={account} onChanged={load} />
        ))}
      </ul>

      <section className="card form wide">
        <h2>New accounts</h2>
        <p className="dim">What an account gets when it signs up from now on. Leave servers at 0 to decide for each person yourself.</p>
        <div className="limits">
          <LimitFields value={defaults} onChange={setDefaults} />
          <button className="small primary" disabled={!changed || busy !== ""} onClick={() => run("save", () => api.setNewAccountLimits(defaults), load)}>
            Save
          </button>
        </div>
        <ErrorNote message={error} />
      </section>
    </>
  );
}
