package internal

import (
	"log"
	"os"
)

var (
	Debug = log.New(os.Stderr, "DEBUG: ", log.Ltime|log.Lshortfile)
	Info  = log.New(os.Stderr, "INFO:  ", log.Ltime|log.Lshortfile)
	Error = log.New(os.Stderr, "ERROR: ", log.Ltime|log.Lshortfile)
)
