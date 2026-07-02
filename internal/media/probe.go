package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ProbeResult struct {
	Streams []Stream `json:"streams"`
	Format  Format   `json:"format"`
}

type Stream struct {
	Index          int         `json:"index"`
	CodecType      string      `json:"codec_type"`
	CodecName      string      `json:"codec_name"`
	Profile        string      `json:"profile,omitempty"`
	Width          int         `json:"width,omitempty"`
	Height         int         `json:"height,omitempty"`
	PixFmt         string      `json:"pix_fmt,omitempty"`
	ColorTransfer  string      `json:"color_transfer,omitempty"`
	ColorPrimaries string      `json:"color_primaries,omitempty"`
	Channels       int         `json:"channels,omitempty"`
	Tags           StreamTags  `json:"tags,omitempty"`
	SideDataList   []SideData  `json:"side_data_list,omitempty"`
	Disposition    Disposition `json:"disposition,omitempty"`
}

type Disposition struct {
	Default int `json:"default"`
	Forced  int `json:"forced"`
}

type StreamTags struct {
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

type SideData struct {
	Type string `json:"side_data_type,omitempty"`
}

type Format struct {
	Filename   string `json:"filename"`
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
}

// Probe runs ffprobe and returns the parsed stream/format metadata.
func Probe(ctx context.Context, path string) (*ProbeResult, error) {
	// -v error keeps stdout pure JSON while still surfacing real errors on
	// stderr, which we attach to the returned error for debuggability.
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("ffprobe: %w: %s", err, ee.Stderr)
		}
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	var r ProbeResult
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("ffprobe json: %w", err)
	}
	return &r, nil
}

func (r *ProbeResult) VideoStream() *Stream {
	for i := range r.Streams {
		if r.Streams[i].CodecType == "video" {
			return &r.Streams[i]
		}
	}
	return nil
}

func (r *ProbeResult) AudioStreams() []Stream {
	var out []Stream
	for _, s := range r.Streams {
		if s.CodecType == "audio" {
			out = append(out, s)
		}
	}
	return out
}

func (r *ProbeResult) SubtitleStreams() []Stream {
	var out []Stream
	for _, s := range r.Streams {
		if s.CodecType == "subtitle" {
			out = append(out, s)
		}
	}
	return out
}

// IsHDR detects HDR10 (PQ transfer) and HLG.
// Dolby Vision is recognized but treated as HDR10 (we use the base layer).
func (r *ProbeResult) IsHDR() bool {
	v := r.VideoStream()
	if v == nil {
		return false
	}
	switch v.ColorTransfer {
	case "smpte2084", "arib-std-b67":
		return true
	}
	for _, sd := range v.SideDataList {
		if sd.Type == "DOVI configuration record" {
			return true
		}
	}
	return false
}

// AudioNeedsTranscode reports whether an audio codec is not natively
// supported by browsers. AC-3, E-AC-3, DTS, TrueHD etc. all need AAC.
func AudioNeedsTranscode(codec string) bool {
	switch codec {
	case "aac", "mp3", "opus", "flac", "vorbis":
		return false
	default:
		return true
	}
}

// IsTextSubtitle reports whether a subtitle codec is text-based and thus
// convertible to WebVTT. Image subs (PGS/VobSub/DVB) are not — they'd need OCR,
// which is out of scope for now.
func IsTextSubtitle(codec string) bool {
	switch codec {
	case "subrip", "srt", "ass", "ssa", "webvtt", "mov_text", "text", "subviewer":
		return true
	default:
		return false
	}
}

// IsBrowserReady reports whether the source can be served directly without any
// processing: an MP4-family container with universally-playable H.264 video and
// audio that needs no transcode. This skips a full ~filesize remux and the
// extra disk it would cost. (HEVC is excluded: not universal, and a raw source
// may lack the 'hvc1' tag Safari needs.)
func (r *ProbeResult) IsBrowserReady() bool {
	v := r.VideoStream()
	if v == nil || v.CodecName != "h264" {
		return false
	}
	if !strings.Contains(r.Format.FormatName, "mp4") &&
		!strings.Contains(r.Format.FormatName, "mov") {
		return false
	}
	for _, a := range r.AudioStreams() {
		if AudioNeedsTranscode(a.CodecName) {
			return false
		}
	}
	// Text subtitles are only extracted on the HLS path; direct play would
	// silently lose them, so such sources go through packaging instead.
	for _, s := range r.SubtitleStreams() {
		if IsTextSubtitle(s.CodecName) {
			return false
		}
	}
	return true
}
