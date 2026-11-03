// what a new hall layout takes down before the roster layer is rebuilt. kept free of three.js so node can test it.
export function releaseRoster(app, removeAgent, drop) {
  for (const id of [...app.agents.keys()]) removeAgent(id);
  // refreshSeats only drops the layer it finds in app.dynamic, so it goes here before the slot is cleared
  drop(app.dynamic && app.dynamic.group);
  app.dynamic = null;
  app.dimmed = null;
}
