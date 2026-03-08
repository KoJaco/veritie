package pipeline

// Worker is a long-running goroutine that processes audio data.
// It is responsible for transcribing, parsing, and summarizing audio data.
// Also included is back-pressure handling, WaitGroup cleanuo, etc.
