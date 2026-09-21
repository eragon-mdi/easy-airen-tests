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
)

func (a *tgBot) onCallback(ctx context.Context, b *bot.Bot, u *models.Update) {
	cq := u.CallbackQuery
	if cq == nil {
		return
	}
	// AnswerCallbackQuery ОБЯЗАТЕЛЕН: без него на кнопке у пользователя
	// бесконечно крутятся «часики». Тут отвечаем без текста (тихо).
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	// cq.Message — MaybeInaccessibleMessage: если сообщению > 48 ч, тело недоступно.
	msg := cq.Message.Message
	if msg == nil {
		return
	}
	chatID := msg.Chat.ID
	data := cq.Data
	log.Printf("[tg] callback chat=%d data=%q msgID=%d", chatID, data, msg.ID)

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
		s.refining, s.query, s.all = false, "", nil
		a.mu.Unlock()
		send(ctx, b, chatID, "Ок, введите новый запрос.")

	default:
		a.mu.Unlock()
	}
}
