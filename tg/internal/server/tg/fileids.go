package tgbot

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// fileIDs помнит file_id, которые Telegram выдал нам за загруженные картинки.
// Повторная отправка/правка по file_id не заливает файл заново — это главный выигрыш
// в скорости листания. Ключ — путь к картинке: склеенные файлы адресуются по хэшу
// содержимого, поэтому новая версия картинки всегда получает новый путь.
// file_id привязан к боту, поэтому имя файла содержит отпечаток токена.
type fileIDs struct {
	mu   sync.RWMutex
	m    map[string]string
	path string // пусто — не сохранять на диск
}

func loadFileIDs(path string) *fileIDs {
	f := &fileIDs{m: map[string]string{}, path: path}
	if path == "" {
		return f
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[tg] file_ids: %v", err)
		}
		return f
	}
	if err := json.Unmarshal(data, &f.m); err != nil {
		log.Printf("[tg] file_ids: битый файл %s, начинаю заново: %v", path, err)
		f.m = map[string]string{}
	}
	log.Printf("[tg] file_id из кэша: %d", len(f.m))
	return f
}

func (f *fileIDs) get(img string) string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.m[img]
}

func (f *fileIDs) set(img, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m[img] == id {
		return
	}
	f.m[img] = id
	f.saveLocked()
}

func (f *fileIDs) drop(img string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, img)
	f.saveLocked()
}

func (f *fileIDs) saveLocked() {
	if f.path == "" {
		return
	}
	data, err := json.Marshal(f.m)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(f.path), 0o755)
	}
	if err == nil {
		tmp := f.path + ".tmp"
		if err = os.WriteFile(tmp, data, 0o644); err == nil {
			err = os.Rename(tmp, f.path)
		}
	}
	if err != nil {
		log.Printf("[tg] file_ids: не удалось сохранить: %v", err)
	}
}
