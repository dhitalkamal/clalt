// the page side of PR reviews: the inbox button with its count badge, the inbox and the settings section.
import { openActivity } from '../activity/index.js';
import { createInbox } from './inbox.js';
import { createReviewsSection } from './section.js';
import { showButton, badgeText, openCount } from './model.js';

export function startReviews() {
  const btn = document.getElementById('b-reviews');
  const badge = btn.querySelector('.badge');
  const paint = (r) => {
    btn.hidden = !showButton(r);
    badge.textContent = badgeText(r ? r.count || 0 : 0);
    badge.hidden = !r || !(r.count > 0);
    btn.classList.toggle('busy', !!(r && r.running));
    btn.title = r && r.running ? 'reviewing ' + r.running.repo + '#' + r.running.number : 'pull request reviews';
  };
  const inbox = createInbox({
    openRun: (rec) => openActivity(rec),
    // the list reply has no count field, the snapshot does: keep the badge in step with what the inbox just read
    onList: (l) => paint({ enabled: l.enabled, running: l.running, count: openCount(l) }),
  });
  btn.addEventListener('click', () => (inbox.isOpen() ? inbox.close() : inbox.open(btn)));
  return { section: createReviewsSection(), update: (snapReviews) => paint(snapReviews || null) };
}
