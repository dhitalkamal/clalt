// floor, walls, windows with sky behind them, wall art and the door of a layout. rebuilt when size, door,
// deck or colours change.
import * as THREE from 'three';
import { furniture } from '../assets.js';
import { skyTexture, paintingTexture } from '../art.js';
import { glowTexture } from '../props.js';
import { MEZ } from './geom.js';

const frameMat = new THREE.MeshStandardMaterial({ color: 0x8a5a36, roughness: 0.8 });

function framedPicture(v) {
  const g = new THREE.Group();
  const frame = new THREE.Mesh(new THREE.BoxGeometry(0.4, 0.3, 0.025), frameMat);
  const art = new THREE.Mesh(new THREE.PlaneGeometry(0.34, 0.25), new THREE.MeshStandardMaterial({ map: paintingTexture(v), roughness: 0.9 }));
  art.position.z = 0.014;
  g.add(frame, art);
  return g;
}

// the wall surface of every wall piece takes the layout's wall colour. materials are shared between clones, so copy first.
function tintWalls(root, color) {
  const c = new THREE.Color(color);
  root.traverse((o) => {
    if (!o.isMesh || o.material.name !== '_defaultMat') return;
    o.material = o.material.clone();
    o.material.color.copy(c);
  });
}

export function buildShell(L) {
  const { W, D, xmin, zmin, door } = L;
  const lay = L.layout;
  const g = new THREE.Group();
  const walls = new THREE.Group();
  g.add(walls);
  const put = (obj, x, y, z, ry = 0, parent = g) => { obj.position.set(x, y, z); obj.rotation.y = ry; parent.add(obj); return obj; };
  // marks a piece as part of a wall for the cutaway: the wall's outward normal and a point on its plane
  const cut = (obj, nx, nz, x, z) => { obj.userData.cut = { nx, nz, x, z }; return obj; };
  const x1 = xmin + W;
  const z1 = zmin + D;
  const nW = Math.ceil(W);
  const nD = Math.ceil(D);
  const floor = new THREE.Color(lay.floorColor || '#bb9676');
  for (let i = 0; i < nW; i++) {
    for (let j = 0; j < nD; j++) {
      const tile = put(furniture('floorFull', { noCast: true }), xmin + i + 0.5, -0.05, zmin + j + 0.5);
      // an odd last row or column is a half tile
      if (i === nW - 1 && W - i < 1) { tile.scale.x = W - i; tile.position.x = xmin + i + (W - i) / 2; }
      if (j === nD - 1 && D - j < 1) { tile.scale.z = D - j; tile.position.z = zmin + j + (D - j) / 2; }
      tile.traverse((o) => { if (o.isMesh) { o.material = o.material.clone(); o.material.color.copy(floor); } });
    }
  }
  const skyMat = new THREE.MeshBasicMaterial({ map: skyTexture(), toneMapped: false });
  const skyGeo = new THREE.PlaneGeometry(1.0, 0.95);
  const patchMat = new THREE.MeshBasicMaterial({ map: glowTexture(), color: 0xffd9a0, transparent: true, opacity: 0.3,
    depthWrite: false, blending: THREE.AdditiveBlending, toneMapped: false });
  const doorBack = door.side === 'back';
  const doorLeft = door.side === 'left';
  const deckX0 = L.deck ? L.deck.x0 : 0;
  const deckX1 = L.deck ? Math.max(L.deck.x1, L.stairs ? L.X(L.stairs.landing.x + L.stairs.landing.w) : 0) : 0;
  let art = 0;
  for (let i = 0; i < nW; i++) {
    const cx = xmin + i + 0.5;
    const win = i % 2 === 1 && i > 0 && i < nW - 1 && !(doorBack && i === door.tile);
    const isDoor = doorBack && i === door.tile;
    cut(put(furniture(isDoor ? 'wallDoorway' : win ? 'wallWindow' : 'wall'), cx, 0, zmin - 0.03, 0, walls), 0, -1, 0, zmin);
    // second storey behind the deck so it has a back wall at its own level
    if (L.deck && cx > deckX0 - 0.6 && cx < deckX1 + 0.6) {
      cut(put(furniture(win ? 'wallWindow' : 'wall'), cx, MEZ, zmin - 0.03, 0, walls), 0, -1, 0, zmin);
      if (win) cut(put(new THREE.Mesh(skyGeo, skyMat), cx, MEZ + 0.7, zmin - 0.25), 0, -1, 0, zmin);
    }
    if (win) cut(put(new THREE.Mesh(skyGeo, skyMat), cx, 0.7, zmin - 0.25), 0, -1, 0, zmin);
    else if (!isDoor && i > 0 && i < nW - 1) cut(put(framedPicture(art++), cx, 0.78, zmin + 0.04), 0, -1, 0, zmin);
  }
  for (let j = 0; j < nD; j++) {
    const isDoor = doorLeft && j === door.tile;
    const win = !isDoor && j % 2 === 0 && j < nD - 3;
    cut(put(furniture(isDoor ? 'wallDoorway' : win ? 'wallWindow' : 'wall'), xmin - 0.03, 0, zmin + j + 0.5, Math.PI / 2, walls), -1, 0, xmin, 0);
    if (win) {
      cut(put(new THREE.Mesh(skyGeo, skyMat), xmin - 0.25, 0.7, zmin + j + 0.5, Math.PI / 2), -1, 0, xmin, 0);
      // soft sun patch on the floor under the window
      const patch = new THREE.Mesh(new THREE.PlaneGeometry(1.8, 0.8), patchMat);
      patch.rotation.x = -Math.PI / 2;
      put(patch, xmin + 1.2, 0.012, zmin + j + 0.55, 0.12);
      patch.rotation.x = -Math.PI / 2;
    } else if (!isDoor && j > 0) cut(put(framedPicture(art++), xmin + 0.04, 0.78, zmin + j + 0.5, Math.PI / 2), -1, 0, xmin, 0);
  }
  // a door on the open sides is a lone doorway piece, the camera looks in from there
  if (door.side === 'right') cut(put(furniture('wallDoorway'), x1 + 0.03, 0, zmin + door.tile + 0.5, Math.PI / 2, walls), 1, 0, x1, 0);
  if (door.side === 'front') cut(put(furniture('wallDoorway'), xmin + door.tile + 0.5, 0, z1 + 0.03, 0, walls), 0, 1, 0, z1);
  cut(put(furniture('wallCorner'), xmin - 0.03, 0, zmin - 0.03, 0, walls), -0.7071, -0.7071, xmin, zmin);
  // doormat just inside the door
  const mat = furniture('rugDoormat', { scale: 1.2, noCast: true });
  const yaw = { left: Math.PI / 2, right: -Math.PI / 2, back: 0, front: Math.PI }[door.side];
  put(mat, door.th.x - door.out[0] * 0.5, 0.012, door.th.z - door.out[1] * 0.5, yaw);
  tintWalls(walls, lay.walls && lay.walls.color ? lay.walls.color : '#ffffff');
  // glass in the window frames: nearly clear so the sky shows, never blocks the sun
  g.traverse((o) => {
    if (!o.isMesh || o.material.name !== 'glass') return;
    o.castShadow = false;
    o.material = o.material.clone();
    o.material.opacity = 0.14;
  });
  return g;
}
