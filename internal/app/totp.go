package app

import (
	"context"
	"errors"
)

// ErrTotpRequired tells that the account asks for the code of the second
// factor and none was given.
var ErrTotpRequired = errors.New("zweiter Faktor erforderlich")

// ErrTotpRejected tells that the API did not accept the code of the second
// factor.
var ErrTotpRejected = errors.New("der Code des zweiten Faktors wurde nicht angenommen")

// SetTotp hands the code of the second factor from the interface over. The
// code is used for the next login only: it runs out after a few seconds.
func (a *App) SetTotp(code string) {
	if a.V3 == nil {
		return
	}
	a.V3.totpMu.Lock()
	a.V3.totpCode = code
	a.V3.totpMu.Unlock()
}

// takeTotp returns the code of the interface once.
func (v *V3) takeTotp() string {
	v.totpMu.Lock()
	defer v.totpMu.Unlock()
	code := v.totpCode
	v.totpCode = ""
	return code
}

// totpCode returns the code of the second factor: from the interface, the
// command line or the environment, otherwise it is asked through the prompt
// of the interface.
func (a *App) totpCode(ctx context.Context, token string) (string, error) {
	if a.V3 != nil {
		if code := a.V3.takeTotp(); code != "" {
			return code, nil
		}
	}
	if a.Config != nil && a.Config.Totp != "" {
		return a.Config.Totp, nil
	}
	if a.V3 != nil && a.V3.TotpPrompt != nil {
		return a.V3.TotpPrompt(ctx)
	}
	return "", nil
}
