package main

// sourceCloseObserver joins source ownership without blocking the window's UI
// thread or relying on another paint. Its methods belong to the UI thread;
// postClose must enqueue a close action through the native dispatcher.
type sourceCloseObserver struct{ finished <-chan struct{} }

func (o *sourceCloseObserver) ready(sourceDone <-chan struct{}, postClose func()) bool {
	select {
	case <-sourceDone:
		return true
	default:
	}
	if o.finished == nil {
		finished := make(chan struct{})
		o.finished = finished
		go func() {
			defer close(finished)
			<-sourceDone
			postClose()
		}()
	}
	return false
}

func (o *sourceCloseObserver) wait() {
	if o.finished != nil {
		<-o.finished
	}
}
