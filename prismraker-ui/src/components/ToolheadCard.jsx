import React from "react";

// Pick readable text color against the swatch background.
function contrast(hex) {
  const h = hex.replace("#", "");
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return lum > 0.6 ? "#11131a" : "#f5f6fa";
}

export default function ToolheadCard({ t }) {
  const low = t.remainingGram > 0 && t.remainingGram < 80;
  const fg = contrast(t.colorHex || "#888888");

  return (
    <div className={`card ${t.active ? "card--active" : ""}`}>
      <div className="swatch" style={{ background: t.colorHex, color: fg }}>
        <span className="swatch__name">{t.name}</span>
        {t.active && <span className="swatch__badge" style={{ color: fg }}>PRINTING</span>}
      </div>
      <div className="card__body">
        <div className="card__color">{t.colorName || "Unassigned"}</div>
        <div className="card__row">
          <span>Nozzle</span>
          <span>{Math.round(t.temperature)}°C{t.target ? ` → ${Math.round(t.target)}°` : ""}</span>
        </div>
        <div className="card__row">
          <span>Used</span>
          <span>{(t.usedMm / 1000).toFixed(2)} m</span>
        </div>
        <div className={`card__row ${low ? "card__row--warn" : ""}`}>
          <span>Remaining</span>
          <span>{t.remainingGram ? `${Math.round(t.remainingGram)} g` : "—"}{low ? " ⚠" : ""}</span>
        </div>
      </div>
    </div>
  );
}
