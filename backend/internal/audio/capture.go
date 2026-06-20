// Package audio provides microphone capture via the Win32 waveIn API (pure Go,
// no CGO) plus a 16-band FFT spectrum analyzer. Ported and adapted from the
// ai-voice-tool project, with device selection and a non-busy polling loop.
//
// Windows-only (waveIn). fft.go and spectrum.go are platform-independent.

//go:build windows

package audio

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	winmm = syscall.NewLazyDLL("winmm.dll")

	waveInOpen            = winmm.NewProc("waveInOpen")
	waveInClose           = winmm.NewProc("waveInClose")
	waveInPrepareHeader   = winmm.NewProc("waveInPrepareHeader")
	waveInUnprepareHeader = winmm.NewProc("waveInUnprepareHeader")
	waveInAddBuffer       = winmm.NewProc("waveInAddBuffer")
	waveInStart           = winmm.NewProc("waveInStart")
	waveInStop            = winmm.NewProc("waveInStop")
	waveInReset           = winmm.NewProc("waveInReset")
	waveInGetNumDevs      = winmm.NewProc("waveInGetNumDevs")
)

const (
	WAVE_FORMAT_PCM  = 1
	WHDR_DONE        = 0x00000001
	WHDR_PREPARED    = 0x00000002
	CALLBACK_NULL    = 0x00000000
	MMSYSERR_NOERROR = 0
	WAVE_MAPPER      = 0xFFFFFFFF
)

type PCMFormat struct {
	SampleRate    int
	BitsPerSample int
	Channels      int
}

type waveFormatEx struct {
	wFormatTag      uint16
	nChannels       uint16
	nSamplesPerSec  uint32
	nAvgBytesPerSec uint32
	nBlockAlign     uint16
	wBitsPerSample  uint16
	cbSize          uint16
}

type waveHdr struct {
	lpData          uintptr
	dwBufferLength  uint32
	dwBytesRecorded uint32
	dwUser          uintptr
	dwFlags         uint32
	dwLoops         uint32
	lpNext          uintptr
	reserved        uintptr
}

// Capture owns a Win32 waveIn handle and a pool of buffers. Completed buffers
// are polled and dispatched to the registered PCM callback. All captured PCM
// is also accumulated into a full buffer accessible via TakePCM().
type Capture struct {
	mu         sync.Mutex
	hWaveIn    uintptr
	buffers    []*waveHdr
	bufData    [][]byte
	running    bool
	pcmFn      func([]byte) // called for each completed buffer
	sampleRate int
	bufCount   int
	bufSize    int
	deviceID   uintptr

	// full accumulated PCM (for ASR after stop)
	accumulated []byte
}

// NewCapture creates a capture for the given format. deviceId <= 0 (or
// WAVE_MAPPER) selects the system default recording device.
func NewCapture(format PCMFormat, deviceID int) *Capture {
	dev := uintptr(deviceID)
	if deviceID <= 0 {
		dev = WAVE_MAPPER
	}
	return &Capture{
		sampleRate: format.SampleRate,
		bufCount:   8,
		bufSize:    format.SampleRate * format.BitsPerSample / 8 * format.Channels / 10, // 100ms buffers
		deviceID:   dev,
	}
}

func (c *Capture) NumDevices() int {
	ret, _, _ := waveInGetNumDevs.Call()
	return int(ret)
}

// SetPCMCallback registers the function invoked with each completed PCM buffer.
func (c *Capture) SetPCMCallback(fn func([]byte)) {
	c.mu.Lock()
	c.pcmFn = fn
	c.mu.Unlock()
}

func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	var wfmt waveFormatEx
	wfmt.wFormatTag = WAVE_FORMAT_PCM
	wfmt.nChannels = 1
	wfmt.nSamplesPerSec = uint32(c.sampleRate)
	wfmt.wBitsPerSample = 16
	wfmt.nBlockAlign = wfmt.nChannels * wfmt.wBitsPerSample / 8
	wfmt.nAvgBytesPerSec = wfmt.nSamplesPerSec * uint32(wfmt.nBlockAlign)

	ret, _, _ := waveInOpen.Call(
		uintptr(unsafe.Pointer(&c.hWaveIn)),
		c.deviceID,
		uintptr(unsafe.Pointer(&wfmt)),
		0, 0,
		CALLBACK_NULL,
	)
	if ret != MMSYSERR_NOERROR {
		return fmt.Errorf("waveInOpen failed: error %d", ret)
	}

	c.buffers = make([]*waveHdr, c.bufCount)
	c.bufData = make([][]byte, c.bufCount)
	c.accumulated = nil

	for i := 0; i < c.bufCount; i++ {
		buf := make([]byte, c.bufSize)
		c.bufData[i] = buf

		hdr := &waveHdr{
			lpData:         uintptr(unsafe.Pointer(&buf[0])),
			dwBufferLength: uint32(c.bufSize),
		}

		ret, _, _ := waveInPrepareHeader.Call(c.hWaveIn, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(waveHdr{}))
		if ret != MMSYSERR_NOERROR {
			c.cleanup()
			return fmt.Errorf("waveInPrepareHeader failed: error %d", ret)
		}

		ret, _, _ = waveInAddBuffer.Call(c.hWaveIn, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(waveHdr{}))
		if ret != MMSYSERR_NOERROR {
			c.cleanup()
			return fmt.Errorf("waveInAddBuffer failed: error %d", ret)
		}

		c.buffers[i] = hdr
	}

	ret, _, _ = waveInStart.Call(c.hWaveIn)
	if ret != MMSYSERR_NOERROR {
		c.cleanup()
		return fmt.Errorf("waveInStart failed: error %d", ret)
	}

	c.running = true
	log.Printf("[audio] capture started: %d Hz, device=%d, %d buffers x %d bytes",
		c.sampleRate, c.deviceID, c.bufCount, c.bufSize)

	go c.pollBuffers()
	return nil
}

func (c *Capture) Stop() {
	c.mu.Lock()
	running := c.running
	c.running = false
	c.mu.Unlock()

	if !running {
		return
	}

	waveInStop.Call(c.hWaveIn)
	waveInReset.Call(c.hWaveIn)
	c.cleanup()
	log.Printf("[audio] capture stopped, accumulated %d bytes", len(c.accumulated))
}

func (c *Capture) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// TakePCM returns and clears the full accumulated PCM buffer (for ASR).
func (c *Capture) TakePCM() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.accumulated
	c.accumulated = nil
	return out
}

func (c *Capture) pollBuffers() {
	for c.IsRunning() {
		for i := 0; i < c.bufCount; i++ {
			c.mu.Lock()
			hdr := c.buffers[i]
			c.mu.Unlock()

			if hdr == nil {
				continue
			}

			if hdr.dwFlags&WHDR_DONE != 0 && hdr.dwBytesRecorded > 0 {
				data := make([]byte, hdr.dwBytesRecorded)
				copy(data, c.bufData[i][:hdr.dwBytesRecorded])

				c.mu.Lock()
				c.accumulated = append(c.accumulated, data...)
				pcmCb := c.pcmFn
				c.mu.Unlock()

				if pcmCb != nil {
					pcmCb(data)
				}

				// re-queue
				hdr.dwFlags = 0
				hdr.dwBytesRecorded = 0
				waveInPrepareHeader.Call(c.hWaveIn, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(waveHdr{}))
				waveInAddBuffer.Call(c.hWaveIn, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(waveHdr{}))
			}
		}
		// avoid busy-poll pegging a core (improvement over ai-voice-tool)
		time.Sleep(2 * time.Millisecond)
	}
}

func (c *Capture) cleanup() {
	for i, hdr := range c.buffers {
		if hdr != nil && hdr.dwFlags&WHDR_PREPARED != 0 {
			waveInUnprepareHeader.Call(c.hWaveIn, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(waveHdr{}))
		}
		c.buffers[i] = nil
		c.bufData[i] = nil
	}
	if c.hWaveIn != 0 {
		waveInClose.Call(c.hWaveIn)
		c.hWaveIn = 0
	}
}

// ComputeRMS16 returns the RMS amplitude of 16-bit LE PCM, normalized [0,1].
func ComputeRMS16(pcm []byte) float64 {
	if len(pcm) < 2 {
		return 0
	}
	samples := len(pcm) / 2
	if samples == 0 {
		return 0
	}
	var sumSq float64
	for i := 0; i < samples; i++ {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		f := float64(s) / 32768.0
		sumSq += f * f
	}
	rms := math.Sqrt(sumSq / float64(samples))
	if rms > 1 {
		rms = 1
	}
	return rms
}
