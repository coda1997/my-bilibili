package transport

import (
	"context"
	"my-bilibili/internal/catalog"
)

type preparationNeed uint8

const (
	wbiPreparation preparationNeed = 1 << iota
	browserPreparation
)

type preparation struct {
	ctx      context.Context
	cancel   context.CancelFunc
	needs    preparationNeed
	done     chan struct{}
	finished bool
	err      error
}

// A caller owns only its wait. The one preparation worker and its deadline
// belong to the scope, so a canceled first caller cannot cancel later waiters.
func (s *Scope) prepare(ctx context.Context, needs preparationNeed) error {
	if needs == 0 {
		return nil
	}
	if err := s.check(ctx); err != nil {
		return err
	}
	now := s.client.now()
	s.client.mu.Lock()
	if err := s.live(); err != nil {
		s.client.mu.Unlock()
		return err
	}
	missing := needs
	if s.keys.usable(now) {
		missing &^= wbiPreparation
	}
	if s.browser.usable(now) {
		missing &^= browserPreparation
	}
	if missing == 0 {
		s.client.mu.Unlock()
		return nil
	}
	p := s.preparing
	if p != nil && !p.finished {
		p.needs |= missing
	} else {
		workCtx, cancel := context.WithTimeout(s.ctx, requestLimit)
		p = &preparation{ctx: workCtx, cancel: cancel, needs: missing, done: make(chan struct{})}
		s.preparing = p
		s.client.workers.Add(1)
		go s.runPreparation(p)
	}
	s.client.mu.Unlock()
	select {
	case <-ctx.Done():
		return contextCode(ctx.Err())
	case <-s.ctx.Done():
		return catalog.Canceled
	case <-p.done:
		if err := s.check(ctx); err != nil {
			return err
		}
		return p.err
	}
}

func (s *Scope) finishPreparation(p *preparation, err error) {
	p.err, p.finished = err, true
	close(p.done)
}
func (s *Scope) runPreparation(p *preparation) {
	defer s.client.workers.Done()
	defer p.cancel()
	var completed preparationNeed
	var keys keyset
	var browser browserState
	for {
		s.client.mu.Lock()
		err := s.live()
		if err == nil && p.ctx.Err() != nil {
			err = contextCode(p.ctx.Err())
		}
		if err != nil {
			s.finishPreparation(p, err)
			s.client.mu.Unlock()
			return
		}
		missing := p.needs &^ completed
		if missing == 0 {
			if completed&wbiPreparation != 0 {
				s.keys = keys
			}
			if completed&browserPreparation != 0 {
				s.browser = browser
			}
			s.finishPreparation(p, nil)
			s.client.mu.Unlock()
			return
		}
		s.client.mu.Unlock()
		if missing&wbiPreparation != 0 {
			var body []byte
			body, _, err = s.request(p.ctx, "https://api.bilibili.com/x/web-interface/nav", "", jsonLimit, false, true)
			if err == nil {
				var value envelope
				value, err = s.acceptJSON(p.ctx, Nav, body)
				if err == nil {
					keys, err = signingKeys(value, s.client.now())
				}
			}
			if err == nil {
				completed |= wbiPreparation
			}
		}
		if err == nil && missing&browserPreparation != 0 {
			_, headers, requestErr := s.request(p.ctx, "https://www.bilibili.com/", "", jsonLimit, false, false)
			err = requestErr
			if err == nil {
				browser, err = browserCookies(headers.Values("Set-Cookie"), s.client.now())
			}
			if err == nil {
				completed |= browserPreparation
			}
		}
		if err != nil {
			s.client.mu.Lock()
			s.finishPreparation(p, err)
			s.client.mu.Unlock()
			return
		}
	}
}
