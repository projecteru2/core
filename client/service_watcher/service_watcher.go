package service_watcher

import "context"

type ServiceWatcher interface {
	Watch(context.Context) <-chan []string
}
