package main

import "my-bilibili/internal/account"

// The embedded pane shares one Manager. Its dismissal never reaches this
// whole-app close path. A pending-close discard re-evaluates the old decision.
type clientAccountController struct{ core *clientCore }

func (a clientAccountController) Snapshot() account.State { return a.core.account.Snapshot() }
func (a clientAccountController) Restore() error {
	if a.core.closing {
		return account.Closed
	}
	return a.core.account.Restore()
}
func (a clientAccountController) Begin() error {
	if a.core.closing {
		return account.Closed
	}
	return a.core.account.Begin()
}
func (a clientAccountController) Logout() error {
	if a.core.closing {
		return account.Closed
	}
	return a.core.account.Logout()
}
func (a clientAccountController) Cancel() error      { return a.core.account.Cancel() }
func (a clientAccountController) RetrySave() error   { return a.core.account.RetrySave() }
func (a clientAccountController) RetryVerify() error { return a.core.account.RetryVerify() }
func (a clientAccountController) DiscardUnsaved() error {
	if err := a.core.account.DiscardUnsaved(); err != nil {
		return err
	}
	if a.core.closing {
		a.core.account.RequestClose(false)
	}
	return nil
}
func (a clientAccountController) RequestClose(discard bool) bool {
	a.core.RequestClose(false)
	if discard {
		state := a.core.account.Snapshot()
		if !state.Busy && (state.Unsaved || state.Phase == account.DeleteFailed) {
			a.core.account.RequestClose(true)
		}
	}
	return a.core.closed
}
