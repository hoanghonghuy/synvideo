package ingestvalidation

import (
	"bytes"
	"sync"
	"testing"
)

func TestBoundedProbeOutputEnforcesAggregateBudget(t *testing.T) {
	sink := newBoundedProbeOutput(ProbeLogMaxBytes)
	chunk := bytes.Repeat([]byte("x"), 1024)
	for i := 0; i < (ProbeLogMaxBytes/1024)+2; i++ {
		if _, err := sink.Write(chunk); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if len(sink.Bytes()) != ProbeLogMaxBytes {
		t.Fatalf("captured %d bytes, want aggregate cap %d", len(sink.Bytes()), ProbeLogMaxBytes)
	}
}

func TestBoundedProbeOutputConcurrentWritesAreRaceSafe(t *testing.T) {
	sink := newBoundedProbeOutput(ProbeLogMaxBytes)
	chunk := bytes.Repeat([]byte("a"), 512)
	var wg sync.WaitGroup
	for i := 0; i < 128; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = sink.Write(chunk)
		}()
		go func() {
			defer wg.Done()
			_, _ = sink.Write(chunk)
		}()
	}
	wg.Wait()
	if len(sink.Bytes()) > ProbeLogMaxBytes {
		t.Fatalf("concurrent capture exceeded cap: %d > %d", len(sink.Bytes()), ProbeLogMaxBytes)
	}
}
