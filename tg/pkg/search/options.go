package search

import (
	"strings"
	"unicode"
)

// Normalizer приводит строку к канонической форме (регистр, ё/е и т.п.).
// Применяется и к документам, и к запросам.
type Normalizer func(string) string

// Tokenizer режет нормализованную строку на слова.
type Tokenizer func(string) []string

// config — настройки индекса. Значения по умолчанию задаёт defaultConfig,
// менять их можно через Option.
type config struct {
	normalize Normalizer
	tokenize  Tokenizer

	// ngramSize — длина n-граммы для сравнения слов.
	ngramSize int
	// minScore — слова словаря со схожестью ниже порога игнорируются.
	minScore float64
	// prefixBonus / substringBonus — добавка к схожести, если слово запроса
	// является началом / частью слова из словаря.
	prefixBonus    float64
	substringBonus float64
	// fuzzyMinLen — слова запроса короче (в символах) сравниваются только
	// точно: нечёткое совпадение на 1–2 буквах даёт один мусор.
	fuzzyMinLen int
}

func defaultConfig() config {
	return config{
		normalize:      defaultNormalize,
		tokenize:       defaultTokenize,
		ngramSize:      3,
		minScore:       0.4,
		prefixBonus:    1,
		substringBonus: 0.5,
		fuzzyMinLen:    3,
	}
}

// Option изменяет настройки при создании индекса.
type Option func(*config)

// WithNormalizer заменяет нормализацию текста.
func WithNormalizer(n Normalizer) Option { return func(c *config) { c.normalize = n } }

// WithTokenizer заменяет разбиение на слова.
func WithTokenizer(t Tokenizer) Option { return func(c *config) { c.tokenize = t } }

// WithNGramSize задаёт длину n-граммы (по умолчанию 3).
func WithNGramSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.ngramSize = n
		}
	}
}

// WithMinScore задаёт порог схожести слов (по умолчанию 0.4).
// Выше — строже к опечаткам, ниже — больше шума.
func WithMinScore(s float64) Option { return func(c *config) { c.minScore = s } }

// WithBonuses задаёт добавки за совпадение по префиксу и по подстроке.
func WithBonuses(prefix, substring float64) Option {
	return func(c *config) {
		c.prefixBonus = prefix
		c.substringBonus = substring
	}
}

// WithFuzzyMinLen задаёт минимальную длину слова запроса (в символах),
// начиная с которой допускаются неточные совпадения (по умолчанию 3).
func WithFuzzyMinLen(n int) Option { return func(c *config) { c.fuzzyMinLen = n } }

// defaultNormalize: нижний регистр и ё → е.
func defaultNormalize(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "ё", "е")
}

// defaultTokenize: слово — непрерывная последовательность букв и цифр,
// всё остальное (пробелы, дефисы, знаки препинания) — разделитель.
func defaultTokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
