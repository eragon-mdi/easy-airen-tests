// Package tgfinder — адаптер между domain.QuestionStore и tgbot.Finder.
//
// Зависимости идут только в одну сторону, циклов нет:
//
//	domain  <-  storage  (реализует QuestionStore)
//	domain  <-  tgfinder ->  server/tg  (tgfinder знает оба конца, они друг о друге — нет)
//
// Интерфейс Finder объявлен на стороне потребителя (tgbot), поэтому tgbot
// не импортирует ни domain, ни storage; сборка происходит в main.
//
// Что попадает в карточку (условие показываем сразу, ответ — по кнопке):
//
//	select   вопрос: заголовок (+ картинки)          ответ: только правильные варианты
//	input    вопрос: заголовок (+ картинки)          ответ: паттерны с quality=1
//	match    вопрос: заголовок + левые элементы 1..N ответ: пары «N: левый → правый»
//	classify вопрос: заголовок + названия групп      ответ: «группа: элементы»
//
// Неправильные варианты select и distractors в match не показываются.
// Несколько картинок склеиваются в одну (см. imgcompose): в Telegram у сообщения одно фото.
package tgfinder

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tgbot/internal/domain"
	"tgbot/internal/imgcompose"
	tgbot "tgbot/internal/server/tg"
)

type Finder struct {
	store       domain.QuestionStore
	picturesDir string // Src картинок в вопросах — относительно result.xml
	limit       int    // 0 — без ограничения; постраничность делает бот
	cacheDir    string // сюда пишутся склеенные картинки

	items    map[int]tgbot.Item // готовые карточки по ID вопроса (заполняется в New)
	rendered atomic.Int64       // статистика прогрева: склеено заново
	reused   atomic.Int64       // ...взято из кэша на диске
}

var _ tgbot.Finder = (*Finder)(nil)

// New создаёт адаптер и СРАЗУ готовит карточки всех вопросов: склеивает картинки в cacheDir
// (готовые файлы не пересоздаются) и держит карточки в памяти. Поэтому поиск потом
// ничего не рендерит. Блокирует до конца прогрева.
func New(store domain.QuestionStore, picturesDir, cacheDir string, limit int) (*Finder, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("tgfinder: каталог кэша картинок: %w", err)
	}
	f := &Finder{store: store, picturesDir: picturesDir, limit: limit, cacheDir: cacheDir}
	f.prerender()
	return f, nil
}

// prerender строит карточки всех вопросов параллельно (по числу ядер).
func (f *Finder) prerender() {
	all := f.store.All()
	start := time.Now()
	f.items = make(map[int]tgbot.Item, len(all))

	var mu sync.Mutex
	var wg sync.WaitGroup
	jobs := make(chan domain.Question)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for q := range jobs {
				it := f.toItem(q)
				mu.Lock()
				f.items[q.ID] = it
				mu.Unlock()
			}
		}()
	}
	for i, q := range all {
		jobs <- q
		if (i+1)%100 == 0 {
			log.Printf("[tgfinder] подготовлено карточек: %d/%d", i+1, len(all))
		}
	}
	close(jobs)
	wg.Wait()
	log.Printf("[tgfinder] карточки готовы: %d, склеено новых картинок: %d, взято из кэша: %d, за %s",
		len(all), f.rendered.Load(), f.reused.Load(), time.Since(start).Round(time.Millisecond))
}

// Find реализует tgbot.Finder.
func (f *Finder) Find(q string) ([]tgbot.Item, error) {
	found := f.store.Search(q, f.limit)
	items := make([]tgbot.Item, 0, len(found))
	for _, qu := range found {
		it, ok := f.items[qu.ID] // после New карта только читается — блокировка не нужна
		if !ok {                 // не должно случаться; на всякий случай строим на лету
			it = f.toItem(qu)
		}
		items = append(items, it)
	}
	return items, nil
}

// block — содержимое Content: текст и пути к картинкам (уже с учётом picturesDir).
type block struct {
	text string
	imgs []string
}

func (f *Finder) content(c domain.Content) block {
	var b strings.Builder
	var imgs []string
	for _, bl := range c {
		switch bl.Kind {
		case domain.KindText:
			b.WriteString(bl.Value)
		case domain.KindBR:
			b.WriteByte('\n')
		case domain.KindImg:
			imgs = append(imgs, filepath.Join(f.picturesDir, bl.Src))
		}
	}
	return block{text: strings.TrimSpace(b.String()), imgs: imgs}
}

func (f *Finder) toItem(q domain.Question) tgbot.Item {
	title := f.content(q.Title)
	// Картинки условия не нумеруем: их нужно просто увидеть целиком.
	qRows := plain(title.imgs)
	qText := title.text
	var aText string
	var aRows []imgcompose.Row

	switch q.Type {
	case domain.TypeSelect:
		var texts, imgs []string
		for _, a := range q.Answers {
			if !a.Correct {
				continue
			}
			c := f.content(a.Content)
			if c.text != "" {
				texts = append(texts, c.text)
			}
			imgs = append(imgs, c.imgs...)
		}
		aText = bullets(texts)
		aRows = numbered(imgs) // несколько правильных картинок — с номерами

	case domain.TypeInput:
		var texts []string
		for _, p := range q.Patterns {
			if p.Quality == 1 {
				texts = append(texts, p.Value)
			}
		}
		if len(texts) == 0 { // на всякий случай: частичные ответы лучше пустоты
			for _, p := range q.Patterns {
				texts = append(texts, p.Value)
			}
		}
		aText = bullets(texts)

	case domain.TypeMatch:
		var qLines, aLines []string
		for i, p := range q.Pairs {
			n := i + 1
			l, r := f.content(p.Left), f.content(p.Right)
			if l.text != "" {
				qLines = append(qLines, fmt.Sprintf("%d. %s", n, l.text))
			}
			if len(l.imgs) > 0 { // левые элементы — часть условия, показываем сразу
				qRows = append(qRows, imgcompose.Row{Label: n, Left: l.imgs})
			}
			if l.text != "" || r.text != "" {
				aLines = append(aLines, fmt.Sprintf("%d. %s → %s", n, orPic(l), orPic(r)))
			}
			if len(r.imgs) > 0 { // строка «номер: левая → правая» — пара не перепутается
				aRows = append(aRows, imgcompose.Row{Label: n, Left: l.imgs, Right: r.imgs})
			}
		}
		qText = joinNonEmpty("\n\n", qText, strings.Join(qLines, "\n"))
		aText = strings.Join(aLines, "\n")

	case domain.TypeClassify:
		var qLines, aLines []string
		for i, g := range q.Groups {
			n := i + 1
			gt := f.content(g.Title)
			if gt.text != "" {
				qLines = append(qLines, fmt.Sprintf("%d. %s", n, gt.text))
			}
			if len(gt.imgs) > 0 {
				qRows = append(qRows, imgcompose.Row{Label: n, Left: gt.imgs})
			}
			var itemTexts []string
			var itemImgs []string
			for _, it := range g.Items {
				c := f.content(it)
				if c.text != "" {
					itemTexts = append(itemTexts, c.text)
				}
				itemImgs = append(itemImgs, c.imgs...)
			}
			if len(itemTexts) > 0 {
				aLines = append(aLines, fmt.Sprintf("%d. %s: %s", n, orPic(gt), strings.Join(itemTexts, ", ")))
			}
			if len(itemImgs) > 0 {
				aRows = append(aRows, imgcompose.Row{Label: n, Left: gt.imgs, Right: itemImgs})
			}
		}
		qText = joinNonEmpty("\n\n", qText, strings.Join(qLines, "\n"))
		aText = strings.Join(aLines, "\n")
	}

	return tgbot.Item{
		Question: tgbot.Side{Text: qText, Image: f.render(qRows)},
		Answer:   tgbot.Side{Text: aText, Image: f.render(aRows)},
	}
}

// render возвращает путь к картинке для карточки: исходный файл, если он один и без
// оформления, иначе — склеенный PNG из кэша. Пусто — картинки нет.
func (f *Finder) render(rows []imgcompose.Row) string {
	if len(rows) == 0 {
		return ""
	}
	if len(rows) == 1 && rows[0].Label == 0 && len(rows[0].Left) == 1 && len(rows[0].Right) == 0 {
		return rows[0].Left[0]
	}

	// Ключ: версия раскладки + строки + размер и время изменения исходных файлов,
	// чтобы подмена картинки или правка раскладки давали новый файл.
	h := sha1.New()
	fmt.Fprintf(h, "v%d %v", imgcompose.Version, rows)
	for _, r := range rows {
		for _, p := range append(append([]string(nil), r.Left...), r.Right...) {
			if st, err := os.Stat(p); err == nil {
				fmt.Fprintf(h, "|%d:%d", st.Size(), st.ModTime().UnixNano())
			}
		}
	}
	path := filepath.Join(f.cacheDir, hex.EncodeToString(h.Sum(nil))[:16]+".png")

	if _, err := os.Stat(path); err == nil {
		f.reused.Add(1)
		return path
	}

	img, skipped := imgcompose.Compose(rows)
	for _, err := range skipped {
		log.Printf("[tgfinder] картинка пропущена: %v", err)
	}
	if img == nil {
		return ""
	}
	// Временный файл с уникальным именем: два потока могут клеить одно и то же.
	out, err := os.CreateTemp(f.cacheDir, "*.tmp")
	if err != nil {
		log.Printf("[tgfinder] кэш: %v", err)
		return ""
	}
	tmp := out.Name()
	err = imgcompose.WritePNG(out, img)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
		log.Printf("[tgfinder] кэш: %v", err)
		return ""
	}
	f.rendered.Add(1)
	return path
}

// plain — каждая картинка отдельной строкой, без номеров.
func plain(imgs []string) []imgcompose.Row {
	rows := make([]imgcompose.Row, 0, len(imgs))
	for _, p := range imgs {
		rows = append(rows, imgcompose.Row{Left: []string{p}})
	}
	return rows
}

// numbered — то же, но с номерами 1..N (если картинка одна, номер не нужен).
func numbered(imgs []string) []imgcompose.Row {
	rows := plain(imgs)
	if len(rows) > 1 {
		for i := range rows {
			rows[i].Label = i + 1
		}
	}
	return rows
}

func bullets(lines []string) string {
	if len(lines) <= 1 {
		return strings.Join(lines, "")
	}
	return "• " + strings.Join(lines, "\n• ")
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// orPic — текст элемента или пометка «рис.», если элемент — только картинка.
func orPic(b block) string {
	if b.text != "" {
		return b.text
	}
	return "рис."
}
