import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "@/App";
import "@/lib/i18n";
import { applyPersistedLanguage } from "@/lib/language";
import "@/themes/tokens.css";
import "@/app.css";

applyPersistedLanguage();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
