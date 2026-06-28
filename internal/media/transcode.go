package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// videoEncodeArgs returns the ffmpeg args for the re-encode fallback path,
// used only when the source codec isn't browser-playable — the common case is
// copy, which is lossless and free. Software x264 with a slow preset / low CRF:
// re-encode is a rare background job and the project prioritizes quality.
// (Hardware-accelerated encoders come later, with the Linux deploy.)
func videoEncodeArgs() []string {
	return []string{
		"-c:v", "libx264", "-preset", "slow", "-crf", "18",
		"-pix_fmt", "yuv420p",
	}
}

// aacBitrate scales the AAC target bitrate with channel count so multichannel
// audio isn't starved (192k is plenty for stereo, far too little for 5.1).
func aacBitrate(channels int) string {
	switch {
	case channels <= 2:
		return "192k"
	case channels <= 6:
		return "384k"
	default:
		return "512k"
	}
}

// Plan describes how a source file will be turned into a browser-playable MP4.
// It is the output of probe-and-branch and drives the ffmpeg invocation.
type Plan struct {
	VideoCopy      bool   // true = remux video bytes as-is (direct play); false = re-encode
	VideoCodec     string // source video codec (for logging / UI hints)
	HDR            bool   // HDR10/HLG/DV present — passed through untouched on copy
	AudioTracks    []AudioPlan
	SubtitleTracks []SubtitlePlan
	SourceWidth    int
	SourceHeight   int
}

type AudioPlan struct {
	SrcIndex  int    // stream index within the source file
	Transcode bool   // true = encode to AAC; false = copy
	Codec     string // source codec
	Language  string
	Title     string
	Channels  int
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
		VideoCopy:    !p.NeedsVideoTranscode(),
		VideoCodec:   v.CodecName,
		HDR:          p.IsHDR(),
		SourceWidth:  v.Width,
		SourceHeight: v.Height,
	}

	for _, a := range p.AudioStreams() {
		plan.AudioTracks = append(plan.AudioTracks, AudioPlan{
			SrcIndex:  a.Index,
			Transcode: AudioNeedsTranscode(a.CodecName),
			Codec:     a.CodecName,
			Language:  a.Tags.Language,
			Title:     a.Tags.Title,
			Channels:  a.Channels,
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

// Process executes the plan: produces a single progressive MP4 at dstPath
// containing the (possibly remuxed) video and all audio tracks as AAC/copy.
// +faststart moves the moov atom to the front so the browser can seek
// immediately over HTTP range requests.
//
// durationSec is the source duration; if > 0 and onProgress is non-nil, ffmpeg
// progress is parsed and reported as a 0..100 percentage (roughly once a
// second). onProgress may be nil to skip progress tracking.
func Process(ctx context.Context, srcPath, dstPath string, plan *Plan,
	durationSec float64, onProgress func(percent float64)) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Global flags first: silence the banner and human stats. -progress streams
	// machine-readable key=value progress to stdout; -stats_period bounds it to
	// one update per second so we don't spam updates.
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostats",
		"-progress", "pipe:1", "-stats_period", "1",
		"-y", "-i", srcPath,
	}

	// --- Video ---
	args = append(args, "-map", "0:v:0")
	if plan.VideoCopy {
		args = append(args, "-c:v", "copy") // lossless remux — best quality, ~0 CPU
		// HEVC in MP4 must carry the 'hvc1' tag or Safari refuses to play it
		// (ffmpeg otherwise writes 'hev1', which <video> won't decode).
		if plan.VideoCodec == "hevc" {
			args = append(args, "-tag:v", "hvc1")
		}
	} else {
		args = append(args, videoEncodeArgs()...)
	}

	// --- Audio: map every track, encode to AAC unless already browser-safe ---
	for outIdx, a := range plan.AudioTracks {
		args = append(args, "-map", "0:"+strconv.Itoa(a.SrcIndex))
		streamSpec := "-c:a:" + strconv.Itoa(outIdx)
		if a.Transcode {
			// Encode to AAC, preserving the source channel layout; bitrate
			// scales with channel count.
			args = append(args, streamSpec, "aac",
				"-b:a:"+strconv.Itoa(outIdx), aacBitrate(a.Channels))
		} else {
			args = append(args, streamSpec, "copy")
		}
		// Preserve language/title metadata for the track-selection UI.
		if a.Language != "" {
			args = append(args, "-metadata:s:a:"+strconv.Itoa(outIdx),
				"language="+a.Language)
		}
		if a.Title != "" {
			args = append(args, "-metadata:s:a:"+strconv.Itoa(outIdx),
				"title="+a.Title)
		}
	}

	// No subtitles in the muxed output yet — handled separately in Step 4.
	args = append(args, "-sn")

	args = append(args,
		"-movflags", "+faststart",
		"-f", "mp4",
		dstPath,
	)

	return runFFmpeg(ctx, args, durationSec, onProgress)
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
