package tgbot

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func isRemote(img string) bool {
	return strings.HasPrefix(img, "http://") || strings.HasPrefix(img, "https://")
}

// imageUsable — можно ли показать картинку: URL считаем годным, локальный файл должен существовать.
func imageUsable(img string) bool {
	if img == "" {
		return false
	}
	if isRemote(img) {
		return true
	}
	if _, err := os.Stat(img); err != nil {
		log.Printf("[tg] картинка недоступна, показываю без неё: %v", err)
		return false
	}
	return true
}

// photoInput готовит фото к отправке: file_id, URL или загрузка локального файла.
// Второе значение — файл, который нужно закрыть после вызова API (или nil).
func (a *tgBot) photoInput(img, id string) (models.InputFile, *os.File, error) {
	switch {
	case id != "":
		return &models.InputFileString{Data: id}, nil, nil
	case isRemote(img):
		return &models.InputFileString{Data: img}, nil, nil
	}
	f, err := os.Open(img)
	if err != nil {
		return nil, nil, err
	}
	return &models.InputFileUpload{Filename: filepath.Base(img), Data: f}, f, nil
}

// remember сохраняет file_id самой крупной версии фото из ответа Telegram.
func (a *tgBot) remember(img string, m *models.Message) {
	if m != nil && len(m.Photo) > 0 {
		a.ids.set(img, m.Photo[len(m.Photo)-1].FileID)
	}
}

// sendCard отправляет карточку. Картинку по возможности берём по file_id (без загрузки).
func (a *tgBot) sendCard(ctx context.Context, b *bot.Bot, chatID int64, side Side, body string, kb *models.InlineKeyboardMarkup) {
	log.Printf("[tg] sendCard chat=%d image=%q bodyRunes=%d", chatID, side.Image, utf8.RuneCountInString(body))
	if side.Image == "" {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: body, ReplyMarkup: kb})
		logErr("SendMessage", err)
		return
	}

	id := a.ids.get(side.Image)
	for {
		photo, f, err := a.photoInput(side.Image, id)
		if err != nil {
			logErr("open image", err)
			send(ctx, b, chatID, body)
			return
		}
		m, err := b.SendPhoto(ctx, &bot.SendPhotoParams{ChatID: chatID, Photo: photo, Caption: body, ReplyMarkup: kb})
		if f != nil {
			f.Close()
		}
		if err == nil {
			a.remember(side.Image, m)
			return
		}
		if id != "" { // file_id мог устареть — забываем и пробуем с загрузкой
			log.Printf("[tg] file_id не подошёл (%v), загружаю файл заново", err)
			a.ids.drop(side.Image)
			id = ""
			continue
		}
		logErr("SendPhoto", err)
		return
	}
}

// editPhoto заменяет картинку и подпись в фото-сообщении; по file_id — без загрузки.
func (a *tgBot) editPhoto(ctx context.Context, b *bot.Bot, chatID int64, msgID int, img, body string, kb *models.InlineKeyboardMarkup) {
	id := a.ids.get(img)
	for {
		media := &models.InputMediaPhoto{Media: img, Caption: body}
		var f *os.File
		switch {
		case id != "":
			media.Media = id
		case !isRemote(img):
			var err error
			if f, err = os.Open(img); err != nil {
				logErr("open image", err)
				return
			}
			// Для локального файла Media = "attach://<имя>", а байты идут в MediaAttachment.
			media.Media, media.MediaAttachment = "attach://"+filepath.Base(img), f
		}
		m, err := b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{ChatID: chatID, MessageID: msgID, Media: media, ReplyMarkup: kb})
		if f != nil {
			f.Close()
		}
		if err == nil {
			a.remember(img, m)
			return
		}
		if id != "" {
			log.Printf("[tg] file_id не подошёл (%v), загружаю файл заново", err)
			a.ids.drop(img)
			id = ""
			continue
		}
		logErr("EditMessageMedia", err)
		return
	}
}

func send(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	log.Printf("[tg] send chat=%d text=%q", chatID, text)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
	logErr("SendMessage", err)
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func logErr(op string, err error) {
	// «message is not modified» — безобидно (нажали ту же кнопку дважды).
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		log.Printf("tg %s: %v", op, err)
	}
}
