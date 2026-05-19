package main

import (
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
