package tgbot

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	// Префиксы callback_data. Лимит Telegram на callback_data — 64 байта,
	// поэтому храним только короткую команду + индекс, а сами данные — в сессии.
	cbNav    = "nav:"   // nav:<idx>  — показать вопрос с номером idx
	cbAnswer = "ans:"   // ans:<idx>  — показать ответ для idx
	cbRefine = "refine" // пользователь будет дописывать запрос
	cbNoop   = "noop"   // кнопка-счётчик «2/5», ничего не делает
	cbNew    = "new"    // выйти из режима уточнения и начать новый запрос
	cbGroup  = "grp:"   // grp:<g> — открыть группу формулировок g
	cbPage   = "pg:"    // pg:<p>  — страница списка формулировок
	cbAll    = "all"    // листать все найденные подряд, без групп
	cbList   = "list"   // вернуться к списку формулировок
)

func (a *tgBot) onCallback(ctx context.Context, b *bot.Bot, u *models.Update) {
	cq := u.CallbackQuery
	if cq == nil {
		return
	}
	// AnswerCallbackQuery ОБЯЗАТЕЛЕН: без него на кнопке у пользователя
	// бесконечно крутятся «часики». Тут отвечаем без текста (тихо).
	// Не ждём ответа: это лишний сетевой проход перед правкой карточки.
	go b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	// cq.Message — MaybeInaccessibleMessage: если сообщению > 48 ч, тело недоступно.
	msg := cq.Message.Message
	if msg == nil {
		return
	}
	chatID := msg.Chat.ID
	data := cq.Data
	log.Printf("[tg] callback chat=%d data=%q msgID=%d", chatID, data, msg.ID)

	// Быстро тапая ▶▶▶, пользователь копит очередь правок. Каждая правка задаёт
	// карточку целиком (индекс лежит в callback_data), поэтому промежуточные можно
	// пропустить: достаточно последней.
	ch := a.chat(chatID)
	isNav := strings.HasPrefix(data, cbNav) || strings.HasPrefix(data, cbAnswer)
	var gen uint64
	if isNav {
		gen = ch.navGen.Add(1)
	}
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if isNav && ch.navGen.Load() != gen {
		log.Printf("[tg] chat=%d пропущено устаревшее нажатие %q", chatID, data)
		return
	}

	a.mu.Lock()
	s := a.sessions[chatID]
	if s == nil || len(s.all) == 0 { // бот перезапускался — состояние потеряно
		a.mu.Unlock()
		send(ctx, b, chatID, "Сессия устарела, отправьте запрос заново.")
		return
	}

	switch {
	case data == cbNoop:
		a.mu.Unlock()

	case strings.HasPrefix(data, cbNav), strings.HasPrefix(data, cbAnswer):
		showAnswer := strings.HasPrefix(data, cbAnswer)
		idx, _ := strconv.Atoi(data[strings.Index(data, ":")+1:])
		a.mu.Unlock()
		a.render(ctx, b, chatID, msg, idx, showAnswer)

	case strings.HasPrefix(data, cbGroup):
		g, _ := strconv.Atoi(data[len(cbGroup):])
		if g < 0 || g >= len(s.groups) {
			a.mu.Unlock()
			return
		}
		s.cur = s.groups[g]
		a.mu.Unlock()
		a.render(ctx, b, chatID, msg, 0, false)

	case strings.HasPrefix(data, cbPage):
		p, _ := strconv.Atoi(data[len(cbPage):])
		a.mu.Unlock()
		a.renderList(ctx, b, chatID, msg, p)

	case data == cbAll:
		s.cur = allIndexes(len(s.all))
		a.mu.Unlock()
		a.render(ctx, b, chatID, msg, 0, false)

	case data == cbList:
		a.mu.Unlock()
		a.renderList(ctx, b, chatID, msg, 0)

	case data == cbRefine:
		s.refining = true
		q := s.query
		a.mu.Unlock()
		// Кнопка «Новый запрос» на этом же сообщении — мгновенный выход из уточнения.
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Продолжите ввод: напишите ещё слова, они добавятся к запросу «" + q + "».\nИли начните новый запрос (/new).",
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "🆕 Новый запрос", CallbackData: cbNew},
			}}},
		})
		logErr("SendMessage", err)

	case data == cbNew:
		s.refining, s.query, s.all, s.groups, s.cur = false, "", nil, nil, nil
		a.mu.Unlock()
		send(ctx, b, chatID, "Ок, введите новый запрос.")

	default:
		a.mu.Unlock()
	}
}
