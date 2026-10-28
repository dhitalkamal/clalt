// which static meshes may share one draw call. pure: works on any object shaped like a mesh, so node tests need no
// three.js. merge.js does the actual geometry work.
const MIN_SIZE = 0.8;
const MIN_TOP = 1.0;

// the single material of a mesh, or null for a material array
const oneMat = (m) => (Array.isArray(m.material) ? null : m.material);

// size and top: the world box of the mesh when known. camera collision (nav/camray.js) only looks at meshes that
// are at least 0.8 wide and reach head height, so those stay alone and the camera rays see the same list as before
export function mergeable(m, box) {
  if (!m.isMesh || m.isInstancedMesh || m.isSkinnedMesh || m.visible === false) return false;
  const mat = oneMat(m);
  if (!mat || mat.transparent || mat.visible === false) return false;
  if (m.userData && Object.keys(m.userData).length) return false;
  if (box && box.size >= MIN_SIZE && box.top >= MIN_TOP) return false;
  return true;
}

// meshes with the same key can go into one geometry
export function mergeKey(m) {
  const g = m.geometry;
  return [oneMat(m).uuid, m.castShadow ? 1 : 0, m.receiveShadow ? 1 : 0, m.renderOrder || 0, [...g.attrs].sort().join('+'), g.indexed ? 'i' : 'n'].join('|');
}

// [{key, meshes}] for every key that has at least two meshes
export function groupMergeable(list) {
  const by = new Map();
  for (const m of list) {
    const k = mergeKey(m);
    if (!by.has(k)) by.set(k, []);
    by.get(k).push(m);
  }
  return [...by].filter(([, v]) => v.length > 1).map(([key, meshes]) => ({ key, meshes }));
}
