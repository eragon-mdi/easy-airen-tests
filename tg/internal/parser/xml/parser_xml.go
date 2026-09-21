// Пакет parserxml разбирает банк тестовых вопросов result.xml
// (формат описан в docs/parser/parsing.md) в доменные структуры domain.Bank.
//
// Вопросы возвращаются плоским списком в порядке файла; путь по категориям
// хранится в Question.Category. Содержимое (заголовки, ответы и т.д.)
// разбирается в упорядоченный список блоков Content — текст, перенос, картинка.
//
// Парсер устойчив к ошибкам: битый вопрос пропускается, остальные читаются
// дальше, а все проблемы (с номером строки, id вопроса и категорией)
// собираются в Bank.Errors. Из Parse возвращается ошибка только при
// критической проблеме (битый синтаксис XML, ошибка чтения), и даже тогда
// Bank содержит всё, что удалось прочитать до неё.
package parserxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"tgbot/internal/domain"
)

// Внутренние xml-структуры. Знание о формате файла живёт только здесь, а
// доменные типы от encoding/xml не зависят: результат разбора переносится
// в них функциями to* ниже.

type xmlMeta struct {
	QuestionsTotal      int           `xml:"questionsTotal"`
	ConflictGroupsTotal int           `xml:"conflictGroupsTotal"`
	Files               []xmlMetaFile `xml:"file"`
}

type xmlMetaFile struct {
	Name              string `xml:"name,attr"`
	Questions         int    `xml:"questions,attr"`
	DuplicatesInFile  int    `xml:"duplicatesInFile,attr"`
	AlreadyInPrevious int    `xml:"alreadyInPrevious,attr"`
}

type xmlQuestion struct {
	ID            int     `xml:"id,attr"`
	Type          string  `xml:"type,attr"`
	Enabled       bool    `xml:"enabled,attr"`
	Weight        float64 `xml:"weight,attr"`
	Multiple      bool    `xml:"multiple,attr"`
	ConflictGroup *int    `xml:"conflictGroup,attr"` // nil, если атрибута нет

	Title   xmlContent `xml:"title"`
	Sources []string   `xml:"sources>source"`
	Scripts []string   `xml:"scripts>script"`

	Answers     []xmlAnswer  `xml:"answers>answer"`
	Patterns    []xmlPattern `xml:"patterns>pattern"`
	Pairs       []xmlPair    `xml:"pairs>pair"`
	Distractors []xmlContent `xml:"distractors>distractor"`
	Groups      []xmlGroup   `xml:"groups>group"`
}

type xmlAnswer struct {
	Correct bool
	Content xmlContent
}

type xmlPattern struct {
	Value         string  `xml:"value,attr"`
	Quality       float64 `xml:"quality,attr"`
	Wildcard      bool    `xml:"wildcard,attr"`
	CaseSensitive bool    `xml:"caseSensitive,attr"`
}

type xmlPair struct {
	Left  xmlContent `xml:"left"`
	Right xmlContent `xml:"right"`
}

type xmlGroup struct {
	Title xmlContent   `xml:"groupTitle"`
	Items []xmlContent `xml:"item"`
}

// xmlContent — блок контента: дочерние теги <text>, <br/>, <img/> в порядке отображения.
type xmlContent []domain.Block

// UnmarshalXML собирает дочерние элементы по порядку, не теряя чередования
// текста, переносов и картинок. Неизвестные теги пропускаются
// (о них сообщает проверка схемы в parse).
func (c *xmlContent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*c = nil
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case domain.KindText:
				var s string
				if err := d.DecodeElement(&s, &t); err != nil {
					return err
				}
				*c = append(*c, domain.Block{Kind: domain.KindText, Value: s})
			case domain.KindBR:
				*c = append(*c, domain.Block{Kind: domain.KindBR})
				if err := d.Skip(); err != nil {
					return err
				}
			case domain.KindImg:
				*c = append(*c, domain.Block{Kind: domain.KindImg, Src: attr(t, "src")})
				if err := d.Skip(); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil // закрылся сам элемент, начатый в start
		}
	}
}

// UnmarshalXML читает атрибут correct и содержимое варианта как блок контента.
func (a *xmlAnswer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	a.Correct = attr(start, "correct") == "true"
	return a.Content.UnmarshalXML(d, start)
}

// attr возвращает значение атрибута элемента или "" если его нет.
func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// mapSlice применяет f к каждому элементу; nil остаётся nil.
func mapSlice[T, U any](in []T, f func(T) U) []U {
	if in == nil {
		return nil
	}
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func toContent(c xmlContent) domain.Content { return domain.Content(c) }

func toMeta(m xmlMeta) domain.Meta {
	return domain.Meta{
		QuestionsTotal:      m.QuestionsTotal,
		ConflictGroupsTotal: m.ConflictGroupsTotal,
		Files: mapSlice(m.Files, func(f xmlMetaFile) domain.MetaFile {
			return domain.MetaFile(f) // поля совпадают
		}),
	}
}

func toQuestion(x *xmlQuestion, category []string) domain.Question {
	return domain.Question{
		ID:            x.ID,
		Type:          x.Type,
		Enabled:       x.Enabled,
		Weight:        x.Weight,
		Multiple:      x.Multiple,
		ConflictGroup: x.ConflictGroup,
		Category:      category,
		Title:         toContent(x.Title),
		Sources:       x.Sources,
		Scripts:       x.Scripts,
		Answers: mapSlice(x.Answers, func(a xmlAnswer) domain.Answer {
			return domain.Answer{Correct: a.Correct, Content: toContent(a.Content)}
		}),
		Patterns: mapSlice(x.Patterns, func(p xmlPattern) domain.Pattern {
			return domain.Pattern(p) // поля совпадают
		}),
		Pairs: mapSlice(x.Pairs, func(p xmlPair) domain.Pair {
			return domain.Pair{Left: toContent(p.Left), Right: toContent(p.Right)}
		}),
		Distractors: mapSlice(x.Distractors, toContent),
		Groups: mapSlice(x.Groups, func(g xmlGroup) domain.Group {
			return domain.Group{Title: toContent(g.Title), Items: mapSlice(g.Items, toContent)}
		}),
	}
}

// ParseFile разбирает файл result.xml по пути path.
// Возвращаемые значения устроены как у Parse.
func ParseFile(path string) (*domain.Bank, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

// Parse разбирает банк вопросов из r.
//
// Ошибка возвращается только при критической проблеме (Severity == Fatal):
// нарушен синтаксис XML или файл обрезан. Тогда Bank всё равно не nil и
// содержит вопросы, прочитанные до этого места; сама ошибка (типа
// domain.ParseError) продублирована последним элементом Bank.Errors.
// Некритичные проблемы (пропущенный вопрос, предупреждения) ошибкой не
// возвращаются — смотрите Bank.Errors. При ошибке чтения r Bank равен nil.
func Parse(r io.Reader) (*domain.Bank, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parserxml: %w", err)
	}
	return parse(data)
}

func parse(data []byte) (*domain.Bank, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	bank := &domain.Bank{}
	var path []string      // текущий путь по вложенным категориям
	seen := map[int]bool{} // уже встреченные id вопросов

	// report добавляет проблему в bank.Errors.
	report := func(sev domain.Severity, line, qid int, err error) domain.ParseError {
		pe := domain.ParseError{Severity: sev, Line: line, QuestionID: qid, Category: slices.Clone(path), Err: err}
		bank.Errors = append(bank.Errors, pe)
		return pe
	}
	// fatal фиксирует критическую ошибку потока и завершает разбор.
	fatal := func(err error) (*domain.Bank, error) {
		line, _ := d.InputPos()
		var se *xml.SyntaxError
		if errors.As(err, &se) {
			line = se.Line
		}
		return bank, report(domain.Fatal, line, 0, err)
	}

	for {
		off := d.InputOffset() // позиция начала следующего токена
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return bank, nil
		}
		if err != nil {
			return fatal(err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "result":
				// корень: просто идём внутрь
			case "category":
				path = append(path, attr(t, "name"))
			case "meta", "question":
				// Пролистываем элемент целиком (синтаксис проверяется здесь),
				// а разбираем его отдельно по вырезанному куску: ошибка значения
				// внутри не портит состояние основного потока.
				if err := d.Skip(); err != nil {
					return fatal(err)
				}
				chunk := data[off:d.InputOffset()]
				line := lineAt(data, off)
				if t.Name.Local == "meta" {
					var m xmlMeta
					if err := xml.Unmarshal(chunk, &m); err != nil {
						report(domain.Error, line, 0, fmt.Errorf("meta пропущена: %w", err))
						continue
					}
					bank.Meta = toMeta(m)
					continue
				}

				qid, _ := strconv.Atoi(attr(t, "id"))
				var xq xmlQuestion
				if err := xml.Unmarshal(chunk, &xq); err != nil {
					report(domain.Error, line, qid, fmt.Errorf("вопрос пропущен: %w", err))
					continue
				}
				q := toQuestion(&xq, slices.Clone(path))

				for _, w := range unknownTags(chunk, line) {
					report(domain.Warning, w.line, q.ID, w.err)
				}
				for _, err := range validate(&q) {
					report(domain.Warning, line, q.ID, err)
				}
				if seen[q.ID] {
					report(domain.Warning, line, q.ID, errors.New("повторяющийся id вопроса"))
				}
				seen[q.ID] = true

				bank.Questions = append(bank.Questions, q)
			default:
				if err := d.Skip(); err != nil {
					return fatal(err)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "category" {
				path = path[:len(path)-1]
			}
		}
	}
}

// lineAt возвращает номер строки (с 1) для смещения off в data.
func lineAt(data []byte, off int64) int {
	return 1 + bytes.Count(data[:off], []byte("\n"))
}

// contentTags — теги, допустимые внутри блока контента.
var contentTags = []string{domain.KindText, domain.KindBR, domain.KindImg}

// schema — допустимые дочерние теги для каждого тега внутри <question>.
// Тегов, которых здесь нет (text, br, img, pattern, source, script), листовые.
var schema = map[string][]string{
	"question":    {"title", "answers", "patterns", "pairs", "distractors", "groups", "sources", "scripts"},
	"title":       contentTags,
	"answer":      contentTags,
	"left":        contentTags,
	"right":       contentTags,
	"item":        contentTags,
	"groupTitle":  contentTags,
	"distractor":  contentTags,
	"answers":     {"answer"},
	"patterns":    {"pattern"},
	"pairs":       {"pair"},
	"pair":        {"left", "right"},
	"distractors": {"distractor"},
	"groups":      {"group"},
	"group":       {"groupTitle", "item"},
	"sources":     {"source"},
	"scripts":     {"script"},
}

type tagWarning struct {
	line int
	err  error
}

// unknownTags находит в вопросе теги, которых нет в схеме: при разборе они
// молча игнорируются, из-за чего опечатка (<answr>) выглядела бы как вопрос без
// данных. baseLine — строка начала вопроса в файле.
func unknownTags(chunk []byte, baseLine int) []tagWarning {
	var res []tagWarning
	d := xml.NewDecoder(bytes.NewReader(chunk))
	var stack []string
	for {
		tok, err := d.Token()
		if err != nil {
			return res // кусок уже проверен на синтаксис, здесь только EOF
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if !slices.Contains(schema[parent], name) {
					line, _ := d.InputPos()
					res = append(res, tagWarning{
						line: baseLine + line - 1,
						err:  fmt.Errorf("неизвестный тег <%s> внутри <%s> (игнорируется)", name, parent),
					})
					if err := d.Skip(); err != nil {
						return res
					}
					continue
				}
			}
			stack = append(stack, name)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
}

// validate проверяет, что разобранный вопрос полон для своего типа.
func validate(q *domain.Question) []error {
	var res []error
	if len(q.Title) == 0 {
		res = append(res, errors.New("пустой заголовок"))
	}
	switch q.Type {
	case domain.TypeSelect:
		if len(q.Answers) == 0 {
			res = append(res, errors.New("select без вариантов ответа"))
		} else if !slices.ContainsFunc(q.Answers, func(a domain.Answer) bool { return a.Correct }) {
			res = append(res, errors.New("select без правильного ответа"))
		}
	case domain.TypeInput:
		if len(q.Patterns) == 0 {
			res = append(res, errors.New("input без patterns"))
		}
	case domain.TypeMatch:
		if len(q.Pairs) == 0 {
			res = append(res, errors.New("match без пар"))
		}
	case domain.TypeClassify:
		if len(q.Groups) == 0 {
			res = append(res, errors.New("classify без групп"))
		}
	default:
		res = append(res, fmt.Errorf("неизвестный тип вопроса %q", q.Type))
	}
	return res
}
