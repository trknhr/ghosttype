package model

type ModelStatus int

const (
	ModelUnknown ModelStatus = iota
	ModelReady
	ModelError
)

type ModelInitEvent struct {
	Name   string
	Status ModelStatus
	Err    error
}
