import { marked } from "marked";
import source from "./docs.md?raw";

export type Doc = { slug: string; group: string; title: string; description: string; html: string; path: string };

// docs.md holds every page. A line starting with "===" begins one:
//   === slug | Group | Title | One-line description
const pages = source.replace(/\r\n/g, "\n").split(/^=== /m).slice(1);

export const docs: Doc[] = pages.map((page) => {
  const newline = page.indexOf("\n");
  const [slug, group, title, description] = page
    .slice(0, newline)
    .split("|")
    .map((part) => part.trim());
  return {
    slug,
    group,
    title,
    description,
    html: marked.parse(page.slice(newline + 1), { async: false }),
    path: slug ? `/docs/${slug}` : "/docs",
  };
});

export const docGroups = [...new Set(docs.map((doc) => doc.group))];
