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
		{Question: "Как  сбросить пароль?"},
		{Question: "Ток открытого диода"},
		{Question: "как сбросить ПАРОЛЬ"},
		{Question: "Ток закрытого диода"}, // отличается одним словом — другая группа
		{Question: ""}, // без формулировки — не группируется
		{Question: ""},
	}
	want := [][]int{{0, 2}, {1}, {3}, {4}, {5}}
	if got := groupItems(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
