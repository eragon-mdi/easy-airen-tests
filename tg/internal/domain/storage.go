package domain

// QuestionStore — что сервисам нужно от хранилища вопросов.
// Как хранится и как ищется — детали реализации.
type QuestionStore interface {
	// ByID возвращает вопрос по ID.
	ByID(id int) (Question, bool)
	// All возвращает все вопросы в порядке появления в файле (для прогрева кэшей).
	All() []Question
	// Search возвращает до limit самых похожих на запрос вопросов,
	// самые релевантные первыми. limit <= 0 — без ограничения.
	Search(query string, limit int) []Question
}
