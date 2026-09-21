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

func sendCard(ctx context.Context, b *bot.Bot, chatID int64, side Side, body string, kb *models.InlineKeyboardMarkup) {
	log.Printf("[tg] sendCard chat=%d image=%q bodyRunes=%d", chatID, side.Image, utf8.RuneCountInString(body))
	if side.Image != "" {
		// SendPhoto — sendPhoto. Photo: InputFileString принимает URL или file_id;
		// InputFileUpload — загрузка байтов локального файла (multipart).
		// Caption — подпись под фото.
		var photo models.InputFile = &models.InputFileString{Data: side.Image}
		if !isRemote(side.Image) {
			f, err := os.Open(side.Image)
			if err != nil {
				logErr("open image", err)
				send(ctx, b, chatID, body)
				return
			}
			defer f.Close()
			photo = &models.InputFileUpload{Filename: filepath.Base(side.Image), Data: f}
		}
		_, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:      chatID,
			Photo:       photo,
			Caption:     body,
			ReplyMarkup: kb,
		})
		logErr("SendPhoto", err)
		return
	}
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: body, ReplyMarkup: kb})
	logErr("SendMessage", err)
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
