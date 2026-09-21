package tgbot

import (
	"fmt"
	"strings"
	"unicode"
)

// normalize приводит формулировку к виду для сравнения: без регистра, знаков
// препинания и лишних пробелов, «ё» = «е». Слова НЕ склеиваем «по смыслу»:
// вопросы «открытого» и «закрытого» диода отличаются одним словом и должны
// оставаться разными.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// groupItems объединяет варианты с одинаковой формулировкой. Возвращает группы
// как списки индексов в items; порядок групп и внутри них — как в выдаче поиска
// (то есть по релевантности первого вопроса группы).
func groupItems(items []Item) [][]int {
	var groups [][]int
	byKey := map[string]int{}
	for i, it := range items {
		key := normalize(it.Question)
		if key == "" { // формулировки нет (только картинка) — не группируем
			key = fmt.Sprintf("\x00%d", i)
		}
		g, ok := byKey[key]
		if !ok {
			g = len(groups)
			byKey[key] = g
			groups = append(groups, nil)
		}
		groups[g] = append(groups[g], i)
	}
	return groups
}

func allIndexes(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}
