package account

import domain "my-bilibili/internal/account"

// PaneController keeps the embedded view independent of a concrete Manager.
// A composition wrapper may reject mutations while still publishing safe state.
type PaneController interface {
	domain.Controller
	DiscardUnsaved() error
}

// NewPane creates the existing login view initially hidden. Construction and
// Show never start a restore or QR attempt. dismiss is optional and is called
// only for the pane's user-facing close action, after Hide has retired the view.
// Whole-application RequestClose and joining Manager.Done remain composition's
// responsibility; a pane dismissal never closes its reusable controller.
func NewPane(controller PaneController, dismiss func()) *View {
	return &View{controller: controller, dismiss: dismiss, discardUnsaved: controller.DiscardUnsaved, embedded: true}
}

// Show, Hide and Visible belong to the UI thread, as does Render. Composition
// invalidates its window after external Show/Hide calls, including when minimized.
func (v *View) Show()         { v.visible = true }
func (v *View) Visible() bool { return v.visible }

// Hide immediately drops QR/bitmap references and cancels only a displayed QR
// generation/wait/scan attempt. Save, verify and delete may finish in background.
// A race to one of those phases may reject Cancel; the pane still hides and the
// safe error is returned and retained for a later presentation. Hide is idempotent
// and never discards an unsaved attempt, deletes storage or requests app close.
func (v *View) Hide() error {
	if !v.visible {
		return nil
	}
	v.visible = false
	v.clearQR()
	v.confirmLogout = false
	v.actionError = domain.None
	s := v.controller.Snapshot()
	switch s.Phase {
	case domain.Generating, domain.Waiting, domain.Scanned:
		v.actionError = actionCode(v.controller.Cancel())
		if v.actionError != domain.None {
			return v.actionError
		}
	}
	return nil
}
