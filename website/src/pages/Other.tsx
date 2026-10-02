import { PageHead, Section } from "../components/Layout";
import Pricing from "../components/Pricing";
import { faqs, firstRelease, site, stages } from "../content";

function FaqList({ items }: { items: typeof faqs }) {
  return (
    <div className="faq">
      {items.map((faq) => (
        <details key={faq.q}>
          <summary>{faq.q}</summary>
          <p>{faq.a}</p>
        </details>
      ))}
    </div>
  );
}

const pricingFaqs = faqs.filter((faq) => /free|paying|data/i.test(faq.q));

export function PricingPage() {
  return (
    <>
      <PageHead
        label="Pricing"
        title="Free to run. Paid plans for the extras."
        intro="You host Consolry yourself on every plan. Paying unlocks more features on your own hardware; it never rents you a server."
      />
      <section className="section alt">
        <div className="wrap">
          <Pricing />
        </div>
      </section>
      <section className="section">
        <div className="wrap split">
          <div>
            <p className="label">The rule</p>
            <h2>Safety is never paywalled.</h2>
          </div>
          <div className="split-text">
            <p>
              Backups, two-factor login, plugin verification and crash reports are in the free edition and
              will stay there.
            </p>
            <p>Paid plans sell time savings, team features and tools for hosting companies. Nothing else.</p>
          </div>
        </div>
      </section>
      <Section alt label="Questions" title="About paying">
        <FaqList items={pricingFaqs} />
        <p className="more">
          <a href="/faq">All questions</a>
        </p>
      </Section>
    </>
  );
}

export function RoadmapPage() {
  return (
    <>
      <PageHead
        label="Roadmap"
        title="An early preview is out"
        intro="Consolry is being built in the order below. The early preview already runs real Minecraft servers; the first full release has no date yet, and we would rather say that than guess."
      />
      <Section alt label="Build order" title="Eight stages, one at a time">
        <ol className="timeline">
          {stages.map((stage) => (
            <li key={stage.name} className={stage.status === "Planned" ? undefined : "marked"}>
              <span className="pixel">{stage.status}</span>
              <h3>{stage.name}</h3>
              <p>{stage.detail}</p>
            </li>
          ))}
        </ol>
      </Section>
      <Section
        label="First release"
        title="What ships first, and what waits"
        intro="The first release is a complete panel for a Minecraft server owner. Everything in the right-hand column comes after."
      >
        <table className="table">
          <thead>
            <tr>
              <th scope="col">Area</th>
              <th scope="col">In the first release</th>
              <th scope="col">Later</th>
            </tr>
          </thead>
          <tbody>
            {firstRelease.map((row) => (
              <tr key={row.area}>
                <th scope="row">{row.area}</th>
                <td>{row.now}</td>
                <td>{row.later}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Section>
    </>
  );
}

const exe = `${site.github}/releases/latest/download/Consolry.exe`;
const installCommand = "curl -fsSL https://www.consolry.com/install.sh | sh";

export function DownloadPage() {
  return (
    <>
      <PageHead
        label="Download"
        title="Install Consolry on your own machine"
        intro="This is an early preview. It runs real Minecraft servers, but it is unfinished: expect bugs, and keep your own backups of anything important."
      />

      <section className="section alt">
        <div className="wrap downloads">
          <article>
            <p className="pixel">Windows 10 and 11</p>
            <h2>Windows</h2>
            <a className="button" href={exe}>
              Download Consolry.exe
            </a>
            <ol className="steps">
              <li>Run the file you downloaded. A window opens and your browser shows the panel.</li>
              <li>Create your admin account.</li>
              <li>Create a server. Consolry downloads Minecraft and the right Java for you.</li>
            </ol>
            <p className="fine">
              Windows may say it "protected your PC", because the preview is not yet signed. Choose <strong>More info</strong>, then{" "}
              <strong>Run anyway</strong>. Keep the window open while your servers run.
            </p>
          </article>

          <article>
            <p className="pixel">64-bit Intel, AMD and ARM</p>
            <h2>Linux</h2>
            <pre className="command">
              <code>{installCommand}</code>
            </pre>
            <ol className="steps">
              <li>Paste the command into a terminal. It installs one program, <code>consolry</code>, and checks the download first.</li>
              <li>
                Start it by typing <code>consolry</code>.
              </li>
              <li>Open http://127.0.0.1:8700 in a browser on that machine and create your admin account.</li>
            </ol>
            <p className="fine">
              No screen on that machine? From your own computer run <code>ssh -L 8700:127.0.0.1:8700 you@your-server</code>, then open the same address
              locally.
            </p>
          </article>
        </div>
      </section>

      <Section label="Good to know" title="Before you start">
        <dl className="basics">
          <div>
            <dt>It runs on your machine</dt>
            <dd>The panel, your servers, worlds and backups all stay on the computer you install it on.</dd>
          </div>
          <div>
            <dt>Friends joining</dt>
            <dd>To let people outside your home join, forward the server's port (usually 25565) on your router. Consolry does not do this for you yet.</dd>
          </div>
          <div>
            <dt>Keep the panel private</dt>
            <dd>The panel only answers on your own machine. Do not open it to the internet in this preview.</dd>
          </div>
        </dl>
        <p className="more">
          <a href={`${site.github}/releases`}>All versions and release notes</a>
        </p>
      </Section>
    </>
  );
}

export function FaqPage() {
  const structuredData = {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: faqs.map((faq) => ({
      "@type": "Question",
      name: faq.q,
      acceptedAnswer: { "@type": "Answer", text: faq.a },
    })),
  };
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }} />
      <PageHead
        label="Questions"
        title="Things people ask"
        intro="Short answers to the common questions. If yours isn't here, it will be once there is a Discord to ask in."
      />
      <section className="section alt">
        <div className="wrap">
          <FaqList items={faqs} />
        </div>
      </section>
    </>
  );
}

export function NotFound() {
  return (
    <PageHead label="404" title="That page doesn't exist" intro="The link may be old, or the address may have a typo." />
  );
}
