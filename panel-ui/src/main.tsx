import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Fonts ship inside the panel so it looks right with no internet connection.
import "@fontsource/jersey-15/latin-400.css";
import "@fontsource/silkscreen/latin-400.css";
import "@fontsource/ibm-plex-sans/latin-400.css";
import "@fontsource/ibm-plex-sans/latin-600.css";
import "@fontsource/ibm-plex-mono/latin-400.css";
import App from "./App";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
