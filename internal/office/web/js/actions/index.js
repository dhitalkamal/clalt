// glue the scene files call: the account card extras (forecast line, Move all).
import { app, ui } from '../app.js';
import { setWingExtra } from '../ui.js';
import { addForecast, forecastKey } from './wing.js';
import { openMoveAll } from './moveall.js';

const ctx = () => ({ accounts: ui.accounts, sessions: app.sessions });

// installActions makes renderWing add the forecast line and the Move all button to each account card
export function installActions() {
  setWingExtra((id) => ({
    key: forecastKey(app.accounts.get(id), Date.now()),
    render: (div) => addForecast(div, app.accounts.get(id), Date.now(), () => openMoveAll(ctx, id)),
  }));
}
