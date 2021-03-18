package loop_test

import (
	"testing"

	loop "github.com/cameronvoglerolive/venddysearch/venddy-search-searchbar/loop"
	ldk "github.com/open-olive/loop-development-kit/ldk/go"
)

func TestNewLoop(t *testing.T) {
	newLoop, err := loop.NewLoop(&ldk.Logger{})
	if err != nil {
		t.Errorf("Test Failed")
	}
	if newLoop == nil {
		t.Errorf("Test Failed")
	}
}
