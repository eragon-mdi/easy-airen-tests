package parserxml

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"tgbot/internal/domain"
)

// Пример собран из docs/parser/parsing.md и покрывает все четыре типа вопросов.
const sample = `<?xml version="1.0" encoding="UTF-8"?>
<result>
  <meta>
    <questionsTotal>6</questionsTotal>
    <conflictGroupsTotal>1</conflictGroupsTotal>
    <file name="a.it2" questions="6" duplicatesInFile="0" alreadyInPrevious="0"/>
  </meta>
  <category name="Диоды" level="1">
    <question id="1" type="select" enabled="true" weight="1" multiple="false">
      <title>
        <text>На рисунке показана структура  диода. </text>
        <br/>
        <text>В каком направлении ток ?</text>
        <br/>
        <img src="images/911a11b181bd.png"/>
      </title>
      <answers>
        <answer correct="true"><text>Верно &amp; точно</text></answer>
        <answer correct="false"><img src="images/x.png"/></answer>
      </answers>
      <sources><source>Экзамен АВТИ 2016</source></sources>
    </question>
    <category name="1" level="2">
      <question id="2" type="input" enabled="false" weight="1" conflictGroup="3">
        <title><text>y = $(y)</text></title>
        <patterns>
          <pattern value="*$(y)*" quality="1.0000" wildcard="true" caseSensitive="false"/>
          <pattern value="0" quality="0.9000" wildcard="false" caseSensitive="true"/>
        </patterns>
        <scripts><script>x1 := RandomFloat;
  if x1 then goto 1;</script></scripts>
      </question>
    </category>
    <question id="3" type="match" enabled="true" weight="1">
      <title><text></text></title>
      <pairs>
        <pair><left><text>A</text></left><right><text>1</text></right></pair>
      </pairs>
      <distractors><distractor><text>лишнее</text></distractor></distractors>
    </question>
  </category>
  <category name="Схемы" level="1">
    <question id="4" type="classify" enabled="true" weight="1">
      <title><text>Разложите</text></title>
      <groups>
        <group>
          <groupTitle><text>Усилитель</text></groupTitle>
          <item><img src="images/a.png"/></item>
          <item><text>ОУ</text></item>
        </group>
      </groups>
    </question>
  </category>
</result>`

func TestParse(t *testing.T) {
	bank, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}

	if len(bank.Errors) != 0 {
		t.Fatalf("неожиданные проблемы: %v", bank.Errors)
	}

	if bank.Meta.QuestionsTotal != 6 || len(bank.Meta.Files) != 1 || bank.Meta.Files[0].Name != "a.it2" {
		t.Errorf("meta: %+v", bank.Meta)
	}

	// Порядок файла: вложенная категория идёт между вопросами родителя.
	var ids []int
	for _, q := range bank.Questions {
		ids = append(ids, q.ID)
	}
	if !reflect.DeepEqual(ids, []int{1, 2, 3, 4}) {
		t.Fatalf("порядок вопросов: %v", ids)
	}

	// select: порядок и пробелы контента, экранирование, категории.
	q := bank.Questions[0]
	wantTitle := domain.Content{
		{Kind: domain.KindText, Value: "На рисунке показана структура  диода. "},
		{Kind: domain.KindBR},
		{Kind: domain.KindText, Value: "В каком направлении ток ?"},
		{Kind: domain.KindBR},
		{Kind: domain.KindImg, Src: "images/911a11b181bd.png"},
	}
	if !reflect.DeepEqual(q.Title, wantTitle) {
		t.Errorf("title: %+v", q.Title)
	}
	if q.Type != domain.TypeSelect || !q.Enabled || q.Weight != 1 || q.Multiple || q.ConflictGroup != nil {
		t.Errorf("атрибуты: %+v", q)
	}
	if len(q.Answers) != 2 || !q.Answers[0].Correct || q.Answers[1].Correct ||
		q.Answers[0].Content[0].Value != "Верно & точно" || q.Answers[1].Content[0].Src != "images/x.png" {
		t.Errorf("answers: %+v", q.Answers)
	}
	if !reflect.DeepEqual(q.Category, []string{"Диоды"}) || !reflect.DeepEqual(q.Sources, []string{"Экзамен АВТИ 2016"}) {
		t.Errorf("category/sources: %v %v", q.Category, q.Sources)
	}

	// input: подкатегория, conflictGroup, enabled=false, patterns, скрипт.
	q = bank.Questions[1]
	if !reflect.DeepEqual(q.Category, []string{"Диоды", "1"}) {
		t.Errorf("category: %v", q.Category)
	}
	if q.Enabled || q.ConflictGroup == nil || *q.ConflictGroup != 3 {
		t.Errorf("атрибуты: %+v", q)
	}
	if len(q.Patterns) != 2 || q.Patterns[0] != (domain.Pattern{Value: "*$(y)*", Quality: 1, Wildcard: true}) ||
		q.Patterns[1] != (domain.Pattern{Value: "0", Quality: 0.9, CaseSensitive: true}) {
		t.Errorf("patterns: %+v", q.Patterns)
	}
	if len(q.Scripts) != 1 || !strings.Contains(q.Scripts[0], "\n  if x1") {
		t.Errorf("scripts: %q", q.Scripts)
	}

	// match: пустой <text>, пары, distractors; после подкатегории путь вернулся.
	q = bank.Questions[2]
	if !reflect.DeepEqual(q.Category, []string{"Диоды"}) {
		t.Errorf("category после подкатегории: %v", q.Category)
	}
	if len(q.Title) != 1 || q.Title[0].Value != "" {
		t.Errorf("пустой text: %+v", q.Title)
	}
	if len(q.Pairs) != 1 || q.Pairs[0].Left[0].Value != "A" || q.Pairs[0].Right[0].Value != "1" ||
		len(q.Distractors) != 1 || q.Distractors[0][0].Value != "лишнее" {
		t.Errorf("pairs/distractors: %+v %+v", q.Pairs, q.Distractors)
	}

	// classify.
	q = bank.Questions[3]
	if !reflect.DeepEqual(q.Category, []string{"Схемы"}) || len(q.Groups) != 1 ||
		q.Groups[0].Title[0].Value != "Усилитель" || len(q.Groups[0].Items) != 2 ||
		q.Groups[0].Items[0][0].Src != "images/a.png" || q.Groups[0].Items[1][0].Value != "ОУ" {
		t.Errorf("groups: %+v", q.Groups)
	}
}

// lineOf возвращает номер строки (с 1) первого вхождения sub в s.
func lineOf(s, sub string) int {
	return 1 + strings.Count(s[:strings.Index(s, sub)], "\n")
}

func TestBadAttributeSkipsOnlyThatQuestion(t *testing.T) {
	src := strings.Replace(sample, `weight="1" multiple="false"`, `weight="abc" multiple="false"`, 1)

	bank, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("не критичная ошибка не должна возвращаться: %v", err)
	}
	if len(bank.Questions) != 3 || bank.Questions[0].ID != 2 {
		t.Fatalf("должны остаться вопросы 2,3,4: %+v", bank.Questions)
	}
	if len(bank.Errors) != 1 {
		t.Fatalf("errors: %v", bank.Errors)
	}
	e := bank.Errors[0]
	if e.Severity != domain.Error || e.QuestionID != 1 || e.Line != lineOf(src, `<question id="1"`) ||
		!reflect.DeepEqual(e.Category, []string{"Диоды"}) {
		t.Errorf("описание проблемы: %+v", e)
	}
}

func TestUnknownTagWarns(t *testing.T) {
	src := strings.ReplaceAll(sample, "answers>", "answr>")

	bank, err := Parse(strings.NewReader(src))
	if err != nil || len(bank.Questions) != 4 {
		t.Fatalf("вопрос должен сохраниться: %v %d", err, len(bank.Questions))
	}
	var unknown, empty bool
	for _, e := range bank.Errors {
		if e.Severity != domain.Warning || e.QuestionID != 1 {
			t.Errorf("ожидалось предупреждение по вопросу 1: %v", e)
		}
		unknown = unknown || strings.Contains(e.Error(), "<answr>")
		empty = empty || strings.Contains(e.Error(), "без вариантов")
	}
	if !unknown || !empty {
		t.Errorf("errors: %v", bank.Errors)
	}
	if bank.Errors[0].Line != lineOf(src, "<answr>") {
		t.Errorf("строка: %d, ожидалась %d", bank.Errors[0].Line, lineOf(src, "<answr>"))
	}
}

func TestNoCorrectAnswerWarns(t *testing.T) {
	src := strings.Replace(sample, `correct="true"`, `correct="false"`, 1)

	bank, err := Parse(strings.NewReader(src))
	if err != nil || len(bank.Errors) != 1 || !strings.Contains(bank.Errors[0].Error(), "без правильного") {
		t.Fatalf("%v %v", err, bank.Errors)
	}
}

func TestSyntaxErrorReturnsPartial(t *testing.T) {
	// Опечатка в закрывающем теге четвёртого вопроса ломает синтаксис.
	src := strings.Replace(sample, "<text>Разложите</text>", "<text>Разложите</tex>", 1)

	bank, err := Parse(strings.NewReader(src))
	var pe domain.ParseError
	if !errors.As(err, &pe) || pe.Severity != domain.Fatal {
		t.Fatalf("ожидалась критическая ошибка: %v", err)
	}
	if bank == nil || len(bank.Questions) != 3 {
		t.Fatalf("должны сохраниться вопросы 1..3: %+v", bank)
	}
	if pe.Line != lineOf(src, "Разложите") {
		t.Errorf("строка ошибки %d, ожидалась %d", pe.Line, lineOf(src, "Разложите"))
	}
	if last := bank.Errors[len(bank.Errors)-1]; last.Severity != domain.Fatal {
		t.Errorf("Fatal должен быть в Bank.Errors: %v", bank.Errors)
	}
}

func TestTruncatedFile(t *testing.T) {
	bank, err := Parse(strings.NewReader(strings.TrimSuffix(sample, "</result>")))
	if err == nil || bank == nil || len(bank.Questions) != 4 {
		t.Fatalf("обрезанный файл: err=%v, вопросов=%d", err, len(bank.Questions))
	}
}
