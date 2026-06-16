import React from "react";
import { useToolheads } from "./useToolheads.js";
import ToolheadCard from "./components/ToolheadCard.jsx";

export default function App() {
  const { toolheads, connected } = useToolheads();

  return (
    <div className="app">
      <header className="app__header">
        <h1>Prismraker</h1>
        <span className={`dot ${connected ? "dot--on" : "dot--off"}`} />
        <span className="app__sub">
          {connected ? "live" : "demo (svc offline)"} · {toolheads.length} toolheads
        </span>
      </header>

      <main className="grid">
        {toolheads.map((t) => (
          <ToolheadCard key={t.index} t={t} />
        ))}
      </main>

      <footer className="app__footer">
        Per-color spool tracking for the Snapmaker U1 · talks to prismraker-svc
      </footer>
    </div>
  );
}
