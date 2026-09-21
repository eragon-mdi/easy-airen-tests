// Package searcher настраивает поиск (pkg/search) под доменную модель вопросов:
// решает, какой текст вопроса индексировать, и переводит настройки из конфига
// в опции движка.
package searcher

import (
	"fmt"
	"strings"

	"tgbot/internal/domain"
	"tgbot/pkg/search"
)

type Config interface {
	NgramSize() int
	MinScore() float64
	PrefixBonus() float64
	SubstringBonus() float64
	FuzzyMinLen() int
	IndexAnswers() bool
}

// config — все настройки поиска. Значения по умолчанию заданы тегами
// env-default и подставляются при чтении конфига (см. internal/cfgs),
// поэтому config{} без загрузки конфига невалиден — см. Validate.
type config struct {
	// NGramSize — длина n-граммы, на которые режутся слова для сравнения.
	// Меньше (2) — терпимее к опечаткам, но больше шума; больше (4) — строже.
	NGramSize int

	// MinScore — порог схожести слова запроса со словом из вопросов (0..1+).
	// Выше — меньше ложных совпадений, но и опечатки прощаются хуже.
	MinScore float64

	// PrefixBonus — добавка к оценке, если слово запроса — начало слова из вопроса
	// («полупр» → «полупроводнике»). Также действует на точное совпадение.
	PrefixBonus float64

	// SubstringBonus — добавка, если слово запроса — часть слова, но не начало
	// («водник» → «полупроводнике»).
	SubstringBonus float64

	// FuzzyMinLen — слова запроса короче (в символах) ищутся только точно:
	// нечёткий поиск по 1–2 буквам («n») даёт один мусор.
	FuzzyMinLen int

	// IndexAnswers — искать ещё и по тексту вариантов ответа (только select).
	// Выключено: поиск идёт только по формулировке вопроса.
	IndexAnswers bool
}

// Validate проверяет, что настройки имеют смысл.
func (c config) Validate() error {
	switch {
	case c.NGramSize < 1:
		return fmt.Errorf("searcher: NGramSize должен быть >= 1, получено %d", c.NGramSize)
	case c.MinScore < 0:
		return fmt.Errorf("searcher: MinScore должен быть >= 0, получено %v", c.MinScore)
	case c.PrefixBonus < 0 || c.SubstringBonus < 0:
		return fmt.Errorf("searcher: бонусы должны быть >= 0")
	case c.FuzzyMinLen < 1:
		return fmt.Errorf("searcher: FuzzyMinLen должен быть >= 1, получено %d", c.FuzzyMinLen)
	}
	return nil
}

// options переводит конфиг в опции движка поиска.
func (c config) options() []search.Option {
	return []search.Option{
		search.WithNGramSize(c.NGramSize),
		search.WithMinScore(c.MinScore),
		search.WithBonuses(c.PrefixBonus, c.SubstringBonus),
		search.WithFuzzyMinLen(c.FuzzyMinLen),
	}
}

// New строит поисковый индекс по вопросам. В индексе ID документа — это
// domain.Question.ID, поэтому результат поиска сопоставляется с вопросом по нему.
func New(questions []domain.Question, c Config) (search.Searcher, error) {
	cfg := &config{
		NGramSize:      c.NgramSize(),
		MinScore:       c.MinScore(),
		PrefixBonus:    c.PrefixBonus(),
		SubstringBonus: c.SubstringBonus(),
		FuzzyMinLen:    c.FuzzyMinLen(),
		IndexAnswers:   c.IndexAnswers(),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	docs := make([]search.Document, len(questions))
	for i := range questions {
		docs[i] = search.Document{ID: questions[i].ID, Text: textOf(&questions[i], cfg)}
	}
	return search.New(docs, cfg.options()...), nil
}

// textOf собирает индексируемый текст вопроса: формулировка и, если включено,
// тексты вариантов ответа. Картинки в текст не попадают.
func textOf(q *domain.Question, cfg *config) string {
	var b strings.Builder
	writeContent(&b, q.Title)

	if cfg.IndexAnswers {
		for _, a := range q.Answers {
			b.WriteByte('\n')
			writeContent(&b, a.Content)
		}
	}
	return b.String()
}

// writeContent дописывает текстовые блоки контента. Соседние текстовые блоки
// склеиваются без разделителя (как при показе), перенос строки — пробельный.
func writeContent(b *strings.Builder, c domain.Content) {
	for _, blk := range c {
		switch blk.Kind {
		case domain.KindText:
			b.WriteString(blk.Value)
		case domain.KindBR:
			b.WriteByte('\n')
		}
	}
}
