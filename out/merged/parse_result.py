#!/usr/bin/env python3
"""Пример: result.xml -> result.json (плоский список вопросов)."""
import json
import xml.etree.ElementTree as ET


def content(el):
    """Содержимое (<title>, <answer>, <left> ...) -> список блоков в исходном порядке."""
    out = []
    for c in el:
        if c.tag == "text":
            out.append({"kind": "text", "value": c.text or ""})
        elif c.tag == "br":
            out.append({"kind": "br"})
        elif c.tag == "img":
            out.append({"kind": "img", "src": c.get("src")})
    return out


def parse_question(q, path):
    d = {
        "id": int(q.get("id")),
        "type": q.get("type"),
        "category": path,                       # ["Задачи", "Задача ТТЛ"]
        "enabled": q.get("enabled") == "true",
        "weight": float(q.get("weight")),
        "conflictGroup": int(q.get("conflictGroup")) if q.get("conflictGroup") else None,
        "title": content(q.find("title")),
        "sources": [s.text for s in q.findall("sources/source")],
    }
    t = d["type"]
    if t == "select":
        d["multiple"] = q.get("multiple") == "true"
        d["answers"] = [{"correct": a.get("correct") == "true", "content": content(a)}
                        for a in q.findall("answers/answer")]
    elif t == "input":
        d["patterns"] = [{"value": p.get("value"), "quality": float(p.get("quality")),
                          "wildcard": p.get("wildcard") == "true",
                          "caseSensitive": p.get("caseSensitive") == "true"}
                         for p in q.findall("patterns/pattern")]
    elif t == "match":
        d["pairs"] = [{"left": content(p.find("left")), "right": content(p.find("right"))}
                      for p in q.findall("pairs/pair")]
        d["distractors"] = [content(x) for x in q.findall("distractors/distractor")]
    elif t == "classify":
        d["groups"] = [{"title": content(g.find("groupTitle")),
                        "items": [content(i) for i in g.findall("item")]}
                       for g in q.findall("groups/group")]
    d["scripts"] = [s.text or "" for s in q.findall("scripts/script")]
    return d


def walk(node, path, out):
    for ch in node:
        if ch.tag == "category":
            walk(ch, path + [ch.get("name")], out)
        elif ch.tag == "question":
            out.append(parse_question(ch, path))


root = ET.parse("result.xml").getroot()
questions = []
walk(root, [], questions)
json.dump(questions, open("result.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(len(questions), "вопросов ->", "result.json")
