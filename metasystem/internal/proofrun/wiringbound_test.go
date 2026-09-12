package proofrun

import "time"

// wiringBound is only a dead-test backstop. The facts under test, rather
// than elapsed time, release every real-process or goroutine handshake.
const wiringBound = 30 * time.Second
