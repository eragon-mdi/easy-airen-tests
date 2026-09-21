package configs

import (
	"errors"
	"io/fs"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

var Default = ".env"

type Configs struct {
	TGBotToken       string        `env:"TELEGRAM_BOT_TOKEN" env-required:"true"`
	ServerURL        string        `env:"TELEGRAM_SERVER_URL"`
	CheckInitTimeout time.Duration `env:"TELEGRAM_CHECK_INIT_TIMEOUT" env-default:"30s"`
	PollTimeout      time.Duration `env:"TELEGRAM_POLL_TIMEOUT" env-default:"1m"`
	Debug            bool          `env:"TELEGRAM_DEBUG" env-default:"false"`

	// минимальная длина запроса в символах (рунах)
	MinQueryLen int `env:"TELEGRAM_RESPONSE_MIN_QUERY_LEN" env-default:"3"`
	// сколько вариантов даём за одну «порцию»
	PageSize int `env:"TELEGRAM_REQUEST_MAX_PAGE_SIZE" env-default:"5"`

	PathToQuestionsFileForParse string `env:"PATH_QUESTIONS_FILE"`
	PathPicturesDir             string `env:"PATH_PICTURES_DIR"`
	// Куда складывать заранее склеенные картинки. Уже готовые файлы не пересоздаются.
	PathRenderedDir string `env:"PATH_RENDERED_DIR" env-default:"cache/rendered"`

	// NGramSize — длина n-граммы, на которые режутся слова для сравнения.
	// Меньше (2) — терпимее к опечаткам, но больше шума; больше (4) — строже.
	SearchenerNGramSize int `env:"SEARCH_NGRAM_SIZE" env-default:"3"`
	// MinScore — порог схожести слова запроса со словом из вопросов (0..1+).
	// Выше — меньше ложных совпадений, но и опечатки прощаются хуже.
	SearchenerMinScore float64 `env:"SEARCH_MIN_SCORE" env-default:"0.4"`
	// PrefixBonus — добавка к оценке, если слово запроса — начало слова из вопроса
	// («полупр» → «полупроводнике»). Также действует на точное совпадение.
	SearchenerPrefixBonus float64 `env:"SEARCH_PREFIX_BONUS" env-default:"1"`
	// SubstringBonus — добавка, если слово запроса — часть слова, но не начало
	// («водник» → «полупроводнике»).
	SearchenerSubstringBonus float64 `env:"SEARCH_SUBSTRING_BONUS" env-default:"0.5"`
	// FuzzyMinLen — слова запроса короче (в символах) ищутся только точно:
	// нечёткий поиск по 1–2 буквам («n») даёт один мусор.
	SearchenerFuzzyMinLen int `env:"SEARCH_FUZZY_MIN_LEN" env-default:"3"`
	// IndexAnswers — искать ещё и по тексту вариантов ответа (только select).
	// Выключено: поиск идёт только по формулировке вопроса.
	SearchenerIndexAnswers bool `env:"SEARCH_INDEX_ANSWERS" env-default:"false"`
}

func MustLoad(path string) *Configs {
	cfg := &Configs{}
	if err := Load(path, cfg); err != nil {
		log.Fatal(err)
	}
	return cfg
}

func Load(path string, cfg any) error {
	// godotenv.Load не перезаписывает уже заданные переменные окружения,
	// так что реальное окружение приоритетнее файла.
	// Отсутствие файла — не ошибка: настройки могут прийти только из окружения.
	if err := godotenv.Load(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return cleanenv.ReadEnv(cfg)
}

func (c Configs) NgramSize() int          { return c.SearchenerNGramSize }
func (c Configs) MinScore() float64       { return c.SearchenerMinScore }
func (c Configs) PrefixBonus() float64    { return c.SearchenerPrefixBonus }
func (c Configs) SubstringBonus() float64 { return c.SearchenerSubstringBonus }
func (c Configs) FuzzyMinLen() int        { return c.SearchenerFuzzyMinLen }
func (c Configs) IndexAnswers() bool      { return c.SearchenerIndexAnswers }
