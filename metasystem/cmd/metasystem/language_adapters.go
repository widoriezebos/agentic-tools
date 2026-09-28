package main

// The language adapters the testing contract and the landing batch detect a
// candidate's language through (internal/testpolicy/adapter). Each registers
// itself when linked; the engine links every built-in one here.
import (
	_ "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
)
