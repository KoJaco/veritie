package sttgoogle

// TODO: Adjust pipeline to depend only on speech.STTClient, all old calls to StartStream, SendAudioChunk, ReadResponse collapse into `transcripts, err := p.deps.STT.Stream(ctx, audioChan)`
