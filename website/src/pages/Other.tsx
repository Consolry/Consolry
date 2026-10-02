import { PageHead, Section } from "../components/Layout";
import Pricing from "../components/Pricing";
import { faqs, firstRelease, stages } from "../content";

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
        title="Nothing is released yet"
        intro="Consolry is being built in the order below, and this site is the first piece. There is no release date, and we would rather say that than guess."
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
