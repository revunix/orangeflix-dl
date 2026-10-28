package config

import "testing"

// TestParseArgsReadsTheWatchFlag checks the flag for the player.
func TestParseArgsReadsTheWatchFlag(t *testing.T) {
	opts, err := ParseArgs([]string{"--format", "mediatheke", "--episode", "420", "--watch"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !opts.Watch {
		t.Error("watch = false, want true")
	}
}

// TestParseArgsWithoutWatch checks that the flag is off by default.
func TestParseArgsWithoutWatch(t *testing.T) {
	opts, err := ParseArgs([]string{"--format", "mediatheke"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if opts.Watch {
		t.Error("watch = true, want false")
	}
}
