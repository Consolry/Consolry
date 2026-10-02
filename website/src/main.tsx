import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import App, { currentPath } from "./App";
import "./styles.css";

const root = document.getElementById("root")!;
const app = (
  <StrictMode>
    <App initialPath={currentPath()} />
  </StrictMode>
);

// The production build ships each page as ready-made HTML, so attach to it instead of redrawing.
if (root.firstElementChild) {
  hydrateRoot(root, app);
} else {
  createRoot(root).render(app);
}
