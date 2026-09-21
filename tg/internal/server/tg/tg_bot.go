package tgbot

import (
	"context"
	"net/http"
	"sync"
	configs "tgbot/internal/cfgs"
	"time"

	"github.com/go-telegram/bot"
)

type tgBot struct {
	mu       sync.Mutex
	sessions map[int64]*session // ключ — chat ID
	finder   Finder
	cfg      *configs.Configs
}

type Finder interface {
	Find(q string) ([]Item, error)
}

// session — состояние диалога с одним чатом. Хранится в памяти:
// callback_data слишком мала, чтобы таскать в ней запрос и результаты.
type session struct {
	query    string
	all      []Item // все найденные варианты
	refining bool   // true — следующий текст дописываем к query
}

// Side — одна «сторона» карточки: текст и (возможно) картинка.
type Side struct {
	Text  string
	Image string // URL или file_id Telegram; пусто — картинки нет
}

// Item — один вариант выдачи: вопрос и ответ.
type Item struct {
	Question Side
	Answer   Side
}

func New(ctx context.Context, cfg *configs.Configs, finder Finder) (func(context.Context), error) {
	app := &tgBot{
		sessions: map[int64]*session{},
		cfg:      cfg,
		finder:   finder,
	}

	opts := []bot.Option{
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
