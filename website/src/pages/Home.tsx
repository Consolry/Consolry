import { Architecture, HeroCard, Platforms, Support } from "../components/Blocks";
import { Clouds, Section } from "../components/Layout";
import Showcase from "../components/Showcase";
import { checks, problems, site } from "../content";

export default function Home() {
  return (
    <>
      <section className="hero">
        <Clouds />
        <div className="wrap hero-grid">
          <div>
            <p className="label">Minecraft server panel</p>
            <h1>
              The game panel that <em>knows your game.</em>
            </h1>
            <p className="lede">
              Other panels start a container and hand you a console. Consolry knows which plugins you run,
              which are out of date, and why the server just crashed.
            </p>
            <div className="actions">
              <a href="/download" className="button">
                Try the pre-release
              </a>
              <a href="/features" className="button ghost">
                See the features
              </a>
              <a href={site.github} className="source-link">
                Follow on GitHub
              </a>
            </div>
            <ul className="checks">
              {checks.map((check) => (
                <li key={check}>{check}</li>
              ))}
            </ul>
          </div>
          <HeroCard />
        </div>
      </section>

      <section className="section alt">
        <div className="wrap">
          <p className="strip-title">Three things panels still get wrong</p>
          <ol className="problems">
            {problems.map((problem, index) => (
              <li key={problem.title}>
                <span className="pixel">{String(index + 1).padStart(2, "0")}</span>
                <h3>{problem.title}</h3>
                <p>{problem.body}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <Section
        label="Look inside"
        title="A panel that reads the server, not just runs it"
        intro="Click through the tabs. Each one is a job other panels leave to you."
      >
        <Showcase />
        <p className="more">
          <a href="/features">All features</a>
        </p>
      </Section>

      <Section
        alt
        label="Architecture"
        title="One panel, one daemon per machine"
        intro="The panel is a single Go binary. Each machine that hosts servers runs a small daemon, which the panel controls over HTTPS and WebSocket."
      >
        <Architecture />
        <p className="more">
          <a href="/features">The full stack</a>
        </p>
      </Section>

      <Section
        label="Platforms"
        title="Runs on the machine you already own"
        intro="One program for the panel and one for each machine that hosts servers. Mix Windows and Linux machines in the same panel."
      >
        <Platforms />
        <h3 className="sub">What's supported</h3>
        <Support />
      </Section>

      <section className="section alt">
        <div className="wrap split">
          <div>
            <p className="label">Free and open</p>
            <h2>Free to run, and safety is never paywalled.</h2>
          </div>
          <div className="split-text">
            <p>
              The Community edition is free, with no limit on servers or nodes. Its core will be released as open source at the first full release.
              Backups, two-factor login and crash reports stay free.
            </p>
            <p>Paid plans start at $6 a month and add time savers, team features and tools for hosts.</p>
            <a href="/pricing" className="button ghost">
              Compare the plans
            </a>
          </div>
        </div>
      </section>
    </>
  );
}
