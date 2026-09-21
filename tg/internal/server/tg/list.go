package tgbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	groupsPerPage = 8  // формулировок на странице списка
	titleMaxRunes = 90 // длиннее — обрезаем в списке
)

// renderList показывает страницу списка формулировок: пользователь выбирает
// похожую на свой вопрос, и только после этого открывается лента картинок.
func (a *tgBot) renderList(ctx context.Context, b *bot.Bot, chatID int64, old *models.Message, page int) {
	a.mu.Lock()
	s := a.sessions[chatID]
	if s == nil || len(s.groups) == 0 {
		a.mu.Unlock()
		return
	}
	groups, total := s.groups, len(s.all)
	titles := make([]string, len(groups))
	for i, g := range groups {
		titles[i] = s.all[g[0]].Question
	}
	a.mu.Unlock()

	pages := (len(groups) + groupsPerPage - 1) / groupsPerPage
	page = min(max(page, 0), pages-1)
	from, to := page*groupsPerPage, min((page+1)*groupsPerPage, len(groups))

	var sb strings.Builder
	fmt.Fprintf(&sb, "🔎 Найдено вопросов: %d, разных формулировок: %d\n", total, len(groups))
	if len(groups) > groupsPerPage {
		fmt.Fprintf(&sb, "Страница %d из %d\n", page+1, pages)
	}
	sb.WriteString("Выберите формулировку, похожую на ваш вопрос:\n")

	var nums []models.InlineKeyboardButton
	for g := from; g < to; g++ {
		t := strings.Join(strings.Fields(titles[g]), " ")
		if t == "" {
			t = "(вопрос в виде картинки)"
		}
		if utf8.RuneCountInString(t) > titleMaxRunes {
			t = string([]rune(t)[:titleMaxRunes-1]) + "…"
		}
		fmt.Fprintf(&sb, "\n%d. %s", g+1, t)
		if n := len(groups[g]); n > 1 {
			fmt.Fprintf(&sb, " — %d шт.", n)
		}
		nums = append(nums, models.InlineKeyboardButton{Text: strconv.Itoa(g + 1), CallbackData: cbGroup + strconv.Itoa(g)})
	}

	var rows [][]models.InlineKeyboardButton
	for i := 0; i < len(nums); i += 4 {
		rows = append(rows, nums[i:min(i+4, len(nums))])
	}
	if pages > 1 {
		prev := models.InlineKeyboardButton{Text: " ", CallbackData: cbNoop}
		if page > 0 {
			prev = models.InlineKeyboardButton{Text: "◀", CallbackData: cbPage + strconv.Itoa(page-1)}
		}
		next := models.InlineKeyboardButton{Text: " ", CallbackData: cbNoop}
		if page < pages-1 {
			next = models.InlineKeyboardButton{Text: "▶", CallbackData: cbPage + strconv.Itoa(page+1)}
		}
		rows = append(rows, []models.InlineKeyboardButton{prev, {Text: fmt.Sprintf("%d/%d", page+1, pages), CallbackData: cbNoop}, next})
	}
	rows = append(rows,
		[]models.InlineKeyboardButton{{Text: fmt.Sprintf("📚 Листать все подряд (%d)", total), CallbackData: cbAll}},
		[]models.InlineKeyboardButton{{Text: "✏️ Уточнить запрос", CallbackData: cbRefine}, {Text: "🆕 Новый запрос", CallbackData: cbNew}},
	)
	kb := &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	text := truncate(sb.String(), 4096)

	if old != nil && len(old.Photo) == 0 { // список — текстовое сообщение, правим на месте
		_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: chatID, MessageID: old.ID, Text: text, ReplyMarkup: kb})
		logErr("EditMessageText", err)
		return
	}
	if old != nil { // была карточка-фото: тип сообщения не сменить — удаляем и шлём новое
		_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: old.ID})
		logErr("DeleteMessage", err)
	}
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: kb})
	logErr("SendMessage", err)
}
