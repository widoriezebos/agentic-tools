package broken

import "os"

func flush(f *os.File) error { return f.Close() }
