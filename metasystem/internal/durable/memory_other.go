//go:build !darwin

package durable

import "os"

func memoryVolume(*os.File) bool { return false }
