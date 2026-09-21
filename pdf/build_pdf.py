#!/usr/bin/env python3
"""result.xml (только чтение) -> questions.html -> questions.pdf (headless Chrome)."""
import html, subprocess, xml.etree.ElementTree as ET
from pathlib import Path

HERE = Path(__file__).parent
SRC = HERE.parent / "out" / "merged"
esc = html.escape


def content(el):
    s = ""
    for c in el:
        if c.tag == "text":
            s += esc(c.text or "")
        elif c.tag == "br":
            s += "<br>"
        elif c.tag == "img":
            s += f'<img src="{esc(c.get("src"))}">'
    return s


def render_question(q, n_total, path):
    t = q.get("type")
    flags = ""
    if q.get("enabled") == "false":
        flags += '<span class="flag off">отключён</span>'
    if q.get("conflictGroup"):
        flags += f'<span class="flag warn">группа сомнений {q.get("conflictGroup")}</span>'
    if q.find("scripts") is not None:
        flags += '<span class="flag par">с параметрами</span>'
    h = [f'<section class="q" id="q{q.get("id")}">',
         f'<div class="bar"><span class="num">Вопрос {q.get("id")} из {n_total}</span>'
         f'<span class="cat">{esc(" › ".join(path))}</span>{flags}</div>',
         f'<div class="title">{content(q.find("title"))}</div>']
    if t == "select":
        multi = q.get("multiple") == "true"
        h.append('<div class="hint">' + ("Выберите все правильные ответы" if multi else "Выберите один правильный ответ") + "</div>")
        for a in q.findall("answers/answer"):
            ok = a.get("correct") == "true"
            mark = ("☑" if ok else "☐") if multi else ("◉" if ok else "○")
            h.append(f'<div class="opt{" ok" if ok else ""}"><span class="mk">{mark}</span>'
                     f'<span class="ot">{content(a)}</span>{"<span class=tick>✓</span>" if ok else ""}</div>')
    elif t == "input":
        h.append('<div class="hint">Введите ответ. Допустимые ответы (правильный ответ):</div>')
        for p in q.findall("patterns/pattern"):
            notes = []
            if p.get("wildcard") == "true":
                notes.append("* — любые символы")
            if p.get("caseSensitive") == "true":
                notes.append("учитывается регистр")
            if float(p.get("quality")) != 1:
                notes.append(f'{float(p.get("quality")) * 100:.0f}% баллов')
            n = f' <span class="note">({"; ".join(notes)})</span>' if notes else ""
            h.append(f'<div class="field ok">{esc(p.get("value"))}</div>{n}')
    elif t == "match":
        h.append('<div class="hint">Установите соответствие</div><table class="match">')
        for p in q.findall("pairs/pair"):
            h.append(f'<tr><td class="l">{content(p.find("left"))}</td><td class="ar">⟶</td>'
                     f'<td class="r ok">{content(p.find("right"))}</td></tr>')
        h.append("</table>")
        d = q.findall("distractors/distractor")
        if d:
            h.append('<div class="dis"><span class="dl">Лишние варианты:</span> ' +
                     " ".join(f'<span class="di">{content(x)}</span>' for x in d) + "</div>")
    elif t == "classify":
        h.append('<div class="hint">Распределите элементы по категориям</div><div class="cls">')
        for g in q.findall("groups/group"):
            h.append('<div class="grp"><div class="gt">' + content(g.find("groupTitle")) + "</div>" +
                     "".join(f'<div class="gi ok">{content(i)}</div>' for i in g.findall("item")) + "</div>")
        h.append("</div>")
    src = ", ".join(s.text for s in q.findall("sources/source"))
    h.append(f'<div class="src">Источник: {esc(src)}</div></section>')
    return "\n".join(h)


CSS = """
@page { size: A4; margin: 14mm 12mm 16mm;
  @bottom-center { content: counter(page) " / " counter(pages); font: 9px sans-serif; color: #666; } }
* { box-sizing: border-box; }
body { font: 13px/1.4 "DejaVu Sans", "Liberation Sans", sans-serif; color: #111; margin: 0; }
h1 { font-size: 22px; margin: 0 0 4px; }
.sub { color: #555; margin-bottom: 14px; }
.toc a { color: #1a4f8b; text-decoration: none; }
.toc li { margin: 2px 0; }
.toc .n { color: #777; }
.legend { border: 1px solid #ccc; padding: 8px 10px; margin: 12px 0; background: #fafafa; }
h2 { font-size: 16px; background: #1a4f8b; color: #fff; padding: 6px 10px; margin: 18px 0 8px;
  break-after: avoid; }
h3 { font-size: 14px; color: #1a4f8b; border-bottom: 2px solid #1a4f8b; padding: 2px 0; margin: 12px 0 6px;
  break-after: avoid; }
.q { border: 1px solid #9aa7b8; border-radius: 3px; margin: 0 0 9px; background: #fff;
  break-inside: avoid; }
.bar { background: linear-gradient(#e9eef6, #d4deec); border-bottom: 1px solid #9aa7b8; padding: 3px 8px;
  font-size: 11px; display: flex; gap: 8px; align-items: center; }
.num { font-weight: bold; }
.cat { color: #445; flex: 1; }
.flag { font-size: 10px; padding: 0 5px; border-radius: 8px; border: 1px solid; }
.flag.off { color: #a00; border-color: #a00; } .flag.warn { color: #a60; border-color: #a60; }
.flag.par { color: #06a; border-color: #06a; }
.title { padding: 8px 10px 4px; font-size: 14px; font-weight: 600; }
.hint { padding: 0 10px 4px; font-size: 11px; color: #666; font-style: italic; }
img { max-width: 100%; vertical-align: middle; }
.title img { display: block; margin: 6px 0; }
.opt { display: flex; gap: 8px; align-items: center; padding: 3px 10px; margin: 0 6px 2px; border-radius: 2px; }
.opt.ok, .field.ok, .r.ok, .gi.ok { background: #dff3df; box-shadow: inset 0 0 0 1px #6bb56b; }
.mk { font-size: 16px; width: 18px; text-align: center; } .ot { flex: 1; }
.tick { color: #237a23; font-weight: bold; }
.field { display: inline-block; min-width: 220px; margin: 2px 10px; padding: 3px 8px; font-family: monospace;
  border: 1px solid #888; background: #fff; }
.field.ok { background: #dff3df; }
.note { font-size: 11px; color: #666; }
.match { border-collapse: separate; border-spacing: 4px 3px; margin: 0 6px 4px; }
.match td { padding: 3px 8px; border: 1px solid #b8c0cc; background: #f6f8fb; vertical-align: middle; }
.match td.ar { border: 0; background: none; color: #237a23; font-weight: bold; padding: 0 2px; }
.dis { margin: 2px 10px 6px; font-size: 12px; } .dl { color: #a00; }
.di { display: inline-block; padding: 2px 6px; border: 1px dashed #a00; margin-right: 6px; }
.cls { display: flex; gap: 8px; padding: 0 8px 6px; }
.grp { flex: 1; border: 1px solid #b8c0cc; } .gt { background: #e4e9f1; font-weight: bold; padding: 3px 6px; }
.gi { margin: 3px; padding: 3px 6px; }
.src { font-size: 10px; color: #888; padding: 3px 10px; border-top: 1px solid #e3e6ea; text-align: right; }
"""


def main():
    root = ET.parse(SRC / "result.xml").getroot()
    total = len(root.findall(".//question"))
    toc, body = [], []

    def walk(node, path):
        for ch in node:
            if ch.tag == "category":
                p = path + [ch.get("name")]
                cid = "c" + str(len(toc))
                cnt = len(ch.findall(".//question"))
                toc.append((len(p), cid, ch.get("name"), cnt))
                body.append(f'<{"h2" if len(p) == 1 else "h3"} id="{cid}">{esc(ch.get("name"))}</{"h2" if len(p) == 1 else "h3"}>')
                walk(ch, p)
            elif ch.tag == "question":
                body.append(render_question(ch, total, path))
    walk(root, [])

    toc_html = "<ol class='toc'>" + "".join(
        f'<li style="margin-left:{(lvl - 1) * 18}px"><a href="#{cid}">{esc(name)}</a> <span class="n">— {cnt}</span></li>'
        for lvl, cid, name, cnt in toc) + "</ol>"
    doc = f"""<!doctype html><html lang="ru"><meta charset="utf-8"><title>Банк вопросов АВТИ</title>
<style>{CSS}</style><body>
<h1>Банк тестовых вопросов АВТИ</h1>
<div class="sub">Объединённый тест (без повторов): {total} вопросов. Оформление вдохновлено окном тестирования Айрен.</div>
<div class="legend"><b>Как читать:</b> правильные ответы выделены <span style="background:#dff3df;box-shadow:inset 0 0 0 1px #6bb56b;padding:0 4px">зелёным</span>
и отмечены (◉ / ☑ / ✓). Для вопросов «с параметрами» значения вида <code>$(x)</code> в реальном тесте подставляются случайно.</div>
<h3>Содержание</h3>{toc_html}
<div style="break-after:page"></div>
{chr(10).join(body)}
</body></html>"""
    # картинки лежат в out/merged/images — HTML кладём рядом с ними ссылкой через <base>
    doc = doc.replace("<meta charset", f'<base href="{SRC.as_uri()}/"><meta charset', 1)
    (HERE / "questions.html").write_text(doc, encoding="utf-8")
    subprocess.run(["google-chrome", "--headless=new", "--no-sandbox", "--disable-gpu",
                    "--no-pdf-header-footer", f"--print-to-pdf={HERE / 'questions.pdf'}",
                    (HERE / "questions.html").as_uri()], check=True, capture_output=True)
    print("ok", total, "вопросов")


main()
