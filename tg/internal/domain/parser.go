package domain

import (
	"fmt"
	"strings"
)

// Bank — весь разобранный файл.
type Bank struct {
	Meta      Meta
	Questions []Question   // в порядке появления в файле; пропущенных (см. Errors) здесь нет
	Errors    []ParseError // все найденные проблемы в порядке появления
}

// Meta — сводка по слиянию исходных файлов (для самих вопросов не нужна).
type Meta struct {
	QuestionsTotal      int
	ConflictGroupsTotal int
	Files               []MetaFile
}

// MetaFile — статистика по одному исходному файлу.
type MetaFile struct {
	Name              string
	Questions         int
	DuplicatesInFile  int
	AlreadyInPrevious int
}

// Типы вопросов (атрибут type).
const (
	TypeSelect   = "select"
	TypeInput    = "input"
	TypeMatch    = "match"
	TypeClassify = "classify"
)

// Question — один вопрос. Заполнены только поля, относящиеся к его типу:
// select → Answers, input → Patterns, match → Pairs и Distractors,
// classify → Groups.
type Question struct {
	ID            int
	Type          string
	Enabled       bool
	Weight        float64
	Multiple      bool     // только select
	ConflictGroup *int     // nil, если атрибута нет
	Category      []string // категорий, например ["Задачи", "Задача ТТЛ"]

	Title   Content
	Sources []string
	Scripts []string // пусто у вопросов без параметров

	Answers     []Answer  // select
	Patterns    []Pattern // input
	Pairs       []Pair    // match
	Distractors []Content // match; может отсутствовать
	Groups      []Group   // classify
}

// Answer — вариант ответа в вопросе select.
type Answer struct {
	Correct bool
	Content Content
}

// Pattern — допустимый ответ в вопросе input.
type Pattern struct {
	Value         string  // может содержать $(имя) — переменную из скрипта
	Quality       float64 // доля баллов: 1.0 — полный ответ
	Wildcard      bool    // * в Value означает «любые символы»
	CaseSensitive bool    // важен ли регистр
}

// Pair — верная пара в вопросе match.
type Pair struct {
	Left  Content
	Right Content
}

// Group — категория для раскладки элементов в вопросе classify
// (не путать с категорией раздела теста).
type Group struct {
	Title Content
	Items []Content // элементы, которые правильно относятся к группе
}

// Виды блоков контента.
const (
	KindText = "text"
	KindBR   = "br"
	KindImg  = "img"
)

// ParseError описывает одну проблему и место, где она найдена.
type ParseError struct {
	Severity   Severity
	Line       int      // строка файла (для вопроса — строка его открывающего тега)
	QuestionID int      // id вопроса; 0, если неизвестен
	Category   []string // путь категорий, в которой найдена проблема
	Err        error
}

func (e ParseError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: строка %d", e.Severity, e.Line)
	if e.QuestionID != 0 {
		fmt.Fprintf(&b, ", вопрос %d", e.QuestionID)
	}
	if len(e.Category) > 0 {
		fmt.Fprintf(&b, " [%s]", strings.Join(e.Category, " / "))
	}
	fmt.Fprintf(&b, ": %v", e.Err)
	return b.String()
}

func (e ParseError) Unwrap() error { return e.Err }

// Block — один элемент контента.
type Block struct {
	Kind  string // KindText, KindBR или KindImg
	Value string // текст (только для KindText), без обрезки пробелов
	Src   string // путь к картинке относительно result.xml (только для KindImg)
}

// Content — блок контента: дочерние теги <text>, <br/>, <img/> в порядке отображения.
type Content []Block

// Severity — серьёзность проблемы.
type Severity int

const (
	// Warning — вопрос прочитан, но что-то с ним не так (лишний тег, нет ответов).
	Warning Severity = iota
	// Error — вопрос (или meta) не удалось разобрать, он пропущен.
	Error
	// Fatal — синтаксис XML нарушен, дальнейшее чтение невозможно.
	Fatal
)

func (s Severity) String() string {
	switch s {
	case Warning:
		return "предупреждение"
	case Error:
		return "ошибка"
	default:
		return "критическая ошибка"
	}
}
