// small dropdown used by the Templates and Layouts buttons.
// load() -> [{key, text, hint?, removable?}], pick(item) runs when one is chosen.
// remove(item) -> promise, given: a removable item gets a delete button that asks twice.
import { h } from './dom.js';
import { createConfirmPress } from '../confirmpress.js';
import { DELETE_LABEL, CONFIRM_LABEL } from '../settings.js';

export function createMenu(ed, label, load, pick, remove) {
  const list = h('div', { class: 'ed-menu', role: 'menu', hidden: true });
  const btn = h('button', { type: 'button', 'aria-haspopup': 'menu', 'aria-expanded': 'false', text: label });
  const wrap = h('div', { class: 'ed-menu-wrap' }, btn, list);
  // at most one delete button is armed at a time
  let armed = null;
  const disarm = () => { if (armed) armed.reset(); armed = null; };
  const api = {
    isOpen: () => !list.hidden,
    close() {
      disarm();
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
    list.replaceChildren(...items.map((it) => {
      const item = h('button', {
        type: 'button', role: 'menuitem',
        onclick: () => {
          api.close();
          pick(it);
        },
      }, h('span', { text: it.text }), it.hint ? h('small', { text: it.hint }) : null);
      return remove && it.removable ? h('div', { class: 'ed-menu-row' }, item, deleteButton(it)) : item;
    }));
    list.querySelector('button').focus();
  }

  // first press arms ("Delete layout?"), a second one within a few seconds deletes
  function deleteButton(it) {
    const btn = h('button', { type: 'button', class: 'ed-del', text: DELETE_LABEL, 'aria-label': 'Delete layout ' + it.text });
    const press = createConfirmPress({
      onChange: (on) => {
        btn.textContent = on ? CONFIRM_LABEL : DELETE_LABEL;
        btn.classList.toggle('ed-danger', on);
      },
    });
    btn.addEventListener('click', async () => {
      // another button's pending delete is dropped first, so it never resets this press
      if (armed !== press) disarm();
      if (press.press() === 'arm') {
        armed = press;
        return;
      }
      armed = null;
      btn.disabled = true;
      try {
        await remove(it);
      } catch (e) {
        ed.setStatus(e && e.message ? e.message : String(e), 'bad');
      }
      // redraw the list, or close it when the layout is all that was there
      if (!list.hidden) show();
    });
    return btn;
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
    // a press elsewhere in the open menu disarms a delete button, outside it closes the menu
    if (armed && !(e.target instanceof Node && e.target.closest && e.target.closest('.ed-del'))) disarm();
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
