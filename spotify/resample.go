package spotify

import (
	"time"
)

type Resampler struct {
	in          <-chan [][]float32
	out         chan [][]float32
	currentRate int
	targetRate  int
	quality     int
	throttle    time.Duration
}

func NewResampler(stream <-chan [][]float32, current int, target int, quality int) *Resampler {
	// Negative and zero value quality isn't really a thing. Default it to 1.
	if quality <= 0 {
		quality = 1
	}

	return &Resampler{
		in:          stream,
		out:         make(chan [][]float32, 10),
		currentRate: current,
		targetRate:  target,
		quality:     quality,
		throttle:    0,
	}
}

func (r *Resampler) SetBufferSize(size int) {
	r.out = make(chan [][]float32, size)
}

func (r *Resampler) SetThrottle(d time.Duration) {
	r.throttle = d
}

func (r *Resampler) SamplesOut() <-chan [][]float32 {
	return r.out
}

func (r *Resampler) Resample() {
	if r.currentRate == r.targetRate {
		r.dontResample()
		return
	}

	// Allocate a slice of points to use for interpolation
	points := make([]point, r.quality*2)

	// Keep track of the current total offset of the entire track. This is moved chunks at a time.
	offset := 0

	// Keep track of the individual number of samples processed.
	samplesProcessed := 0

	// Calculate the resample ratio
	ratio := float64(r.currentRate) / float64(r.targetRate)

	// For this to work we will need two buffers worth of data because as we approach the end of the current buffer we
	// will need data points from future samples in order to appropriate calculate the interpolation of the point.
	//
	// At the very beginning of the track though we can just store the same buffer in each twice. This logic may change,
	// but it makes the most sense at the moment.

	// Create a buffer for storing the previous sample.
	var previousSamples [][]float32
	var currentSamples [][]float32
	var resampled [][]float32
	var ok bool

	// Begin iterating over the in channel until it's closed.
	for {
		// At the very beginning of processing, we should just duplicate the data in currentSample to previousSample.
		if samplesProcessed == 0 {
			currentSamples, ok = <-r.in

			// Verify that the channel isn't closed, and we weren't sent an empty slice.
			if !ok || currentSamples == nil || len(currentSamples) == 0 {
				close(r.out)
				return
			}

			previousSamples = make([][]float32, len(currentSamples))
			for i := range previousSamples {
				previousSamples[i] = make([]float32, len(currentSamples[0]))
			}
			copy(previousSamples, currentSamples)
		}

		// We need to remake this every time because slices are passed by reference inherently. We don't want to assume
		// the consumer of the out channel is keeping up with the sends.
		resampled = make([][]float32, len(currentSamples))
		for i := range resampled {
			resampled[i] = make([]float32, len(currentSamples[0]))
		}

		for sampleIndex, sample := range currentSamples {
			for channelIndex := range sample {
				scaledOffset := float64(samplesProcessed) * ratio

				// Build the set of surrounding points to calculate the value for the sample.
				for pIndex := range points {
					// Calculate the index of the sample to add to the set
					i := int(scaledOffset) + pIndex - len(points)/2 + 1

					switch {
					// The sample is in previousSamples.
					case i < offset:
						points[pIndex] = point{X: float32(i), Y: previousSamples[len(previousSamples)+i-offset][channelIndex]}
					// The sample is in currentSamples.
					case i < offset+len(currentSamples):
						points[pIndex] = point{X: float32(i), Y: currentSamples[i-offset][channelIndex]}
					// The sample is beyond currentSamples.
					default:
						offset += len(currentSamples)
						previousSamples, currentSamples = currentSamples, previousSamples

						// There's nothing more to process, time to exit.
						if currentSamples, ok = <-r.in; !ok {
							close(r.out)
							return
						}

						points[pIndex] = point{X: float32(i), Y: currentSamples[i-offset][channelIndex]}
					}
				}
				resampled[sampleIndex][channelIndex] = LagInt(points, float32(scaledOffset))
			}
			samplesProcessed++
		}
		r.out <- resampled

		// Throttle if requested. This is useful for when you want resample fast but not max out available cores.
		time.Sleep(r.throttle)
	}
}

// ResampleAll is a convenience function for resampling a complete set of samples so no channels are needed.
func (r *Resampler) ResampleAll(samples [][]float32) [][]float32 {
	return nil
}

func Resample(samples [][]float32, old int, new int, quality int) [][]float32 {
	ratio := float64(old) / float64(new)
	points := make([]point, quality*2)

	resampled := make([][]float32, 0, len(samples))
	for i := range resampled {
		resampled[i] = make([]float32, 0, len(samples[0]))
	}

	for i := 0; ; i++ {
		j := float64(i) * ratio
		if int(j) >= len(samples) {
			break
		}
		sample := make([]float32, len(samples[0]))
		for channelIndex := range sample {
			for pIndex := range points {
				k := int(j) + pIndex - len(points)/2 + 1
				if k >= 0 && k < len(samples) {
					points[pIndex] = point{X: float32(k), Y: samples[k][channelIndex]}
				} else {
					points[pIndex] = point{X: float32(k), Y: 0}
				}
			}
			y := LagInt(points[:], float32(j))
			sample[channelIndex] = y
		}
		resampled = append(resampled, sample)
	}

	return resampled
}

func (r *Resampler) dontResample() {
	for sample := range r.in {
		r.out <- sample
	}
	close(r.out)
}

// LagInt calculates the Lagrange interpolating polynomial y for x in a given set of points. Useful for resampling
// stereo audio.
// Read more here: https://en.wikipedia.org/wiki/Lagrange_polynomial.
func LagInt(pts []point, x float32) (y float32) {
	y = float32(0.0)

	for i := range pts {
		mu := pts[i].Y
		for j := range pts {
			if i != j {
				mu *= (x - pts[j].X) / (pts[i].X - pts[j].X)
			}
		}
		y += mu
	}

	return y
}

type point struct {
	X, Y float32
}
