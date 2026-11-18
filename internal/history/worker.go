package history

import "slices"

type storeOperation uint8

const (
	noOperation storeOperation = iota
	loadStore
	saveStore
	resetStore
	closeStore
)

type storeRequest struct {
	operation storeOperation
	records   []Record
	revision  uint64
}

type storeResult struct {
	request  storeRequest
	records  []Record
	acquired bool
	err      error
	consumed chan struct{}
}

func runStore(store Store, requests <-chan storeRequest, results chan<- storeResult, retired chan<- struct{}) {
	defer close(retired)
	for request := range requests {
		result := storeResult{request: request, consumed: make(chan struct{})}
		switch request.operation {
		case loadStore:
			result.err = store.Acquire()
			result.acquired = result.err == nil
			if result.acquired {
				result.records, result.err = store.Load()
				result.records = slices.Clone(result.records)
			}
		case saveStore, resetStore:
			result.err = store.Save(request.records)
		case closeStore:
			result.err = store.Close()
		}
		results <- result
		<-result.consumed
		if request.operation == closeStore && result.err == nil {
			return
		}
	}
}
