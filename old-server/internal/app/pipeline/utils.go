package pipeline

import (
	"time"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// helper: wait for first chunk, returns it + a proxy channel
func LazyConnect(upstream <-chan speech.AudioChunk) (
	first speech.AudioChunk,
	toSTT chan speech.AudioChunk,
	ok bool,
) {
	first, ok = <-upstream
	if !ok {
		return
	}

	toSTT = make(chan speech.AudioChunk, 64)

	go func() {
		// flush first buffered chunk
		firstChunkTime := time.Now()
		toSTT <- first
		logger.ServiceDebugf("PIPELINE", "TIMING: First audio chunk forwarded to STT at %s",
			firstChunkTime.Format("15:04:05.000"))
		

		// relay until upstream closes
		for c := range upstream {
			chunkForwardTime := time.Now()
			toSTT <- c

			logger.ServiceDebugf("PIPELINE", "TIMING: Audio chunk forwarded to STT at %s (size: %d bytes)",
				chunkForwardTime.Format("15:04:05.000"), len(c))
		}

		close(toSTT)
	}()

	return
}
