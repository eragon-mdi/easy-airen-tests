package tgbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (a *tgBot) onText(ctx context.Context, b *bot.Bot, u *models.Update) {
	// Апдейт может быть не сообщением (правка, реакция и т.п.) — пропускаем.
	if u.Message == nil || u.Message.Text == "" {
		return
	}
	chatID := u.Message.Chat.ID
	text := strings.TrimSpace(u.Message.Text)
	ch := a.chat(chatID)
	ch.mu.Lock()
	defer ch.mu.Unlock()
	log.Printf("[tg] text chat=%d raw=%q runes=%d", chatID, u.Message.Text, utf8.RuneCountInString(text))

	a.mu.Lock()
	s := a.sessions[chatID]
	if s == nil {
		s = &session{}
		a.sessions[chatID] = s
	}
	// Команды сбрасывают сессию в любой момент: следующий текст — новый запрос.
	if text == "/start" || text == "/new" || text == "/cancel" {
		s.refining, s.query, s.all, s.groups, s.cur = false, "", nil, nil, nil
		a.mu.Unlock()
		send(ctx, b, chatID, "Ок, начинаем заново. Введите новый запрос.")
		return
	}
	// Режим «уточнить»: новые слова дописываем к прежнему запросу.
	query := text
	if s.refining && s.query != "" {
		query = s.query + " " + text
	}
	log.Printf("[tg] chat=%d refining=%v prevQuery=%q -> query=%q", chatID, s.refining, s.query, query)
	s.refining = false

	if utf8.RuneCountInString(query) < a.cfg.MinQueryLen {
		a.mu.Unlock()
		// b.SendMessage — метод Bot API sendMessage. ChatID — кому, Text — что.
		send(ctx, b, chatID, fmt.Sprintf("Запрос слишком короткий: введите минимум %d символа(ов).", a.cfg.MinQueryLen))
		return
	}

	// Поиск может быть небыстрым — не держим мьютекс во время вызова.
	a.mu.Unlock()
	items, err := a.finder.Find(query)
	log.Printf("[tg] Find(%q) -> %d items, err=%v", query, len(items), err)
	for i, it := range items {
		log.Printf("[tg]   #%d Q=%q (img=%q) A=%q (img=%q)", i, it.Question.Text, it.Question.Image, it.Answer.Text, it.Answer.Image)
	}
	if err != nil {
		send(ctx, b, chatID, "Ошибка поиска, попробуйте позже.")
		return
	}
	if len(items) == 0 {
		send(ctx, b, chatID, "По вашему запросу ничего не найдено. Попробуйте другие слова.")
		return
	}

	groups := groupItems(items)
	log.Printf("[tg] chat=%d групп по формулировке: %d", chatID, len(groups))

	a.mu.Lock()
	s.query, s.all, s.groups = query, items, groups
	if len(groups) == 1 { // формулировка одна — сразу лента
		s.cur, s.fromList = allIndexes(len(items)), false
		a.mu.Unlock()
		a.render(ctx, b, chatID, nil, 0, false)
		return
	}
	s.fromList = true
	a.mu.Unlock()
	a.renderList(ctx, b, chatID, nil, 0)
}
