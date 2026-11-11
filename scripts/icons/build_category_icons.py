#!/usr/bin/env python3
"""Build the checked-in category icon assets from catalog.json (see design/category-icons/README.md)."""
import base64
import hashlib
import json
from pathlib import Path
from xml.sax.saxutils import quoteattr
import skia
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.basePen import BasePen
from fontTools.svgLib.path import parse_path
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.pens.cu2quPen import Cu2QuPen
from fontTools.pens.recordingPen import RecordingPen

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'design/category-icons'
OUT = ROOT / 'miniprogram/assets/category-icons'
CATALOG = json.loads((SOURCE / 'catalog.json').read_text())
OUT.mkdir(parents=True, exist_ok=True)
(SOURCE / 'svg').mkdir(exist_ok=True)
(OUT / 'LICENSE').write_text((SOURCE / 'LICENSE').read_text())

class SkiaPen(BasePen):
    def __init__(self): super().__init__(None); self.path=skia.Path()
    def _moveTo(self,p): self.path.moveTo(*p)
    def _lineTo(self,p): self.path.lineTo(*p)
    def _curveToOne(self,p1,p2,p3): self.path.cubicTo(*p1,*p2,*p3)
    def _qCurveToOne(self,p1,p2): self.path.quadTo(*p1,*p2)
    def _closePath(self): self.path.close()
    def _endPath(self): pass

def node_path(tag, a):
    p = skia.Path()
    if tag == 'path':
        pen=SkiaPen();parse_path(a['d'],pen);return pen.path
    if tag == 'circle': p.addCircle(float(a['cx']), float(a['cy']), float(a['r']))
    elif tag == 'ellipse': p.addOval(skia.Rect.MakeLTRB(float(a['cx'])-float(a['rx']),float(a['cy'])-float(a['ry']),float(a['cx'])+float(a['rx']),float(a['cy'])+float(a['ry'])))
    elif tag == 'rect': p.addRoundRect(skia.Rect.MakeXYWH(float(a['x']),float(a['y']),float(a['width']),float(a['height'])),float(a.get('rx',0)),float(a.get('ry',a.get('rx',0))))
    elif tag == 'line': p.moveTo(float(a['x1']),float(a['y1']));p.lineTo(float(a['x2']),float(a['y2']))
    elif tag in ('polyline','polygon'):
        nums=[float(n) for n in a['points'].replace(',',' ').split()]
        p.moveTo(nums[0],nums[1])
        for i in range(2,len(nums),2):p.lineTo(nums[i],nums[i+1])
        if tag=='polygon':p.close()
    else:raise ValueError(tag)
    return p

def draw_to_pen(path, pen):
    iterator = iter(path)
    for verb, pts in iterator:
        xy=lambda p:(p.x(),p.y())
        if verb==skia.Path.kMove_Verb:pen.moveTo(xy(pts[0]))
        elif verb==skia.Path.kLine_Verb:pen.lineTo(xy(pts[1]))
        elif verb==skia.Path.kQuad_Verb:pen.qCurveTo(xy(pts[1]),xy(pts[2]))
        elif verb==skia.Path.kConic_Verb:
            q=skia.Path.ConvertConicToQuads(pts[0],pts[1],pts[2],iterator.conicWeight(),3)
            for i in range(1,len(q),2):pen.qCurveTo(xy(q[i]),xy(q[i+1]))
        elif verb==skia.Path.kCubic_Verb:pen.curveTo(*[xy(p) for p in pts[1:]])
        elif verb==skia.Path.kClose_Verb:pen.closePath()

stroke=skia.Paint(Style=skia.Paint.kStroke_Style,StrokeWidth=1.8,StrokeCap=skia.Paint.kRound_Cap,StrokeJoin=skia.Paint.kRound_Join)
glyphs={'.notdef':TTGlyphPen(None).glyph()}
source_hash=hashlib.sha256(json.dumps(CATALOG,sort_keys=True).encode()).hexdigest()
outline_file=SOURCE/'glyph-outlines.json'
cache=json.loads(outline_file.read_text()) if outline_file.exists() else {}
use_cache=cache.get('source_sha256')==source_hash
outlines={}
for item in CATALOG:
    nodes=item['nodes']
    svg='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">'+''.join('<'+tag+' '+ ' '.join(k+'='+quoteattr(str(v)) for k,v in a.items())+'/>' for tag,a in nodes)+'</svg>\n'
    (SOURCE/'svg'/(item['key']+'.svg')).write_text(svg)
    if use_cache:
        pen=TTGlyphPen(None)
        for op,args in cache['glyphs'][item['key']]:
            getattr(pen,op)(*[tuple(p) if p is not None else None for p in args])
        glyphs[item['key']]=pen.glyph()
        outlines[item['key']]=cache['glyphs'][item['key']]
        continue
    path=skia.Path()
    for tag,a in nodes:
        dst=skia.Path();stroke.getFillPath(node_path(tag,a),dst);path.addPath(skia.AsWinding(dst))
    # Normalize each stroke separately; preserve overlap winding without non-deterministic path union.
    path.transform(skia.Matrix.MakeAll(1000/24,0,0,0,-1000/24,900,0,0,1))
    pen=TTGlyphPen(None);draw_to_pen(path,Cu2QuPen(pen,1.0));glyph=pen.glyph();glyphs[item['key']]=glyph
    recording=RecordingPen();glyph.draw(recording,None);outlines[item['key']]=recording.value
    normalized=TTGlyphPen(None)
    recording.replay(normalized);glyphs[item['key']]=normalized.glyph()

outline_file.write_text(json.dumps({'source_sha256':source_hash,'glyphs':outlines},separators=(',',':'))+'\n')
fb=FontBuilder(1000,isTTF=True)
fb.setupGlyphOrder(list(glyphs));fb.setupCharacterMap({i['codepoint']:i['key'] for i in CATALOG})
fb.setupGlyf(glyphs);fb.setupHorizontalMetrics({key:(1000,0) for key in glyphs});fb.setupHorizontalHeader(ascent=900,descent=-100)
fb.setupNameTable({'familyName':'CostCategoryIcons','styleName':'Regular','uniqueFontIdentifier':'CostCategoryIcons-1','fullName':'Cost Category Icons','psName':'CostCategoryIcons','version':'Version 1.0','copyright':'Lucide contributors ISC; custom glyphs Cost project'})
fb.setupOS2(sTypoAscender=900,sTypoDescender=-100,usWinAscent=1000,usWinDescent=200);fb.setupPost();fb.setupMaxp()
# Fixed timestamps make the generator reproducible.
fb.font.recalcTimestamp=False
fb.font['head'].created=fb.font['head'].modified=3845664000
fb.font.save(OUT/'category-icons.ttf')
fb.font.flavor='woff2';fb.font.save(OUT/'category-icons.woff2')
meta=[{k:v for k,v in i.items() if k not in ('nodes','source')} for i in CATALOG]
(OUT/'catalog.js').write_text('// Generated by scripts/icons/build_category_icons.py; do not edit manually.\nmodule.exports = '+json.dumps(meta,ensure_ascii=False,indent=2)+';\n')
(ROOT/'backend/internal/domain/category_icon_catalog.json').write_text(json.dumps(meta,ensure_ascii=False,indent=2)+'\n')
# Render from the generated font, rather than source paths, to check glyph encoding/outlines.
face=skia.Typeface.MakeFromFile(str(OUT/'category-icons.ttf'))
font=skia.Font(face,30);label_font=skia.Font(skia.Typeface.MakeFromName('Arial',skia.FontStyle()),11)
surface=skia.Surface(1080,850);c=surface.getCanvas();c.clear(skia.ColorWHITE)
for i,item in enumerate(CATALOG):
    x=(i%8)*135;y=(i//8)*135
    for j,(bg,fg) in enumerate([(0xfffff0d5,0xffffb020),(0xffffb020,0xffffffff)]):
        c.drawCircle(x+35+j*55,y+36,23,skia.Paint(Color=bg,AntiAlias=True))
        c.drawString(chr(item['codepoint']),x+20+j*55,y+46,font,skia.Paint(Color=fg,AntiAlias=True))
    c.drawString(item['key'],x+5,y+83,label_font,skia.Paint(Color=skia.ColorBLACK,AntiAlias=True))
    c.drawString('20px',x+5,y+113,label_font,skia.Paint(Color=skia.ColorGRAY))
    c.drawString(chr(item['codepoint']),x+42,y+116,skia.Font(face,20),skia.Paint(Color=skia.ColorBLACK,AntiAlias=True))
surface.makeImageSnapshot().save('/private/tmp/cost-category-icons-preview.png',skia.kPNG)
# Standalone review artifact uses the generated font and covers all 48 keys.
import html
cards=''.join('<article><div class="states"><span class="normal">'+chr(i['codepoint'])+'</span><span class="selected">'+chr(i['codepoint'])+'</span><span class="disabled">'+chr(i['codepoint'])+'</span></div><h3>'+html.escape(i['label'])+'</h3><code>'+i['key']+'</code><p>Emoji 回退 '+i['emoji']+'</p><div class="sizes"><span style="font-size:20px">'+chr(i['codepoint'])+'</span><span style="font-size:32px">'+chr(i['codepoint'])+'</span><span style="font-size:44px">'+chr(i['codepoint'])+'</span></div></article>' for i in CATALOG)
preview='<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>Cost 分类图标预览</title><style>@font-face{font-family:CostCategoryIcons;src:url(../../miniprogram/assets/category-icons/category-icons.woff2)}body{font-family:system-ui;background:#f4f5f7;color:#1a1d21;margin:32px}main{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:16px}article{background:white;border-radius:16px;padding:20px}h3{margin:12px 0 4px}p,code{font-size:12px;color:#68717c}.states,.sizes{display:flex;align-items:center;gap:12px}.states span{font:24px CostCategoryIcons;width:40px;height:40px;display:grid;place-items:center;border-radius:50%;background:#fff0d5;color:#c27a00}.states .selected{background:#ffb020;color:white;outline:2px solid #1a1d21}.states .disabled{filter:grayscale(1);opacity:.5}.sizes{font-family:CostCategoryIcons}</style><h1>Cost · 48 个分类图标</h1><p>未选中／选中／停用；下排是 20／32／44px 线稿本体。此为字体产物预览，不是真机截图。</p><main>'+cards+'</main></html>'
(SOURCE/'preview.html').write_text(preview)
print(json.dumps({'glyphs':len(CATALOG),'ttf':(OUT/'category-icons.ttf').stat().st_size,'woff2':(OUT/'category-icons.woff2').stat().st_size,'base64_woff2':len(base64.b64encode((OUT/'category-icons.woff2').read_bytes()))}))
