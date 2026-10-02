import { docGroups, docs } from "../docs";
import { NotFound } from "./Other";

export default function DocsPage({ path }: { path: string }) {
  const index = docs.findIndex((doc) => doc.path === path);
  if (index < 0) return <NotFound />;
  const doc = docs[index];
  const previous = docs[index - 1];
  const next = docs[index + 1];

  return (
    <div className="wrap docs">
      <nav className="docs-nav" aria-label="Documentation">
        {docGroups.map((group) => (
          <div key={group}>
            <p className="pixel">{group}</p>
            <ul>
              {docs
                .filter((item) => item.group === group)
                .map((item) => (
                  <li key={item.path}>
                    <a href={item.path} aria-current={item.path === path ? "page" : undefined}>
                      {item.title}
                    </a>
                  </li>
                ))}
            </ul>
          </div>
        ))}
      </nav>

      <article className="doc">
        <p className="label">{doc.group}</p>
        <h1>{doc.title}</h1>
        <p className="lede">{doc.description}</p>
        {/* The page text comes from docs.md in this repository, not from visitors. */}
        <div className="doc-body" dangerouslySetInnerHTML={{ __html: doc.html }} />

        <nav className="doc-pager" aria-label="More pages">
          {previous ? (
            <a href={previous.path}>
              <span className="pixel">Previous</span>
              {previous.title}
            </a>
          ) : (
            <span />
          )}
          {next && (
            <a href={next.path} className="next">
              <span className="pixel">Next</span>
              {next.title}
            </a>
          )}
        </nav>
      </article>
    </div>
  );
}
