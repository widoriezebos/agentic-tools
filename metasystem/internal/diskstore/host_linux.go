package diskstore

// readHostTempRoot is /tmp on Linux, whatever TMPDIR says.
var readHostTempRoot = func() (string, error) { return "/tmp", nil }
