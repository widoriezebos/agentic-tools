package gocache

import "os"

// TrimHooks are the test-only observation points of one trim pass.
type TrimHooks struct {
	AfterLock    func(*os.File)
	AfterMeasure func()
	BeforeStat   func(shard, name string)
	BeforeRemove func(shard, name string)
}

// WithTrimHooks returns cfg with the test hooks set.
func WithTrimHooks(cfg TrimConfig, hooks TrimHooks) TrimConfig {
	cfg.hooks = trimHooks{afterLock: hooks.AfterLock, afterMeasure: hooks.AfterMeasure, beforeStat: hooks.BeforeStat, beforeRemove: hooks.BeforeRemove}
	return cfg
}
