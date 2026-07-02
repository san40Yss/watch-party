package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// tailBuffer keeps only the last maxBytes written to it. ffmpeg can run for
// hours; this bounds how much of its stderr we hold in memory while still
// retaining the tail, which is where the actual error message lands.
type tailBuffer struct {
	max int
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.buf) }

// Plan describes how a source file will be packaged for the browser. It is the
// output of probe-and-branch and drives the HLS ffmpeg invocation.
type Plan struct {
	VideoCodec     string // source video codec ("h264" is copied; anything else is re-encoded)
	HDR            bool   // HDR10/HLG/DV present
	AudioTracks    []AudioPlan
	SubtitleTracks []SubtitlePlan
	SourceWidth    int
	SourceHeight   int
}

type AudioPlan struct {
	SrcIndex int    // stream index within the source file
	Codec    string // source codec
	Language string
	Title    string
	Channels int
}

type SubtitlePlan struct {
	SrcIndex int    // stream index within the source file
	Text     bool   // text-based (convertible to WebVTT); image subs are skipped
	Codec    string // source codec
	Language string
	Title    string
	Forced   bool
}

// BuildPlan runs probe-and-branch over a ProbeResult and decides the strategy.
func BuildPlan(p *ProbeResult) (*Plan, error) {
	v := p.VideoStream()
	if v == nil {
		return nil, fmt.Errorf("no video stream found")
	}

	plan := &Plan{
		VideoCodec:   v.CodecName,
		HDR:          p.IsHDR(),
		SourceWidth:  v.Width,
		SourceHeight: v.Height,
	}

	for _, a := range p.AudioStreams() {
		plan.AudioTracks = append(plan.AudioTracks, AudioPlan{
			SrcIndex: a.Index,
			Codec:    a.CodecName,
			Language: a.Tags.Language,
			Title:    a.Tags.Title,
			Channels: a.Channels,
		})
	}

	for _, s := range p.SubtitleStreams() {
		plan.SubtitleTracks = append(plan.SubtitleTracks, SubtitlePlan{
			SrcIndex: s.Index,
			Text:     IsTextSubtitle(s.CodecName),
			Codec:    s.CodecName,
			Language: s.Tags.Language,
			Title:    s.Tags.Title,
			Forced:   s.Disposition.Forced == 1,
		})
	}

	return plan, nil
}

// runFFmpeg executes ffmpeg with the given args, streaming progress (via the
// -progress pipe:1 the caller must have included) and capturing the tail of
// stderr for error reporting.
func runFFmpeg(ctx context.Context, args []string, durationSec float64, onProgress func(percent float64)) error {
	stderr := &tailBuffer{max: 64 << 10} // keep last 64 KiB
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	// Drain the progress stream until ffmpeg closes stdout on exit. This must
	// happen before Wait so the pipe doesn't block the process.
	if onProgress != nil && durationSec > 0 {
		parseProgress(stdout, durationSec, onProgress)
	} else {
		io.Copy(io.Discard, stdout)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, stderr)
	}
	return nil
}

// parseProgress reads ffmpeg's -progress output and reports a 0..100 percent
// based on out_time relative to the total duration.
func parseProgress(r io.Reader, durationSec float64, onProgress func(percent float64)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// Lines look like "out_time_us=12345678"; ignore everything else.
		v, ok := strings.CutPrefix(scanner.Text(), "out_time_us=")
		if !ok {
			continue
		}
		us, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			continue // "N/A" early in the run
		}
		pct := float64(us) / 1e6 / durationSec * 100
		if pct < 0 {
			pct = 0
		} else if pct > 100 {
			pct = 100
		}
		onProgress(pct)
	}
}
