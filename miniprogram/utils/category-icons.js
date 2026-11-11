const catalog = require('../assets/category-icons/catalog');
const byKey = {};
for (const item of catalog) byKey[item.key] = item;
const RELEASE = 'category-line-icons-v1';
let loading, ready = false;
const listeners = new Set();
function known(key) { return typeof key === 'string' && Object.prototype.hasOwnProperty.call(byKey, key); }
function glyph(key) { return known(key) ? String.fromCodePoint(byKey[key].codepoint) : ''; }
function fallback(icon) { return typeof icon === 'string' && icon.trim() ? icon : '📦'; }
function loadFont() {
 if (loading) return loading;
 loading = new Promise(resolve => {
  if (typeof wx.loadFontFace !== 'function' || typeof wx.getFileSystemManager !== 'function') { resolve(false); return; }
  const formats = ['woff2', 'ttf'];
  const attempt = index => {
   if (index >= formats.length) { resolve(false); return; }
   let ended = false;
   const finish = ok => {
    if (ended) return; ended = true; clearTimeout(timer);
    if (ok) { ready = true; for (const listener of listeners) listener(true); resolve(true); }
    else attempt(index + 1);
   };
   const timer = setTimeout(() => finish(false), 2500);
   try {
    wx.getFileSystemManager().readFile({filePath: 'assets/category-icons/category-icons.' + formats[index], encoding: 'base64',
     success: file => {
      if (ended) return;
      try { wx.loadFontFace({global: true, family: 'CostCategoryIcons', source: 'url("data:font/' + (formats[index] === 'ttf' ? 'ttf' : 'woff2') + ';base64,' + file.data + '")', success: () => finish(true), fail: () => finish(false)}); }
      catch (_) { finish(false); }
     }, fail: () => finish(false)});
   } catch (_) { finish(false); }
  };
  attempt(0);
 });
 return loading;
}
function subscribe(listener) { listeners.add(listener); listener(ready); loadFont(); return () => listeners.delete(listener); }
function bannerKey(user, ledger) { return 'cost_icon_banner:' + RELEASE + ':' + user + ':' + ledger; }
function emojiCount(categories) { return categories.filter(c => known(c.system_icon_key) && !known(c.icon_key)).length; }
module.exports = { catalog, known, glyph, fallback, loadFont, subscribe, bannerKey, emojiCount, RELEASE };
