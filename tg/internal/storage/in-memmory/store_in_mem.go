// Package inmemmorystore — хранилище вопросов в памяти вместе с поиском по ним.
// Поиск спрятан за хранилищем: снаружи видно только domain.QuestionStore.
package inmemmorystore

import (
	"errors"
	"fmt"

	"tgbot/internal/domain"
	"tgbot/internal/searcher"
	"tgbot/pkg/search"
)

// Store неизменяем после создания, поэтому безопасен для конкурентного чтения.
type Store struct {
	questions []domain.Question // в порядке появления в файле
	byID      map[int]int       // ID вопроса -> позиция в questions
	search    search.Searcher
}

var _ domain.QuestionStore = (*Store)(nil)

// LoadIntoStore кладёт разобранные вопросы в хранилище и строит по ним поисковый индекс.
func LoadIntoStore(bank *domain.Bank, searchCfg searcher.Config) (*Store, error) {
	if bank == nil {
		return nil, errors.New("inmemmorystore: пустой банк вопросов")
	}

	s := &Store{
		questions: bank.Questions,
		byID:      make(map[int]int, len(bank.Questions)),
	}
	for i := range s.questions {
		id := s.questions[i].ID
		// ID служит ключом и в поиске, поэтому повторы недопустимы.
		if _, dup := s.byID[id]; dup {
			return nil, fmt.Errorf("inmemmorystore: повторяющийся ID вопроса %d", id)
		}
		s.byID[id] = i
	}

	idx, err := searcher.New(s.questions, searchCfg)
	if err != nil {
		return nil, err
	}
	s.search = idx
	return s, nil
}

// Len — количество вопросов в хранилище.
func (s *Store) Len() int { return len(s.questions) }

// All реализует domain.QuestionStore. Возвращает копию среза (вопросы менять нельзя).
func (s *Store) All() []domain.Question {
	return append([]domain.Question(nil), s.questions...)
}

// ByID реализует domain.QuestionStore.
func (s *Store) ByID(id int) (domain.Question, bool) {
	i, ok := s.byID[id]
	if !ok {
		return domain.Question{}, false
	}
	return s.questions[i], true
}

// Search реализует domain.QuestionStore: движок отдаёт ID по убыванию релевантности,
// хранилище превращает их обратно в вопросы.
func (s *Store) Search(query string, limit int) []domain.Question {
	found := s.search.Search(query, limit)

	out := make([]domain.Question, 0, len(found))
	for _, r := range found {
		out = append(out, s.questions[s.byID[r.ID]])
	}
	return out
}
