// frees the GPU side of an object tree. meshes cloned from the loaded models and the shared textures are
// flagged userData.shared and are left alone, everything else was made for this tree.
export function disposeTree(root) {
  root.traverse((o) => {
    if (o.isSkinnedMesh) o.skeleton?.dispose();
    if (o.userData.shared) return;
    o.geometry?.dispose();
    for (const m of [].concat(o.material || [])) {
      if (m.map && !m.map.userData.shared) m.map.dispose();
      m.dispose();
    }
  });
}
