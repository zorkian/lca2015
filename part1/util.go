/*
   util.go

   Utility types/functions go in here. This is code that is reasonably self contained and does
   a very focused job, so we can isolate some of the testing.

   This code is released to the public domain. Originally prepared for the LCA 2015 conference
   by Mark Smith <mark@qq.is>.
*/

package main

import (
	"io"
	"log"
	"time"
)

type DelayedStartReader struct {
	Delay     time.Duration
	Content   string
	curOffset int
}

// Read will return some bytes from the Content string into the given slice. If this is the first
// time Read is called, it will first sleep for the requested delay period.
func (t *DelayedStartReader) Read(p []byte) (n int, err error) {
	if t.curOffset == 0 {
		time.Sleep(t.Delay)
	} else if t.curOffset >= len(t.Content) {
		return 0, io.EOF
	}

	bytesCopied := copy(p, t.Content[t.curOffset:])
	log.Printf("copied %d bytes from offset %d size %d to size %d", bytesCopied, t.curOffset, len(t.Content), len(p))
	t.curOffset += bytesCopied
	return bytesCopied, nil
}

func MakeDelayedStartReader(delay time.Duration, content string) io.Reader {
	return &DelayedStartReader{Delay: delay, Content: content}
}
