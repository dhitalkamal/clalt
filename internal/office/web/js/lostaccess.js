// what a page says when the server no longer knows its link (the server restarted or the link is old)
export const LOST_ACCESS = 'this page lost its access to clalt office. open a new link with clalt office open; unsaved changes here will be lost';

// a failed reply as one line: 401 is the lost access text, else the server's body or the status
export const failText = (status, text) => (status === 401 ? LOST_ACCESS : (text || '').trim() || 'http ' + status);

// the banner of the snapshot poll
export const pollErrorText = (e) => (e && e.message === 'auth' ? LOST_ACCESS : 'lost the connection to clalt office, trying again');
