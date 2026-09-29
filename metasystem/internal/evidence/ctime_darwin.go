package evidence

import "syscall"

func ctimeNs(stat *syscall.Stat_t) int64 { return stat.Ctimespec.Nano() }
