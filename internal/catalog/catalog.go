// Package catalog owns the framework-free video detail model.
package catalog

import "time"

type VideoID string
type PartID string

// Part identifies one playable section. Resource locations belong to adapters.
type Part struct {
	ID       PartID
	Title    string
	Duration time.Duration
}

// Video contains display data, with an opaque cover identity for presentation.
type Video struct {
	ID                          VideoID
	Title, Creator, Description string
	CoverID                     string
	Parts                       []Part
}
