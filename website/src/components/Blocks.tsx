import { comparisons, platforms } from "../content";

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
