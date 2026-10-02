import { Compare, HeroCard, Platforms } from "../components/Blocks";
import { Clouds, Section } from "../components/Layout";
import Showcase from "../components/Showcase";
import { checks, problems } from "../content";

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
              <a href="#waitlist" className="button">
                Join the waitlist
              </a>
              <a href="/features" className="button ghost">
                See the features
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
        label="Before and after"
        title="The same jobs, without the chores"
        intro="Nothing here is new work. It is the work you already do, with the panel doing the tedious half."
      >
        <Compare limit={3} />
        <p className="more">
          <a href="/features">See all five</a>
        </p>
      </Section>

      <Section
        label="Platforms"
        title="Runs on the machine you already own"
        intro="One program for the panel and one for each machine that hosts servers. Mix Windows and Linux machines in the same panel."
      >
        <Platforms />
      </Section>

      <section className="section alt">
        <div className="wrap split">
          <div>
            <p className="label">Free and open</p>
            <h2>The core is open source, and safety is never paywalled.</h2>
          </div>
          <div className="split-text">
            <p>
              The Community edition is free under the AGPL-3.0 licence, with no limit on servers or nodes.
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
