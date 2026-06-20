package audio

import (
	"encoding/binary"
	"math"
)

const (
	kBands      = 16
	kFrameSize  = 1024 // 64ms @ 16kHz
	kHopSize    = 512  // 50% overlap = 32ms
	kSpecBufCap = kFrameSize + kHopSize
	kMinFreqHz  = 60.0
	kMaxFreqHz  = 4000.0
	kSampleRate = 16000.0
)

// LevelMeter is a one-pole envelope follower with peak-hold, ported from
// ai-voice-tool's spectrum.cpp. attack rises fast, release falls slowly, so
// the displayed level looks like a real VU meter instead of a twitchy RMS.
type LevelMeter struct {
	attackAlpha  float64
	releaseAlpha float64
	peakHold     float64
	smoothed     float64
	peak         float64
	peakTimer    float64
}

func newLevelMeter(attack, release, peakHold, rateHz float64) LevelMeter {
	dt := 1.0 / rateHz
	return LevelMeter{
		attackAlpha:  1.0 - math.Exp(-dt/math.Max(attack, 0.001)),
		releaseAlpha: 1.0 - math.Exp(-dt/math.Max(release, 0.001)),
		peakHold:     peakHold,
	}
}

func (m *LevelMeter) update(input float64) {
	alpha := m.attackAlpha
	if input < m.smoothed {
		alpha = m.releaseAlpha
	}
	m.smoothed += alpha * (input - m.smoothed)

	if m.smoothed >= m.peak {
		m.peak = m.smoothed
		m.peakTimer = m.peakHold
	} else {
		m.peakTimer -= (1.0 / 30.0)
		if m.peakTimer <= 0 {
			decay := 1.0 - math.Exp(-(1.0/30.0)/math.Max(m.releaseAlpha*1.5, 0.001))
			m.peak += decay * (m.smoothed - m.peak)
		}
	}
}

// SpectrumAnalyzer accumulates PCM into overlapping 1024-sample frames,
// runs a Hann-windowed FFT, maps to 16 linear bands, normalizes by a rolling
// max, log-compresses, and smooths each band with a LevelMeter.
//
// Process(pcm) feeds PCM incrementally; it invokes onBands whenever a full
// frame produces a new 16-band spectrum.
type SpectrumAnalyzer struct {
	specBuf     [kSpecBufCap]int16
	specHead    int
	bands       [kBands]float64
	meters      [kBands]LevelMeter
	rollingMax  float64
	hann        [kFrameSize]float64
	binStart    [kBands]int
	binEnd      [kBands]int
	onBands     func([kBands]float64)
	frameCount  int
}

// NewSpectrumAnalyzer creates an analyzer. onBands is called (from the same
// goroutine that calls Process) whenever a new spectrum frame is ready.
func NewSpectrumAnalyzer(onBands func([kBands]float64)) *SpectrumAnalyzer {
	s := &SpectrumAnalyzer{
		rollingMax: 0.01,
		onBands:    onBands,
	}
	for i := 0; i < kFrameSize; i++ {
		s.hann[i] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(kFrameSize-1)))
	}
	// precompute band -> bin ranges
	binHz := kSampleRate / float64(kFrameSize)
	nBins := kFrameSize/2 + 1
	minBin := int(math.Max(1, math.Ceil(kMinFreqHz/binHz)))
	maxBin := int(math.Min(float64(nBins-1), math.Floor(kMaxFreqHz/binHz)))
	binsPerBand := float64(maxBin-minBin+1) / float64(kBands)
	for b := 0; b < kBands; b++ {
		start := minBin + int(float64(b)*binsPerBand)
		end := minBin + int(float64(b+1)*binsPerBand) - 1
		if end > maxBin {
			end = maxBin
		}
		s.binStart[b] = start
		s.binEnd[b] = end
	}
	for b := 0; b < kBands; b++ {
		s.meters[b] = newLevelMeter(0.04, 0.15, 0.10, 30.0)
	}
	return s
}

// Process feeds PCM bytes (16-bit LE) and may emit a spectrum frame.
func (s *SpectrumAnalyzer) Process(pcm []byte) {
	samples := len(pcm) / 2
	if samples == 0 {
		return
	}
	for i := 0; i < samples; i++ {
		sv := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		s.specBuf[s.specHead] = sv
		s.specHead++
		if s.specHead == kFrameSize {
			s.computeFrame()
			// shift: keep last kHopSize samples (50% overlap)
			copy(s.specBuf[:kHopSize], s.specBuf[kFrameSize-kHopSize:kFrameSize])
			s.specHead = kHopSize
		}
	}
}

func (s *SpectrumAnalyzer) computeFrame() {
	// 1. Hann window + int16 -> float [-1,1]
	windowed := make([]float64, kFrameSize)
	for i := 0; i < kFrameSize; i++ {
		windowed[i] = float64(s.specBuf[i]) * s.hann[i] / 32768.0
	}
	// 2. real FFT -> magnitudes
	mag := realFFT(windowed) // len = kFrameSize/2+1 = 513

	// 3. map bins to 16 bands + track frame max
	var frameMax float64
	for b := 0; b < kBands; b++ {
		var sum float64
		count := 0
		for i := s.binStart[b]; i <= s.binEnd[b]; i++ {
			sum += mag[i]
			count++
		}
		avg := 0.0
		if count > 0 {
			avg = sum / float64(count)
		}
		if avg > frameMax {
			frameMax = avg
		}
		s.bands[b] = avg
	}

	// 4. rolling-max normalize
	s.rollingMax += 0.001 * (frameMax - s.rollingMax)
	if s.rollingMax < 0.01 {
		s.rollingMax = 0.01
	}
	for b := 0; b < kBands; b++ {
		v := s.bands[b] / s.rollingMax
		if v > 1 {
			v = 1
		}
		// 5. log compress
		s.bands[b] = math.Log10(1 + 9*v)
	}

	// 6. smooth via envelope followers
	for b := 0; b < kBands; b++ {
		s.meters[b].update(s.bands[b])
		s.bands[b] = s.meters[b].smoothed
	}

	s.frameCount++
	if s.onBands != nil {
		s.onBands(s.bands)
	}
}

// Clear resets all band state (e.g. when recording stops).
func (s *SpectrumAnalyzer) Clear() {
	s.specHead = 0
	s.rollingMax = 0.01
	for b := 0; b < kBands; b++ {
		s.bands[b] = 0
		s.meters[b] = newLevelMeter(0.04, 0.15, 0.10, 30.0)
	}
}
