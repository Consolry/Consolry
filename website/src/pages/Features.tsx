import { Compare, Platforms } from "../components/Blocks";
import { PageHead, Section } from "../components/Layout";
import { AppWindow, views } from "../components/Showcase";
import { audiences, basics } from "../content";

const gameViews = views.filter((view) => view.id !== "console");
const consoleView = views.find((view) => view.id === "console")!;

export default function Features() {
  return (
    <>
      <PageHead
        label="Features"
        title="What Consolry does that your panel doesn't"
        intro="Four things that need the panel to understand Minecraft, and the basics every panel should already get right."
      />

      {gameViews.map((view, index) => (
        <section key={view.id} className={`section feature${index % 2 ? "" : " alt"}`}>
          <div className="wrap">
            <div className="feature-text">
              <p className="pixel">{String(index + 1).padStart(2, "0")}</p>
              <h2>{view.title}</h2>
              <p>{view.caption}</p>
            </div>
            <AppWindow viewId={view.id}>{view.body}</AppWindow>
          </div>
        </section>
      ))}

      <Section
        label="The basics"
        title="And everything a panel should already do"
        intro="Consolry replaces your current panel, so the everyday tools are all here."
      >
        <AppWindow viewId={consoleView.id}>{consoleView.body}</AppWindow>
        <dl className="basics">
          {basics.map((item) => (
            <div key={item.name}>
              <dt>{item.name}</dt>
              <dd>{item.detail}</dd>
            </div>
          ))}
        </dl>
      </Section>

      <Section alt label="Before and after" title="The same five jobs, without the chores">
        <Compare />
      </Section>

      <Section
        label="Platforms"
        title="Runs on the machine you already own"
        intro="Mix Windows and Linux machines in the same panel."
      >
        <Platforms />
        <h3 className="sub">Who gets what, and when</h3>
        <ul className="audiences">
          {audiences.map((audience) => (
            <li key={audience.who}>
              <strong>{audience.who}</strong>
              <span>{audience.what}</span>
              <em>{audience.when}</em>
            </li>
          ))}
        </ul>
      </Section>
    </>
  );
}
