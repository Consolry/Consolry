import { comparisons, platforms, stack, support } from "../content";

export function Architecture() {
  return (
    <div
      className="arch"
      role="img"
      aria-label="Your browser talks to the panel over HTTPS and WebSocket. The panel stores data in SQLite or PostgreSQL and talks to one daemon on each machine. On Linux the daemon runs each server in a container; on Windows it runs each server as a process."
    >
      <div className="arch-node">
        <p className="pixel">You</p>
        <h3>Browser</h3>
        <p>The React interface, or your own scripts through the API.</p>
      </div>
      <p className="arch-link">HTTPS + WebSocket</p>
      <div className="arch-node main">
        <p className="pixel">One binary</p>
        <h3>Panel</h3>
        <p>Users, roles, schedules, backups and the game modules.</p>
        <p className="arch-db">SQLite or PostgreSQL</p>
      </div>
      <p className="arch-link">HTTPS + WebSocket</p>
      <div className="arch-group">
        <p className="pixel">Daemon, one per machine</p>
        <div className="arch-node">
          <h3>Linux</h3>
          <p>Each server in its own container.</p>
        </div>
        <div className="arch-node">
          <h3>Windows</h3>
          <p>Each server as a normal process.</p>
        </div>
      </div>
    </div>
  );
}

export function Stack() {
  return (
    <table className="table spec">
      <tbody>
        {stack.map((row) => (
          <tr key={row.part}>
            <th scope="row">{row.part}</th>
            <td>{row.detail}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function Support() {
  return (
    <table className="table">
      <thead>
        <tr>
          <th scope="col">Area</th>
          <th scope="col">First release</th>
          <th scope="col">After</th>
        </tr>
      </thead>
      <tbody>
        {support.map((row) => (
          <tr key={row.area}>
            <th scope="row">{row.area}</th>
            <td>{row.first}</td>
            <td>{row.later}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function Compare({ limit }: { limit?: number }) {
  return (
    <div className="compare" role="table" aria-label="Common tasks today and with Consolry">
      <div className="compare-head" role="row">
        <span role="columnheader">Job</span>
        <span role="columnheader">Today</span>
        <span role="columnheader">With Consolry</span>
      </div>
      {comparisons.slice(0, limit).map((item) => (
        <div key={item.task} className="compare-row" role="row">
          <h3 role="rowheader">{item.task}</h3>
          <p role="cell" className="before">
            {item.before}
          </p>
          <p role="cell" className="after">
            {item.after}
          </p>
        </div>
      ))}
    </div>
  );
}

export function Platforms() {
  return (
    <div className="platforms">
      {platforms.map((platform) => (
        <article key={platform.name} className={platform.when === "Later" ? "later" : undefined}>
          <p className="pixel when">{platform.when}</p>
          <h3>{platform.name}</h3>
          <p>{platform.how}</p>
        </article>
      ))}
    </div>
  );
}

export function HeroCard() {
  return (
    <div className="hero-art" aria-hidden="true">
      <span className="block b1" />
      <span className="block b2" />
      <span className="block b3" />
      <div className="server-card">
        <div className="server-card-head">
          <span className="dot online" />
          <strong>survival</strong>
          <small>Paper 1.21.4</small>
        </div>
        <dl>
          <div>
            <dt>Players</dt>
            <dd>12 / 40</dd>
          </div>
          <div>
            <dt>TPS</dt>
            <dd>19.9</dd>
          </div>
          <div>
            <dt>Plugins</dt>
            <dd>34</dd>
          </div>
        </dl>
        <p className="server-card-row">
          6 plugin updates ready <span className="app-button">Update all</span>
        </p>
      </div>
      <div className="crash-card">
        <p className="pixel">Crash explained</p>
        <p>
          <strong>EssentialsX Chat</strong> didn't load because <strong>EssentialsX</strong> isn't installed.
        </p>
      </div>
    </div>
  );
}
