// what the page forgets when the activity drawer closes: the pinned agent and region and their card
export function releaseSelection(app, ui) {
  app.selected = null;
  app.selectedRegion = null;
  ui.showCard(null);
}
