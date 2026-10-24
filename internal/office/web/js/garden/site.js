// the ground of a v2 layout turned into what the planners ask about: smoothed paths, water, clear spots. pure.
import { curve, inPoly, distPoly, boundsOf } from './geo.js';
import { SegIndex } from './segindex.js';
import { polyOf, sizeOf } from './footprints.js';

// how much room a prop takes, as a radius
const PROP_R = { bench: 0.5, bed: 0.6, lamp: 0.2, fountain: 0.7, 'standing-table': 0.4, sign: 0.3 };

export function buildSite(layout) {
  const s = layout.site;
  const paths = (s.paths || []).map((p) => ({ pts: p.smooth ? curve(p.pts, 8) : p.pts, w: p.w || 0.9 }));
  const bridges = (s.bridges || []).map((b) => ({ a: b.a, b: b.b, w: b.w || 1.1 }));
  const ponds = s.ponds || [];
  const river = s.river || null;
  const pavs = (layout.pavilions || []).filter((p) => sizeOf(p)).map((p) => ({ p, poly: polyOf(p) }));
  const roads = new SegIndex(3);
  for (const p of paths) roads.add(p.pts, p.w);
  const bridgeIdx = new SegIndex(3);
  for (const b of bridges) bridgeIdx.add([b.a, b.b], b.w);
  const riverIdx = river ? new SegIndex(4).add(river.pts, river.w) : null;
  const land = s.land.map((l) => l.poly);
  const baseBounds = boundsOf(s.base.poly);

  const inLand = (x, z) => land.some((poly) => inPoly(x, z, poly));
  const inPond = (x, z, m = 0) => ponds.some((pd) => ((x - pd.x) / (pd.rx + m)) ** 2 + ((z - pd.z) / (pd.rz + m)) ** 2 < 1);
  const onBridge = (x, z, m = 0) => bridgeIdx.gap(x, z) < m;
  // open water: the river and ponds, except where a bridge crosses it
  const onWater = (x, z) => (riverIdx !== null && riverIdx.gap(x, z) < 0 && !onBridge(x, z)) || inPond(x, z);

  const keepEye = (x, z, m) => {
    if (!layout.eye) return true;
    const [ex, ez] = layout.eye.from;
    const dl = Math.hypot(layout.eye.look[0] - ex, layout.eye.look[1] - ez) || 1;
    for (const k of [0, 2.5, 5]) {
      if (Math.hypot(x - (ex + ((layout.eye.look[0] - ex) / dl) * k), z - (ez + ((layout.eye.look[1] - ez) / dl) * k)) < 1.6 + m) return false;
    }
    return true;
  };

  // is a spot of clearance m free of pavilions, water, paths, bridges, props and the eye level view
  const free = (x, z, m) => {
    for (const it of pavs) if (inPoly(x, z, it.poly) || distPoly(x, z, it.poly) < m + 0.35) return false;
    if (inPond(x, z, m + 0.3)) return false;
    if (riverIdx !== null && riverIdx.gap(x, z) < m) return false;
    if (roads.gap(x, z) < m + 0.2) return false;
    if (bridgeIdx.gap(x, z) < m + 0.5) return false;
    for (const pr of s.props || []) if (Math.hypot(x - pr.x, z - pr.z) < (PROP_R[pr.t] || 0.4) + m) return false;
    return keepEye(x, z, m);
  };

  // on a path ribbon or a bridge deck
  const onRoad = (x, z, m = 0) => roads.gap(x, z) < m || bridgeIdx.gap(x, z) < m;
  return { paths, bridges, ponds, river, pavs, land, props: s.props || [], bounds: baseBounds, inLand, inPond, onWater, onBridge, onRoad, free, roads, bridgeIdx };
}
