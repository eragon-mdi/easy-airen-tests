package search

import (
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"
)

// padRune окаймляет слово при нарезке на n-граммы, чтобы начало и конец слова
// тоже давали отдельные n-граммы (важно для коротких слов и префиксов).
const padRune = '\x00'

// Index — реализация Searcher. После построения не меняется,
// поэтому безопасен для одновременного использования из нескольких горутин.
type Index struct {
	cfg config

	vocab []word           // уникальные слова всех документов
	inv   map[string][]int // слово -> индексы документов (позиции в docIDs)
	docs  []int            // позиция документа -> его внешний ID
}

// word — слово словаря с заранее посчитанными n-граммами.
type word struct {
	text  string
	grams []string // без повторов
}

var _ Searcher = (*Index)(nil)

// New строит индекс по документам. Документы с пустым текстом допустимы,
// но найти их нельзя.
func New(docs []Document, opts ...Option) *Index {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	ix := &Index{
		cfg:  cfg,
		inv:  make(map[string][]int),
		docs: make([]int, len(docs)),
	}

	// wordPos — позиция слова в словаре, чтобы не дублировать слова.
	wordPos := make(map[string]struct{})

	for pos, d := range docs {
		ix.docs[pos] = d.ID

		// Слово, повторённое в одном документе, регистрируем один раз.
		seen := make(map[string]struct{})
		for _, w := range ix.words(d.Text) {
			if _, dup := seen[w]; dup {
				continue
			}
			seen[w] = struct{}{}

			if _, known := wordPos[w]; !known {
				wordPos[w] = struct{}{}
				ix.vocab = append(ix.vocab, word{text: w, grams: ngrams(w, cfg.ngramSize)})
			}
			ix.inv[w] = append(ix.inv[w], pos)
		}
	}
	return ix
}

// Search реализует Searcher.
func (ix *Index) Search(query string, limit int) []Result {
	// scores: позиция документа -> накопленная оценка.
	scores := make(map[int]float64)

	for _, q := range ix.words(query) {
		// Для каждого слова запроса ищем лучшую оценку по каждому документу:
		// слово запроса засчитывается документу один раз (лучшим совпадением),
		// сколько бы похожих слов в нём ни было.
		for pos, s := range ix.matchWord(q) {
			scores[pos] += s
		}
	}

	res := make([]Result, 0, len(scores))
	for pos, s := range scores {
		res = append(res, Result{ID: ix.docs[pos], Score: s})
	}

	// Порядок детерминирован: сначала оценка, при равенстве — меньший ID.
	slices.SortFunc(res, func(a, b Result) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})

	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res
}

// matchWord возвращает для слова запроса: позиция документа -> лучшая оценка.
func (ix *Index) matchWord(q string) map[int]float64 {
	best := make(map[int]float64)
	add := func(w string, score float64) {
		for _, pos := range ix.inv[w] {
			if score > best[pos] {
				best[pos] = score
			}
		}
	}

	// Короткое слово: только точное совпадение, без нечёткого поиска.
	if utf8.RuneCountInString(q) < ix.cfg.fuzzyMinLen {
		add(q, 1+ix.cfg.prefixBonus)
		return best
	}

	// Набор n-грамм запроса строим один раз — он же проверяется для каждого слова словаря.
	qGrams := ngrams(q, ix.cfg.ngramSize)
	qSet := make(map[string]struct{}, len(qGrams))
	for _, g := range qGrams {
		qSet[g] = struct{}{}
	}

	for _, w := range ix.vocab {
		s := dice(qSet, len(qGrams), w.grams)

		// Запрос — начало или часть слова: пользователь мог ввести
		// слово не целиком, поэтому такие слова поощряем.
		switch {
		case strings.HasPrefix(w.text, q):
			s += ix.cfg.prefixBonus
		case strings.Contains(w.text, q):
			s += ix.cfg.substringBonus
		}

		if s >= ix.cfg.minScore {
			add(w.text, s)
		}
	}
	return best
}

// words нормализует строку и режет на слова.
func (ix *Index) words(s string) []string {
	return ix.cfg.tokenize(ix.cfg.normalize(s))
}

// ngrams режет слово (с паддингом по краям) на n-граммы без повторов.
// Слово короче n даёт одну n-грамму — само слово с паддингом.
func ngrams(w string, n int) []string {
	r := make([]rune, 0, utf8.RuneCountInString(w)+2)
	r = append(r, padRune)
	r = append(r, []rune(w)...)
	r = append(r, padRune)

	if len(r) <= n {
		return []string{string(r)}
	}

	out := make([]string, 0, len(r)-n+1)
	seen := make(map[string]struct{}, len(r)-n+1)
	for i := 0; i+n <= len(r); i++ {
		g := string(r[i : i+n])
		if _, dup := seen[g]; dup {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	return out
}

// dice — коэффициент Дайса двух наборов n-грамм: 2·|A∩B| / (|A|+|B|), от 0 до 1.
// Первый набор передан как множество (qSet) и его размер (qLen),
// второй — как слайс уникальных n-грамм.
func dice(qSet map[string]struct{}, qLen int, grams []string) float64 {
	common := 0
	for _, g := range grams {
		if _, ok := qSet[g]; ok {
			common++
		}
	}
	return 2 * float64(common) / float64(qLen+len(grams))
}
