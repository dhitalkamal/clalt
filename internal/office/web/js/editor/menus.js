// small dropdown used by the Templates and Layouts buttons.
// load() -> [{key, text, hint?}], pick(item) runs when one is chosen.
import { h } from './dom.js';

export function createMenu(ed, label, load, pick) {
  const list = h('div', { class: 'ed-menu', role: 'menu', hidden: true });
  const btn = h('button', { type: 'button', 'aria-haspopup': 'menu', 'aria-expanded': 'false', text: label });
  const wrap = h('div', { class: 'ed-menu-wrap' }, btn, list);
  const api = {
    isOpen: () => !list.hidden,
    close() {
      list.hidden = true;
      btn.setAttribute('aria-expanded', 'false');
      btn.focus();
    },
  };

  async function show() {
    list.hidden = false;
    btn.setAttribute('aria-expanded', 'true');
    list.replaceChildren(h('p', { class: 'ed-note', text: 'Loading...' }));
    let items = [];
    try {
      items = await load();
    } catch (e) {
      list.replaceChildren(h('p', { class: 'ed-note ed-bad', text: 'Could not load: ' + (e && e.message ? e.message : e) }));
      return;
    }
    if (!items.length) {
      list.replaceChildren(h('p', { class: 'ed-note', text: 'Nothing here yet.' }));
      return;
    }
    list.replaceChildren(...items.map((it) => h('button', {
      type: 'button', role: 'menuitem',
      onclick: () => {
        api.close();
        pick(it);
      },
    }, h('span', { text: it.text }), it.hint ? h('small', { text: it.hint }) : null)));
    list.querySelector('button').focus();
  }

  btn.addEventListener('click', () => (list.hidden ? show() : api.close()));
  // arrow keys move through the items
  list.addEventListener('keydown', (e) => {
    const items = [...list.querySelectorAll('button')];
    const i = items.indexOf(document.activeElement);
    if (e.key === 'ArrowDown') items[(i + 1) % items.length].focus();
    else if (e.key === 'ArrowUp') items[(i - 1 + items.length) % items.length].focus();
    else return;
    e.preventDefault();
  });
  const outside = (e) => {
    if (!list.hidden && !wrap.contains(e.target)) {
      list.hidden = true;
      btn.setAttribute('aria-expanded', 'false');
    }
  };
  window.addEventListener('pointerdown', outside, true);
  ed.menus.push(api);
  api.el = wrap;
  api.dispose = () => window.removeEventListener('pointerdown', outside, true);
  return api;
}
