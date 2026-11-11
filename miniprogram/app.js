const categoryIcons = require('./utils/category-icons');
App({ onLaunch() { categoryIcons.loadFont(); }, globalData: { pendingEdit: null } });
