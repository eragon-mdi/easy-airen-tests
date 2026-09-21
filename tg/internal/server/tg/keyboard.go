package tgbot

import (
	"fmt"
	"strconv"

	"github.com/go-telegram/bot/models"
)

func keyboard(idx, total int, showAnswer, fromList bool) *models.InlineKeyboardMarkup {
	// Ряд навигации: ◀ 2/5 ▶. Кнопка = models.InlineKeyboardButton;
	// CallbackData — то, что придёт в CallbackQuery.Data при нажатии.
	prev := models.InlineKeyboardButton{Text: " ", CallbackData: cbNoop}
	if idx > 0 {
		prev = models.InlineKeyboardButton{Text: "◀", CallbackData: cbNav + strconv.Itoa(idx-1)}
	}
	next := models.InlineKeyboardButton{Text: " ", CallbackData: cbNoop}
	if idx < total-1 {
		next = models.InlineKeyboardButton{Text: "▶", CallbackData: cbNav + strconv.Itoa(idx+1)}
	}
	counter := models.InlineKeyboardButton{Text: fmt.Sprintf("%d/%d", idx+1, total), CallbackData: cbNoop}

	// Переключатель вопрос <-> ответ.
	toggle := models.InlineKeyboardButton{Text: "Показать ответ", CallbackData: cbAnswer + strconv.Itoa(idx)}
	if showAnswer {
		toggle = models.InlineKeyboardButton{Text: "Показать вопрос", CallbackData: cbNav + strconv.Itoa(idx)}
	}

	rows := [][]models.InlineKeyboardButton{{prev, counter, next}, {toggle}}
	if fromList {
		rows = append(rows, []models.InlineKeyboardButton{{Text: "↩ К списку формулировок", CallbackData: cbList}})
	}

	// Дошли до конца ленты — предлагаем уточнить запрос или начать новый.
	if idx == total-1 {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "✏️ Уточнить запрос", CallbackData: cbRefine},
			{Text: "🆕 Новый запрос", CallbackData: cbNew},
		})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
