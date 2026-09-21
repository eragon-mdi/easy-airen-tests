package tgbot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	configs "tgbot/internal/cfgs"
	"time"

	"github.com/go-telegram/bot"
)

type tgBot struct {
	mu       sync.Mutex
	sessions map[int64]*session   // ключ — chat ID
	chats    map[int64]*chatState // ключ — chat ID
	finder   Finder
	cfg      *configs.Configs
	ids      *fileIDs // file_id уже загруженных картинок
}

// chatState сериализует обработку апдейтов одного чата: воркеров несколько (разные
// чаты идут параллельно), а два нажатия одного пользователя не должны править
// карточку одновременно.
type chatState struct {
	mu     sync.Mutex
	navGen atomic.Uint64 // номер последнего нажатия «листать/ответ»; см. onCallback
}

func (a *tgBot) chat(id int64) *chatState {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.chats[id]
	if c == nil {
		c = &chatState{}
		a.chats[id] = c
	}
	return c
}

type Finder interface {
	Find(q string) ([]Item, error)
}

// session — состояние диалога с одним чатом. Хранится в памяти:
// callback_data слишком мала, чтобы таскать в ней запрос и результаты.
type session struct {
	query    string
	all      []Item  // все найденные варианты
	groups   [][]int // группы по одинаковой формулировке: индексы в all
	cur      []int   // что сейчас листает пользователь: индексы в all (группа или всё)
	fromList bool    // true — в ленту попали из списка формулировок (есть куда вернуться)
	refining bool    // true — следующий текст дописываем к query
}

// Item — один вариант выдачи: карточка целиком, ответ виден сразу.
type Item struct {
	Question string // формулировка вопроса; по ней же группируем одинаковые
	Answer   string // ответ текстом; пусто, если ответ только на картинке
	Code     string // скрипт параметрического вопроса — показываем моноширинным блоком
	Image    string // одна склеенная картинка: условие сверху, ответ снизу; пусто — без картинки
}

func New(ctx context.Context, cfg *configs.Configs, finder Finder) (func(context.Context), error) {
	app := &tgBot{
		sessions: map[int64]*session{},
		chats:    map[int64]*chatState{},
		cfg:      cfg,
		finder:   finder,
	}

	sum := sha1.Sum([]byte(cfg.TGBotToken))
	app.ids = loadFileIDs(filepath.Join(cfg.PathRenderedDir, "file_ids_"+hex.EncodeToString(sum[:])[:10]+".json"))

	opts := []bot.Option{
		// По умолчанию воркер один: все апдейты идут строго по очереди, и медленная
		// правка одной карточки тормозит всех. Чаты обрабатываются параллельно.
		bot.WithWorkers(8),
		bot.WithDefaultHandler(app.onText),
		bot.WithCallbackQueryDataHandler("", bot.MatchTypePrefix, app.onCallback),
		// Таймаут проверки токена (getMe) при создании бота.
		bot.WithCheckInitTimeout(cfg.CheckInitTimeout),
		// Таймаут long polling; HTTP-клиент должен жить дольше, иначе
		// он оборвёт getUpdates раньше, чем ответит Telegram.
		bot.WithHTTPClient(cfg.PollTimeout, &http.Client{Timeout: cfg.PollTimeout + 5*time.Second}),
	}
	// Свой адрес Bot API (прокси / локальный сервер) — только если задан.
	if cfg.ServerURL != "" {
		opts = append(opts, bot.WithServerURL(cfg.ServerURL))
	}
	if cfg.Debug {
		opts = append(opts, bot.WithDebug())
	}

	b, err := bot.New(cfg.TGBotToken, opts...)
	if err != nil {
		return nil, err
	}

	return b.Start, nil
}
