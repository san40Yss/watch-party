package media

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SubtitleRendition is a WebVTT subtitle track produced during HLS packaging.
type SubtitleRendition struct {
	TrackIndex int
	SrcIndex   int
	Language   string
	Title      string
	Forced     bool
	Path       string // relative to the video's processed dir, e.g. "subs/s0.vtt"
}

// PackageHLS builds an HLS package under outDir from src:
//
//	master.m3u8
//	stream_video/{init_*,playlist.m3u8,seg_*.m4s}
//	stream_audN/{...}   (one dir per audio track)
//	subs/sN.vtt + subs/sN.m3u8   (one per text subtitle track)
//
// Video is copied (lossless, ~0 CPU); each audio track becomes its own AAC
// rendition and each text subtitle a WebVTT rendition, so every viewer picks
// their own audio + subtitles client-side. Returns the subtitle renditions for
// persistence. Image subtitles (PGS/VobSub) are skipped — they'd need OCR.
//
// Playback can start while packaging is still running: as soon as the first
// video segment is on disk, subtitles are extracted (a demux-only pass running
// alongside the encode), the master playlist is finalized and onWatchable
// fires. The stream playlists are EVENT-typed, so players keep polling for new
// segments and flip to normal VOD at the final ENDLIST.
func PackageHLS(ctx context.Context, src, outDir string, plan *Plan, targetHeight int, durationSec float64, onProgress func(percent float64), onWatchable func([]SubtitleRendition)) ([]SubtitleRendition, error) {
	// Start clean so a re-process at different settings (fewer tracks, different
	// segment naming) doesn't leave orphaned files behind.
	_ = os.RemoveAll(outDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	// If finalization fails while the encode is still running, returning must
	// also kill the encode — otherwise ffmpeg would keep writing for hours
	// after the job is already reported as failed.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The AV pass runs in the background so the package can be finalized
	// (subtitles + master rewrite) while it is still encoding.
	encDone := make(chan struct{})
	var encErr error
	go func() {
		defer close(encDone)
		encErr = packageAV(ctx, src, outDir, plan, targetHeight, durationSec, onProgress)
	}()

	// finalize computes the subtitle timestamp offset from the first segment
	// (MPEG-TS muxing adds a ~1.48s delay; WebVTT cues are extracted from 0 and
	// must be shifted onto that timeline), extracts the subtitles and rewrites
	// the master playlist with friendly names + subtitle renditions.
	var subs []SubtitleRendition
	finalize := func() error {
		tsOffset := videoStartPTS(ctx, outDir)
		var err error
		subs, err = extractSubtitles(ctx, src, outDir, plan, durationSec, tsOffset)
		if err != nil {
			return fmt.Errorf("subtitles: %w", err)
		}
		if err := rewriteMaster(filepath.Join(outDir, "master.m3u8"), plan.AudioTracks, subs); err != nil {
			return fmt.Errorf("master: %w", err)
		}
		return nil
	}

	finalized := false
	if waitWatchable(ctx, outDir, encDone) {
		if err := finalize(); err != nil {
			return nil, err
		}
		finalized = true
		if onWatchable != nil {
			onWatchable(subs)
		}
	}

	<-encDone
	if encErr != nil {
		return nil, encErr
	}
	// Sources that finish faster than the watchable poll (short files, copies)
	// take the classic path: finalize after the encode.
	if !finalized {
		if err := finalize(); err != nil {
			return nil, err
		}
	}
	return subs, nil
}

// waitWatchable polls until the package is minimally playable — the master
// playlist exists and the video playlist lists its first completed segment
// (ffmpeg rewrites the playlist only after a segment is fully written).
// Returns false when the encode ends first (or the context is canceled).
func waitWatchable(ctx context.Context, outDir string, encDone <-chan struct{}) bool {
	master := filepath.Join(outDir, "master.m3u8")
	videoPl := filepath.Join(outDir, "stream_video", "playlist.m3u8")
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-encDone:
			return false
		case <-tick.C:
			// The master must be fully written (it lists every variant) before
			// finalize rewrites it, or the rewrite would bake in a torso.
			if m, err := os.ReadFile(master); err != nil ||
				!strings.Contains(string(m), "#EXT-X-STREAM-INF") {
				continue
			}
			if pl, err := os.ReadFile(videoPl); err == nil && strings.Contains(string(pl), "seg_000.ts") {
				return true
			}
		}
	}
}

// packageAV runs the ffmpeg pass that produces the video + audio renditions.
func packageAV(ctx context.Context, src, outDir string, plan *Plan, targetHeight int, durationSec float64, onProgress func(percent float64)) error {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostats",
		"-progress", "pipe:1", "-stats_period", "1",
		"-y", "-i", src,
	}

	args = append(args, "-map", "0:v:0")
	for _, a := range plan.AudioTracks {
		args = append(args, "-map", "0:"+strconv.Itoa(a.SrcIndex))
	}

	// H.264 is the only codec browsers reliably decode over HLS/MSE. Copy it
	// as-is; transcode anything else (HEVC etc) to H.264, downscaling to the
	// requested height (never upscaling).
	if plan.VideoCodec == "h264" {
		args = append(args, "-c:v", "copy")
	} else {
		vf := fmt.Sprintf("scale=-2:min(ih\\,%d)", targetHeight)
		if plan.HDR {
			// HDR (PQ/HLG) → SDR: a naive 8-bit conversion washes the colors
			// out. Linearize, tonemap (hable) and convert to BT.709. The float
			// zscale pipeline is the accurate path; slower, but re-encodes are
			// rare background jobs and quality wins here.
			vf += ",zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709," +
				"tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv"
		}
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "20",
			"-vf", vf+",format=yuv420p")
	}
	// Downmix every track to stereo AAC. Multichannel (5.1) AAC fails to parse
	// in browser MSE / hls.js ("stream parsing failed" → bufferAppendError);
	// stereo plays everywhere. This is THE fix for HLS not playing in Chrome.
	args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")

	var sm strings.Builder
	sm.WriteString("v:0,agroup:aud,name:video")
	for i, a := range plan.AudioTracks {
		fmt.Fprintf(&sm, " a:%d,agroup:aud,name:aud%d", i, i)
		if a.Language != "" {
			sm.WriteString(",language:" + a.Language)
		}
		if i == 0 {
			sm.WriteString(",default:yes")
		}
	}

	// MPEG-TS segments (not fMP4): the traditional, most-compatible HLS format
	// for H.264 in hls.js. Since the video rendition is always H.264, TS is the
	// safe choice (fMP4 was only needed for HEVC, which we no longer keep).
	//
	// EVENT playlists (not VOD) are what makes watch-while-processing work:
	// players poll for appended segments during the encode, then treat the
	// final ENDLIST as VOD. Deliberately NO temp_file flag: with it, ffmpeg
	// lists a segment in the playlist before renaming it from .tmp, and players
	// chasing the live edge hit 404s; without it the playlist is only updated
	// after the segment file is complete (players never request unlisted names).
	args = append(args,
		"-var_stream_map", sm.String(),
		"-master_pl_name", "master.m3u8",
		"-f", "hls",
		"-hls_segment_type", "mpegts",
		"-hls_playlist_type", "event",
		"-hls_time", "6",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(outDir, "stream_%v", "seg_%03d.ts"),
		filepath.Join(outDir, "stream_%v", "playlist.m3u8"),
	)

	return runFFmpeg(ctx, args, durationSec, onProgress)
}

// videoStartPTS returns the first video segment's start time as a 90 kHz PTS
// value (for the WebVTT X-TIMESTAMP-MAP). MPEG-TS muxing adds a ~1.48s delay, so
// subtitles (extracted from 0) must be offset by it or they show up early.
//
// It first reads the container start_time; if that's missing/zero (seen on
// copied — not transcoded — streams, where the field can probe as empty right
// after muxing), it falls back to the first video packet's PTS, which carries
// the delay reliably. Returns 0 only if both probes fail.
func videoStartPTS(ctx context.Context, outDir string) int64 {
	seg := filepath.Join(outDir, "stream_video", "seg_000.ts")

	probe := func(args ...string) float64 {
		full := append([]string{"-v", "error", "-select_streams", "v:0"}, args...)
		full = append(full, "-of", "default=noprint_wrappers=1:nokey=1", seg)
		out, err := exec.CommandContext(ctx, "ffprobe", full...).Output()
		if err != nil {
			return 0
		}
		// A packet probe can emit several lines; take the first.
		first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
		v, _ := strconv.ParseFloat(strings.TrimSpace(first), 64)
		return v
	}

	st := probe("-show_entries", "stream=start_time")
	if st <= 0 {
		st = probe("-show_entries", "packet=pts_time", "-read_intervals", "%+#1")
	}
	return int64(st*90000 + 0.5)
}

// extractSubtitles converts every text subtitle to a WebVTT file + a one-segment
// HLS subtitle playlist, in a single ffmpeg pass. Image subtitles are skipped.
// tsOffset (90 kHz PTS) is written into each VTT's X-TIMESTAMP-MAP so cues line
// up with the MPEG-TS video timeline.
func extractSubtitles(ctx context.Context, src, outDir string, plan *Plan, durationSec float64, tsOffset int64) ([]SubtitleRendition, error) {
	var text []SubtitlePlan
	for _, s := range plan.SubtitleTracks {
		if s.Text {
			text = append(text, s)
		}
	}
	if len(text) == 0 {
		return nil, nil
	}

	subsDir := filepath.Join(outDir, "subs")
	if err := os.MkdirAll(subsDir, 0o755); err != nil {
		return nil, err
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", src}
	var out []SubtitleRendition
	for i, s := range text {
		vtt := "s" + strconv.Itoa(i) + ".vtt"
		args = append(args, "-map", "0:"+strconv.Itoa(s.SrcIndex),
			"-c:s", "webvtt", filepath.Join(subsDir, vtt))
		out = append(out, SubtitleRendition{
			TrackIndex: i,
			SrcIndex:   s.SrcIndex,
			Language:   s.Language,
			Title:      s.Title,
			Forced:     s.Forced,
			Path:       "subs/" + vtt,
		})
	}

	if err := runFFmpeg(ctx, args, 0, nil); err != nil {
		return nil, err
	}

	tsMap := fmt.Sprintf("X-TIMESTAMP-MAP=MPEGTS:%d,LOCAL:00:00:00.000", tsOffset)
	for i := range out {
		vtt := filepath.Join(subsDir, "s"+strconv.Itoa(i)+".vtt")
		// Offset the cues onto the TS timeline (insert the map after WEBVTT).
		data, err := os.ReadFile(vtt)
		if err != nil {
			return nil, err
		}
		body := strings.Replace(string(data), "WEBVTT", "WEBVTT\n"+tsMap, 1)
		if err := os.WriteFile(vtt, []byte(body), 0o644); err != nil {
			return nil, err
		}

		// Each subtitle is a single-segment VOD playlist pointing at its .vtt.
		pl := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n"+
			"#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:%.3f,\n%s\n#EXT-X-ENDLIST\n",
			int(math.Ceil(durationSec)), durationSec, filepath.Base(vtt))
		if err := os.WriteFile(filepath.Join(subsDir, "s"+strconv.Itoa(i)+".m3u8"),
			[]byte(pl), 0o644); err != nil {
			return nil, err
		}
	}
	return out, nil
}

var nameAttrRe = regexp.MustCompile(`NAME="[^"]*"`)

// rewriteMaster fixes the audio NAMEs ffmpeg emits (to friendly labels) and adds
// the subtitle renditions, wiring SUBTITLES="subs" onto the video stream so
// players show a subtitle menu.
func rewriteMaster(masterPath string, audio []AudioPlan, subs []SubtitleRendition) error {
	data, err := os.ReadFile(masterPath)
	if err != nil {
		return err
	}

	var b strings.Builder
	audioIdx := 0
	for _, ln := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(ln, "#EXT-X-MEDIA:TYPE=AUDIO"):
			if audioIdx < len(audio) {
				ln = nameAttrRe.ReplaceAllString(ln, `NAME="`+audioLabel(audio[audioIdx], audioIdx)+`"`)
			}
			audioIdx++
		case strings.HasPrefix(ln, "#EXT-X-STREAM-INF:"):
			if len(subs) > 0 {
				// Emit subtitle renditions just before the stream that uses them.
				for _, s := range subs {
					b.WriteString(subtitleMediaLine(s))
					b.WriteByte('\n')
				}
				ln += `,SUBTITLES="subs"`
			}
		}
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	// strings.Split + rejoin adds a trailing newline; trim the duplicate.
	return os.WriteFile(masterPath, []byte(strings.TrimRight(b.String(), "\n")+"\n"), 0o644)
}

func subtitleMediaLine(s SubtitleRendition) string {
	forced := "NO"
	if s.Forced {
		forced = "YES"
	}
	return fmt.Sprintf(
		`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="%s",LANGUAGE="%s",`+
			`DEFAULT=NO,AUTOSELECT=NO,FORCED=%s,URI="%s"`,
		subtitleLabel(s), s.Language, forced, "subs/s"+strconv.Itoa(s.TrackIndex)+".m3u8")
}

func subtitleLabel(s SubtitleRendition) string {
	label := s.Title
	if label == "" {
		label = s.Language
	}
	if label == "" {
		label = "Subtitles " + strconv.Itoa(s.TrackIndex+1)
	}
	if s.Forced && !strings.Contains(strings.ToLower(label), "forced") {
		label += " (forced)"
	}
	return strings.ReplaceAll(label, `"`, "")
}

// audioLabel builds a human label: the studio/title when present, otherwise the
// language, otherwise a numbered fallback.
func audioLabel(a AudioPlan, idx int) string {
	switch {
	case a.Title != "" && a.Language != "":
		return strings.ReplaceAll(a.Title+" · "+a.Language, `"`, "")
	case a.Title != "":
		return strings.ReplaceAll(a.Title, `"`, "")
	case a.Language != "":
		return a.Language
	default:
		return "Audio " + strconv.Itoa(idx+1)
	}
}
