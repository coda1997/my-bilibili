package transport

import (
	"context"
	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	"net/http"
	"testing"
)

type verifiedGateway struct{}

func (verifiedGateway) Begin(context.Context) (account.Attempt, error) { return nil, account.Invalid }
func (verifiedGateway) Verify(context.Context, account.Material) error { return nil }

type retainedStore struct{ deleted func() error }

func (retainedStore) Load() (account.Material, error) { return testMaterial(), nil }
func (retainedStore) Save(account.Material) error     { return account.Storage }
func (s retainedStore) Delete() error {
	if s.deleted != nil {
		return s.deleted()
	}
	return account.Storage
}

func TestManagerPublicationAndTransportRevocationUseOneLockOrder(t *testing.T) {
	entered := make(chan struct{})
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: &observedBody{Reader: &waitingReader{ctx: req.Context(), entered: entered}}}, nil
	})
	var manager *account.Manager
	var scope *Scope
	store := retainedStore{deleted: func() error {
		if scope.Context().Err() == nil || manager.Snapshot().SessionEpoch != 0 {
			t.Error("native Delete began before scope revoke or under account mutex")
		}
		return account.Storage
	}}
	manager = account.NewWithPublisher(verifiedGateway{}, store, c)
	if manager.Restore() != nil {
		t.Fatal("synthetic restore failed")
	}
	eventually(t, func() bool { state := manager.Snapshot(); return state.Phase == account.SignedIn && !state.Busy })
	scope = bind(t, c)
	if scope.State().AccountEpoch != manager.Snapshot().SessionEpoch || !scope.State().Authenticated {
		t.Fatal("SignedIn preceded transport activation")
	}
	done := make(chan error, 1)
	go func() { _, err := scope.JSON(context.Background(), Detail, detailParams()); done <- err }()
	await(t, entered)
	if manager.Logout() != nil {
		t.Fatal("synthetic logout failed")
	}
	if await(t, done) != catalog.Canceled {
		t.Fatal("logout did not cancel account-bound body")
	}
	eventually(t, func() bool { state := manager.Snapshot(); return state.Phase == account.DeleteFailed && !state.Busy })
	if c.State().Authenticated || !manager.Snapshot().Stored || manager.RequestClose(false) {
		t.Fatal("delete failure hid persistence or restored authentication")
	}
	if !manager.RequestClose(true) {
		t.Fatal("explicit failed-delete acknowledgement failed")
	}
	await(t, manager.Done())
}

func TestNavExpiryFeedbackPreservesRetainedAccount(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { return response(`{"code":-101}`), nil })
	manager := account.NewWithPublisher(verifiedGateway{}, retainedStore{}, c)
	_ = manager.Restore()
	eventually(t, func() bool { state := manager.Snapshot(); return state.Phase == account.SignedIn && !state.Busy })
	s := bind(t, c)
	if _, err := s.JSON(context.Background(), Nav, nil); err != catalog.SessionExpired {
		t.Fatal("authenticated nav did not report expiry")
	}
	if !manager.SessionExpired(c.State().ExpiredEpoch) || manager.SessionExpired(c.State().ExpiredEpoch) {
		t.Fatal("matching expiry feedback was not one-shot")
	}
	if state := manager.Snapshot(); state.Phase != account.Expired || !state.Stored || state.SessionEpoch != 0 {
		t.Fatal("expiry lost stored account or usable epoch")
	}
	manager.RequestClose(false)
	await(t, manager.Done())
}
