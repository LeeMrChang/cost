const icons = require('../../utils/category-icons');
Component({
 properties: {iconKey: {type: String, value: ''}, emoji: {type: String, value: '📦'}, size: {type: Number, value: 32}, selected: {type: Boolean, value: false}, disabled: {type: Boolean, value: false}},
 data: {fontReady: false, glyph: '', fallback: '📦'},
 observers: {'iconKey, emoji': function(key, emoji) {this.setData({glyph: icons.glyph(key), fallback: icons.fallback(emoji)});}},
 lifetimes: {
  attached() {this.setData({glyph: icons.glyph(this.data.iconKey), fallback: icons.fallback(this.data.emoji)});this.unsubscribe = icons.subscribe(fontReady => this.setData({fontReady}));},
  detached() {if (this.unsubscribe) this.unsubscribe();}
 }
});
