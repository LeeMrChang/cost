# 分类线性图标源素材

48 个图标，基于 Lucide 1.8.0（ISC，许可见 LICENSE）。午餐便当盒、红包、退税、钱袋、梳子、跑步、还款、鞋履和水龙头图形由项目绘制，其余从已安装 Lucide 图标节点导出；没有从参考应用提取素材。source 字段保留来源名，catalog.json 为可编辑节点及稳定 key/码点清单。

用临时环境安装 fonttools==4.60.2、brotli==1.2.1、skia-python==144.1，再运行 `python scripts/icons/build_category_icons.py`。生成本地字体、SVG源、前后端清单及 /private/tmp 预览；不需要运行时安装这些依赖。

glyph-outlines.json 保存与 catalog 源哈希对应的字体轮廓，避免底层描边展开的轮廓顺序变化影响重复构建。源节点变化时脚本自动重新展开并更新它；提交时同时检查 SVG、轮廓与预览。

小程序运行时清单生成 CommonJS `catalog.js`，不使用 Node 专有的 JSON require 扩展；后端仍保留 JSON 嵌入清单。生成后运行 `node --test miniprogram/tests/wechat-modules.test.js` 检查启动依赖兼容。
