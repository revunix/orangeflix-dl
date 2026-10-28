package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/app"
)

// totpPrompt asks for the code of the second factor on the console.
func totpPrompt(out io.Writer, in io.Reader) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		fmt.Fprint(out, "Zweiter Faktor (TOTP-Code): ")
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("der TOTP-Code konnte nicht gelesen werden: %w", err)
		}
		return strings.TrimSpace(line), nil
	}
}

// warnExpiry warns about a subscription that runs out soon. It stays silent
// when the profile cannot be fetched or the subscription is fine.
func warnExpiry(ctx context.Context, application *app.App, logs *logRouter) {
	if application.V3 == nil {
		return
	}
	summary, err := application.ProfileSummary(ctx)
	if err != nil {
		return
	}
	if line := summary.ExpiryWarning(); line != "" {
		logs.Logf("%s", line)
	}
}
