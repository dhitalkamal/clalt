// builds the whole garden campus of a v2 layout into one group: island, paths, bridges, ponds, pavilions, props,
// plants and fence. returns the group, the per-frame ticks and the island bounds. pure scene building, no
// agents: seats and walking live in compat.js.
import * as THREE from 'three';
import { rad } from './geo.js';
import { tintFor } from '../model.js';
import { buildTerrain } from './terrain.js';
import { buildPaths, bridge } from './pathmesh.js';
import { buildPlants } from './plantmesh.js';
import { buildFence } from './fence.js';
import { flowerBed, bench, lampPost, pond, fountain, standingTable, sign } from './propmesh.js';
import { cabin, space } from './pavdesk.js';
import { lobby } from './pavlobby.js';
import { cafe, play } from './pavbreak.js';
import { treehouse } from './treehouse.js';
import { std, geo, cached } from './res.js';
import { flagTex } from './tex.js';

const PLAZA = 0x9b9488;

// the pavilion of one spec: group in its own frame, plus a tick when it animates
function makePavilion(p) {
  switch (p.type) {
    case 'cabin': return { group: cabin(p, tintFor(p.account)) };
    case 'research': case 'workspace': return { group: space(p) };
    case 'lobby': return { group: lobby(p) };
    case 'cafe': return { group: cafe(p) };
    case 'treehouse': return { group: treehouse() };
    default: return play();
  }
}

// a round flagstone plaza under a fountain
function plaza(r) {
  const top = cached('m:flag-plaza', () => new THREE.MeshStandardMaterial({ map: flagTex(2, 2), roughness: 0.9 }));
  const side = std(PLAZA);
  const m = new THREE.Mesh(geo(`plaza:${r}`, () => new THREE.CylinderGeometry(r, r, 0.1, 40)), [side, top, side]);
  m.position.y = -0.05;
  m.receiveShadow = true;
  m.castShadow = true;
  return m;
}

function placeProps(site, root, ticks) {
  const put = (o, p, y = -0.03) => { o.position.set(p.x, y, p.z); o.rotation.y = rad(p.rot || 0); root.add(o); return o; };
  let seed = 3;
  for (const p of site.props) {
    seed++;
    if (p.t === 'bed') put(flowerBed(p.w || 1, p.d || 0.5, seed), p);
    else if (p.t === 'bench') put(bench(), p).scale.setScalar(1.1);
    else if (p.t === 'lamp') put(lampPost(), p);
    else if (p.t === 'standing-table') put(standingTable(), p);
    else if (p.t === 'sign') put(sign(), p);
    else if (p.t === 'fountain') {
      put(plaza(1.2), p, 0);
      const f = put(fountain(), p, 0);
      f.scale.setScalar(2.0);
      ticks.push((t) => f.userData.tick(t));
    }
  }
}

export function buildGarden(layout, site, plan) {
  const root = new THREE.Group();
  const ticks = [];
  const terr = buildTerrain(layout);
  root.add(terr.group);
  ticks.push(...terr.ticks);
  root.add(buildPaths(site.paths));
  for (const b of site.bridges) root.add(bridge(b.a, b.b, b.w));
  for (const pd of site.ponds) {
    const p = pond(pd.rx, pd.rz);
    p.group.position.set(pd.x, 0.02, pd.z);
    root.add(p.group);
    ticks.push(p.tick);
  }
  for (const { p } of site.pavs) {
    const made = makePavilion(p);
    made.group.position.set(p.x, 0, p.z);
    made.group.rotation.y = rad(p.rot || 0);
    root.add(made.group);
    if (made.tick) ticks.push(made.tick);
  }
  placeProps(site, root, ticks);
  if (layout.site.fence !== false) root.add(buildFence(layout, site));
  root.add(buildPlants(plan));
  return { group: root, ticks, bb: terr.bb };
}

