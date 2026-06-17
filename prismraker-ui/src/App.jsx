import React, { useState, useCallback } from "react";
import { useToolheads } from "./useToolheads.js";
import ToolheadCard from "./components/ToolheadCard.jsx";
import SpoolPicker from "./components/SpoolPicker.jsx";

export default function App() {
  const { toolheads, connected } = useToolheads();
  const [picker, setPicker] = useState(null); // the toolhead being assigned
  const [spools, setSpools] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // Open the picker for a toolhead and fetch the current spool catalog.
  const openPicker = useCallback(async (t) => {
    setPicker(t);
    setError(null);
    setLoading(true);
    try {
      const res = await fetch("/api/spools");
      if (!res.ok) throw new Error(`spools: ${res.status}`);
      setSpools(await res.json());
    } catch (e) {
      setError("Couldn't load spools — is prismraker-svc running?");
      setSpools([]);
    } finally {
      setLoading(false);
    }
  }, []);

  // Assign (or clear, spoolId 0) — the websocket broadcast updates the cards.
  const pick = useCallback(async (toolhead, spoolId) => {
    try {
      await fetch("/api/assign", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ toolhead, spoolId }),
      });
    } catch {
      /* surfaced on next load; keep the UI responsive */
    }
    setPicker(null);
  }, []);

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
          <ToolheadCard key={t.index} t={t} onClick={openPicker} />
        ))}
      </main>

      <footer className="app__footer">
        Click a toolhead to set its loaded spool · per-color tracking for the Snapmaker U1
      </footer>

      <SpoolPicker
        toolhead={picker}
        spools={spools}
        loading={loading}
        error={error}
        onPick={pick}
        onClose={() => setPicker(null)}
      />
    </div>
  );
}
