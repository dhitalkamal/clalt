// the forecast line and the Move all button of an account card.
import { el } from '../dom.js';
import { forecastView } from './forecast.js';

// info: the page's account record {label, limits, forecast}; onMoveAll: () => void or null
export function addForecast(div, info, now, onMoveAll) {
  const f = forecastView(info && info.forecast, now);
  if (!f) return;
  div.classList.add('warn-' + f.level);
  div.append(el('div', 'fc ' + f.level, f.text));
  if (!onMoveAll) return;
  const b = el('button', 'fc-btn', 'Move all');
  b.type = 'button';
  b.title = 'move the sessions of this account to the account with the most room';
  b.addEventListener('click', (e) => { e.stopPropagation(); onMoveAll(); });
  div.append(b);
}

// part of the label key: the line changes about once a minute, not every second
export const forecastKey = (info, now) => {
  const f = forecastView(info && info.forecast, now);
  return f ? f.text : '';
};
