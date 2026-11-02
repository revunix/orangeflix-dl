package app

import (
	"testing"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

func TestReplayChatOffset(t *testing.T) {
	begin := int64(1_700_000_000)
	cases := []struct {
		name    string
		message orangeflix.V3ChatMessage
		want    float64
	}{
		{name: "video offset", message: orangeflix.V3ChatMessage{VideoTime: 12.5, Time: begin + 12}, want: 12.5},
		{name: "unix wall clock", message: orangeflix.V3ChatMessage{Time: begin + 40}, want: 40},
		{name: "unix milliseconds", message: orangeflix.V3ChatMessage{Time: (begin + 15) * 1000}, want: 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := replayChatOffset(tc.message, begin, 3600); got != tc.want {
				t.Fatalf("offset = %v, want %v", got, tc.want)
			}
		})
	}
}
