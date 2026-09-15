package api

const (
	defaultVideoTaggingFrameSamples = 3
	maxVideoTaggingFrameSamples     = 5
)

// videoTaggingFrameSampleTimes returns evenly distributed representative frame
// timestamps away from the exact beginning and end of a video. Frame analysis
// remains explicit; this helper only defines the deterministic sampling plan.
func videoTaggingFrameSampleTimes(durationSeconds float64, requested int) []float64 {
	if durationSeconds <= 0 {
		return nil
	}
	if requested <= 0 {
		requested = defaultVideoTaggingFrameSamples
	}
	if requested > maxVideoTaggingFrameSamples {
		requested = maxVideoTaggingFrameSamples
	}

	step := durationSeconds / float64(requested+1)
	times := make([]float64, requested)
	for index := range times {
		times[index] = step * float64(index+1)
	}
	return times
}
