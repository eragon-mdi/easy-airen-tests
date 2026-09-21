package tgbot

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// manyResults — с такого числа найденного советуем уточнить запрос.
const manyResults = 10

// render показывает карточку idx текущей ленты: вопрос и сразу ответ. Если old != nil —
// правим существующее сообщение (чтобы листание не засоряло чат), иначе шлём новое.
func (a *tgBot) render(ctx context.Context, b *bot.Bot, chatID int64, old *models.Message, idx int) {
	a.mu.Lock()
	s := a.sessions[chatID]
	if s == nil || idx < 0 || idx >= len(s.cur) {
		a.mu.Unlock()
		return
	}
	item := s.all[s.cur[idx]]
	total, fromList, found := len(s.cur), s.fromList, len(s.all)
	a.mu.Unlock()

	if !imageUsable(item.Image) {
		item.Image = ""
	}
	log.Printf("[tg] render chat=%d idx=%d total=%d edit=%v image=%q", chatID, idx, total, old != nil, item.Image)

	head := fmt.Sprintf("Вопрос %d из %d", idx+1, total)
	if idx == 0 && !fromList { // формулировка одна: сразу даём понять, сколько нашлось
		head = fmt.Sprintf("🔎 Найдено вопросов: %d\n", found) + head
		if total > manyResults {
			head += "\n⚠️ Результатов много — лучше уточнить запрос (кнопка в конце ленты)."
		}
	}
	// Подпись у фото ограничена 1024 символами, текст сообщения — 4096.
	limit := 4096
	if item.Image != "" {
		limit = 1024
	}
	body := cardHTML(head, item, limit)
	kb := keyboard(idx, total, fromList)

	hasPhoto := item.Image != ""
	oldHasPhoto := old != nil && len(old.Photo) > 0

	switch {
	case old == nil:
		a.sendCard(ctx, b, chatID, item.Image, body, kb)

	case hasPhoto && oldHasPhoto:
		a.editPhoto(ctx, b, chatID, old.ID, item.Image, body, kb)

	case !hasPhoto && !oldHasPhoto:
		// EditMessageText — поменять текст и клавиатуру у текстового сообщения.
		_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   old.ID,
			Text:        body,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: kb,
		})
		logErr("EditMessageText", err)

	default:
		// Telegram не позволяет превратить текстовое сообщение в фото и наоборот,
		// поэтому удаляем старое и шлём новое.
		_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: old.ID})
		logErr("DeleteMessage", err)
		a.sendCard(ctx, b, chatID, item.Image, body, kb)
	}
}

// cardHTML собирает текст карточки в HTML-разметке Telegram. Лимит считается по
// видимому тексту (без тегов), поэтому режем сырой текст до экранирования: сначала
// скрипт, потом формулировку — ответ важнее всего.
func cardHTML(head string, it Item, limit int) string {
	answer := it.Answer
	if answer == "" && it.Image != "" {
		answer = "ответ на картинке"
	}
	const codeTitle = "\n\n🧮 Скрипт (по нему считается ответ):\n"

	fixed := utf8.RuneCountInString(head) + utf8.RuneCountInString("\n\n❓ \n\n✅ ") + utf8.RuneCountInString(answer)
	question := it.Question
	if room := limit - fixed; utf8.RuneCountInString(question) > room {
		question = truncate(question, max(room, 20))
	}
	fixed += utf8.RuneCountInString(question)

	code := it.Code
	if code != "" {
		room := limit - fixed - utf8.RuneCountInString(codeTitle)
		if room < 80 { // места нет — лучше без скрипта, чем обрывок
			code = ""
		} else {
			code = truncate(strings.TrimSpace(code), room)
		}
	}

	var sb strings.Builder
	sb.WriteString(html.EscapeString(head))
	sb.WriteString("\n\n❓ " + html.EscapeString(question))
	sb.WriteString("\n\n✅ " + html.EscapeString(answer))
	if code != "" {
		sb.WriteString(html.EscapeString(codeTitle) + "<pre>" + html.EscapeString(code) + "</pre>")
	}
	return sb.String()
}
