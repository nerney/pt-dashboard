package main

import (
	"context"
	"errors"
	"testing"

	"github.com/nerney/ptv/internal/config"
	"github.com/nerney/ptv/internal/handlers"
)

func TestEmbeddedTemplatesParse(t *testing.T) {
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handlers.NewRouter(store, nil, nil, assets)
}

func TestWaitStartupDefsPropagatesReadyError(t *testing.T) {
	want := errors.New("catalog unavailable")
	got := waitStartupDefs(func(context.Context) error {
		return want
	})
	if !errors.Is(got, want) {
		t.Fatalf("waitStartupDefs() = %v, want %v", got, want)
	}
}
