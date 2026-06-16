import { useEffect, useState } from "react";

// Demo data so the dashboard renders before prismraker-svc is wired to a
// real U1. Replace by connecting to the live websocket below.
const DEMO = [
  { index: 0, name: "T0", colorName: "Galaxy Black", colorHex: "#1c1c20", temperature: 215, target: 215, active: true, usedMm: 4210, remainingGram: 642 },
  { index: 1, name: "T1", colorName: "Signal Red", colorHex: "#d7263d", temperature: 60, target: 0, active: false, usedMm: 1180, remainingGram: 318 },
  { index: 2, name: "T2", colorName: "Cyan", colorHex: "#1aa7c4", temperature: 61, target: 0, active: false, usedMm: 905, remainingGram: 41 },
  { index: 3, name: "T3", colorName: "Bone White", colorHex: "#ece5d8", temperature: 59, target: 0, active: false, usedMm: 2640, remainingGram: 770 },
];

// useToolheads subscribes to prismraker-svc's broadcast websocket and falls
// back to demo data when the service isn't reachable (e.g. on first load).
export function useToolheads() {
  const [toolheads, setToolheads] = useState(DEMO);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let ws;
    try {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      ws = new WebSocket(`${proto}://${location.host}/api/stream`);
      ws.onopen = () => setConnected(true);
      ws.onclose = () => setConnected(false);
      ws.onmessage = (ev) => {
        try {
          const data = JSON.parse(ev.data);
          if (Array.isArray(data) && data.length) setToolheads(data);
        } catch {
          /* ignore malformed frames */
        }
      };
    } catch {
      setConnected(false);
    }
    return () => ws && ws.close();
  }, []);

  return { toolheads, connected };
}
