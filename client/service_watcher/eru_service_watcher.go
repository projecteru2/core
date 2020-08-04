package service_watcher

import "context"

type eruServiceWatcher struct{}

func New() *eruServiceWatcher {
	return nil
}

func (w *eruServiceWatcher) Watch(ctc context.Context) <-chan []string {
	return nil
}
