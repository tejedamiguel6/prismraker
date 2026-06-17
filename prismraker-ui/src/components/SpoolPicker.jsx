import React from "react";

// SpoolPicker is a modal that lists available spools (from Spoolman, or demo
// data offline) so the user can load one onto the selected toolhead.
export default function SpoolPicker({ toolhead, spools, onPick, onClose, loading, error }) {
  if (!toolhead) return null;

  return (
    <div className="modal" onClick={onClose}>
      <div className="modal__panel" onClick={(e) => e.stopPropagation()}>
        <header className="modal__head">
          <span>Load spool onto <strong>{toolhead.name}</strong></span>
          <button className="modal__x" onClick={onClose} aria-label="Close">×</button>
        </header>

        {loading && <div className="modal__msg">Loading spools…</div>}
        {error && <div className="modal__msg modal__msg--err">{error}</div>}

        <ul className="spoollist">
          {spools.map((s) => (
            <li key={s.id}>
              <button className="spool" onClick={() => onPick(toolhead.index, s.id)}>
                <span className="spool__dot" style={{ background: s.colorHex || "#2a2d36" }} />
                <span className="spool__name">{s.name}</span>
                <span className="spool__grams">{s.remainingGram ? `${Math.round(s.remainingGram)} g` : "—"}</span>
              </button>
            </li>
          ))}
          {!loading && spools.length === 0 && (
            <li className="modal__msg">No spools found. Add some in Spoolman.</li>
          )}
        </ul>

        {toolhead.colorName && (
          <button className="spool spool--clear" onClick={() => onPick(toolhead.index, 0)}>
            Unassign {toolhead.colorName}
          </button>
        )}
      </div>
    </div>
  );
}
