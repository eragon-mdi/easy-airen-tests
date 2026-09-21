package tgbot

import (
	"context"
	"fmt"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// manyResults — с такого числа найденного советуем уточнить запрос.
const manyResults = 10

// render показывает вариант idx (вопрос или ответ). Если old != nil — правим
// существующее сообщение (чтобы листание не засоряло чат), иначе шлём новое.
func (a *tgBot) render(ctx context.Context, b *bot.Bot, chatID int64, old *models.Message, idx int, showAnswer bool) {
	a.mu.Lock()
	s := a.sessions[chatID]
	if s == nil || idx < 0 || idx >= len(s.cur) {
		a.mu.Unlock()
		return
	}
	item := s.all[s.cur[idx]]
	total, fromList, found := len(s.cur), s.fromList, len(s.all)
	a.mu.Unlock()

	side, title := item.Question, "❓ Вопрос"
	if showAnswer {
		side, title = item.Answer, "✅ Ответ"
	}
	if !imageUsable(side.Image) {
		side.Image = ""
	}
	log.Printf("[tg] render chat=%d idx=%d total=%d answer=%v edit=%v image=%q", chatID, idx, total, showAnswer, old != nil, side.Image)
	// Caption у фото ограничен 1024 символами, текст сообщения — 4096.
	limit := 4096
	if side.Image != "" {
		limit = 1024
	}
	head := fmt.Sprintf("%s %d из %d", title, idx+1, total)
	if idx == 0 && !showAnswer && !fromList { // формулировка одна: сразу даём понять, сколько нашлось
		head = fmt.Sprintf("🔎 Найдено вопросов: %d\n", found) + head
		if total > manyResults {
			head += "\n⚠️ Результатов много — лучше уточнить запрос (кнопка в конце ленты)."
		}
	}
	body := truncate(head+"\n\n"+side.Text, limit)
	kb := keyboard(idx, total, showAnswer, fromList)

	hasPhoto := side.Image != ""
	oldHasPhoto := old != nil && len(old.Photo) > 0

	switch {
	case old == nil:
		a.sendCard(ctx, b, chatID, side, body, kb)

	case hasPhoto && oldHasPhoto:
		a.editPhoto(ctx, b, chatID, old.ID, side.Image, body, kb)

	case !hasPhoto && !oldHasPhoto:
		// EditMessageText — поменять текст и клавиатуру у текстового сообщения.
		_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   old.ID,
			Text:        body,
			ReplyMarkup: kb,
		})
		logErr("EditMessageText", err)

	default:
		// Telegram не позволяет превратить текстовое сообщение в фото и наоборот,
		// поэтому удаляем старое и шлём новое.
		_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: old.ID})
		logErr("DeleteMessage", err)
		a.sendCard(ctx, b, chatID, side, body, kb)
	}
}
