import * as THREE from 'three';
import { furniture } from './assets.js';
import { skyTexture, paintingTexture } from './art.js';
import { glowTexture } from './props.js';

const WOOD = new THREE.Color(0xa8744a);
const frameMat = new THREE.MeshStandardMaterial({ color: 0x8a5a36, roughness: 0.8 });

function framedPicture(v) {
  const g = new THREE.Group();
  const frame = new THREE.Mesh(new THREE.BoxGeometry(0.4, 0.3, 0.025), frameMat);
  const art = new THREE.Mesh(new THREE.PlaneGeometry(0.34, 0.25), new THREE.MeshStandardMaterial({ map: paintingTexture(v), roughness: 0.9 }));
  art.position.z = 0.014;
  g.add(frame, art);
  return g;
}

// walls, floor, windows with sky behind them, wall art, floor lamps. rebuilt only when the room size changes.
export function buildShell(layout) {
  const { W, D, xmin, zmin, door } = layout;
  const g = new THREE.Group();
  const lamps = [];
  const put = (obj, x, y, z, ry = 0) => { obj.position.set(x, y, z); obj.rotation.y = ry; g.add(obj); return obj; };
  for (let i = 0; i < W; i++) {
    for (let j = 0; j < D; j++) {
      const tile = put(furniture('floorFull', { noCast: true }), xmin + i + 0.5, -0.05, zmin + j + 0.5);
      tile.traverse((o) => { if (o.isMesh) { o.material = o.material.clone(); o.material.color.multiplyScalar(0.62).lerp(WOOD, 0.35); } });
    }
  }
  const skyMat = new THREE.MeshBasicMaterial({ map: skyTexture(), toneMapped: false });
  const skyGeo = new THREE.PlaneGeometry(1.0, 0.95);
  const patchMat = new THREE.MeshBasicMaterial({ map: glowTexture(), color: 0xffd9a0, transparent: true, opacity: 0.3,
    depthWrite: false, blending: THREE.AdditiveBlending, toneMapped: false });
  let art = 0;
  for (let i = 0; i < W; i++) {
    const win = i % 2 === 1 && i > 0 && i < W - 1;
    put(furniture(win ? 'wallWindow' : 'wall'), xmin + i + 0.5, 0, zmin - 0.03);
    // second storey of wall behind the mezzanine so the deck has a back wall at its own level
    const cx = xmin + i + 0.5;
    if (cx > layout.deck.x0 - 0.6 && cx < layout.deck.x1 + 1.6) {
      put(furniture(win ? 'wallWindow' : 'wall'), cx, 1.29, zmin - 0.03);
      if (win) put(new THREE.Mesh(skyGeo, skyMat), cx, 1.29 + 0.7, zmin - 0.25);
    }
    if (win) {
      const sky = new THREE.Mesh(skyGeo, skyMat);
      put(sky, xmin + i + 0.5, 0.7, zmin - 0.25);
      sky.rotation.y = 0;
    } else if (i > 0 && i < W - 1) {
      const pic = framedPicture(art++);
      put(pic, xmin + i + 0.5, 0.78, zmin + 0.04);
    }
  }
  const doorTile = Math.round(door.z - zmin - 0.5);
  for (let j = 0; j < D; j++) {
    const isDoor = j === doorTile;
    const win = !isDoor && j % 2 === 0 && j < D - 3;
    put(furniture(isDoor ? 'wallDoorway' : win ? 'wallWindow' : 'wall'), xmin - 0.03, 0, zmin + j + 0.5, Math.PI / 2);
    if (win) {
      const sky = new THREE.Mesh(skyGeo, skyMat);
      put(sky, xmin - 0.25, 0.7, zmin + j + 0.5, Math.PI / 2);
      // soft sun patch on the floor under the window
      const patch = new THREE.Mesh(new THREE.PlaneGeometry(1.8, 0.8), patchMat);
      patch.rotation.x = -Math.PI / 2;
      put(patch, xmin + 1.2, 0.012, zmin + j + 0.55, 0.12);
      patch.rotation.x = -Math.PI / 2;
    } else if (!isDoor && j > 0) {
      put(framedPicture(art++), xmin + 0.04, 0.78, zmin + j + 0.5, Math.PI / 2);
    }
  }
  put(furniture('wallCorner'), xmin - 0.03, 0, zmin - 0.03);
  put(furniture('lampRoundFloor', { scale: 1.1 }), xmin + W - 0.25, 0, zmin + 0.25);
  lamps.push(new THREE.Vector3(xmin + W - 0.25, 0.85, zmin + 0.5));
  put(furniture('pottedPlant', { scale: 1.5 }), xmin + 0.25, 0, zmin + 0.25);
  put(furniture('lampRoundFloor', { scale: 1.1 }), xmin + 0.25, 0, door.z - 2.0, Math.PI / 2);
  lamps.push(new THREE.Vector3(xmin + 0.4, 0.85, door.z - 2.0));
  // glass in the window frames: nearly clear so the sky shows, never blocks the sun
  g.traverse((o) => {
    if (!o.isMesh || o.material.name !== 'glass') return;
    o.castShadow = false;
    o.material = o.material.clone();
    o.material.opacity = 0.14;
  });
  g.userData.lamps = lamps;
  return g;
}
