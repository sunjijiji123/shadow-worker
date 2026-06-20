package audio

import "math"

// fft computes the in-place radix-2 Cooley-Tukey FFT of a complex sequence
// whose length must be a power of two. dir=+1 forward, dir=-1 inverse.
// Pure Go, no external deps; ~tens of µs for n=1024.
func fft(re, im []float64, dir int) {
	n := len(re)
	if n <= 1 {
		return
	}
	// bit-reversal permutation
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	// butterflies
	for length := 2; length <= n; length <<= 1 {
		half := length >> 1
		angle := float64(dir) * (-2.0 * math.Pi / float64(length))
		wStepRe := math.Cos(angle)
		wStepIm := math.Sin(angle)
		for i := 0; i < n; i += length {
			wRe, wIm := 1.0, 0.0
			for k := 0; k < half; k++ {
				a := i + k
				b := i + k + half
				tRe := wRe*re[b] - wIm*im[b]
				tIm := wRe*im[b] + wIm*re[b]
				re[b] = re[a] - tRe
				im[b] = im[a] - tIm
				re[a] += tRe
				im[a] += tIm
				nwRe := wRe*wStepRe - wIm*wStepIm
				wIm = wRe*wStepIm + wIm*wStepRe
				wRe = nwRe
			}
		}
	}
}

// realFFT computes the magnitudes of the first n/2+1 frequency bins of a real
// input signal of length n (n must be a power of two). Direct method: run a
// full complex FFT with im=0 and take |X[k]|. Simple and correct; fast enough
// for n=1024 (~50µs).
func realFFT(input []float64) []float64 {
	n := len(input)
	if n < 2 || n&(n-1) != 0 {
		return nil
	}
	re := make([]float64, n)
	copy(re, input)
	im := make([]float64, n)
	fft(re, im, 1)
	bins := n/2 + 1
	out := make([]float64, bins)
	for k := 0; k < bins; k++ {
		out[k] = math.Sqrt(re[k]*re[k] + im[k]*im[k])
	}
	return out
}
