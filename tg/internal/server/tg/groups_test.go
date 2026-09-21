package tgbot

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got := normalize("  Установить соответствие,  приведённых АЧХ схемам ? "); got != "установить соответствие приведенных ачх схемам" {
		t.Fatalf("got %q", got)
	}
}

func TestGroupItems(t *testing.T) {
	items := []Item{
		{Title: "Как  сбросить пароль?"},
		{Title: "Ток открытого диода"},
		{Title: "как сбросить ПАРОЛЬ"},
		{Title: "Ток закрытого диода"}, // отличается одним словом — другая группа
		{Title: ""}, // без формулировки — не группируется
		{Title: ""},
	}
	want := [][]int{{0, 2}, {1}, {3}, {4}, {5}}
	if got := groupItems(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
