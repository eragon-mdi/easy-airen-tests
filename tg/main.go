package main

import (
	"log"

	rootctx "github.com/eragon-mdi/go-playground/server/root-ctx"

	"tgbot/internal/adapter/tgfinder"
	configs "tgbot/internal/cfgs"
	parserxml "tgbot/internal/parser/xml"
	tgbot "tgbot/internal/server/tg"
	inmemmorystore "tgbot/internal/storage/in-memmory"
)

func main() {
	rCtx, rCtxCancel := rootctx.NotifyBackgroundCtxToShutdownSignal()
	defer rCtxCancel()

	cfgs := configs.MustLoad(configs.Default)

	bankQuestions, err := parserxml.ParseFile(cfgs.PathToQuestionsFileForParse)
	if err != nil {
		log.Fatal(err)
	}

	store, err := inmemmorystore.LoadIntoStore(bankQuestions, cfgs)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("вопросов в хранилище: %d из %d", store.Len(), bankQuestions.Meta.QuestionsTotal)

	finder, err := tgfinder.New(store, cfgs.PathPicturesDir, cfgs.PathRenderedDir, 0)
	if err != nil {
		log.Fatal(err)
	}

	tgBotStartF, err := tgbot.New(rCtx, cfgs, finder)
	if err != nil {
		log.Fatal(err)
	}

	tgBotStartF(rCtx)
}
