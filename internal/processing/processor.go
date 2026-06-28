package processing

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"watchparty/internal/db"
	"watchparty/internal/media"
)

// Processor turns source files into browser-ready MP4s. Work runs in the
// background; status is tracked in the DB so the UI can poll it.
type Processor struct {
	pool          *pgxpool.Pool
	processedRoot string // container path where output MP4s are written

	mu       sync.Mutex
	inflight map[int]bool // video IDs currently being processed
}

func New(pool *pgxpool.Pool, processedRoot string) *Processor {
	return &Processor{
		pool:          pool,
		processedRoot: processedRoot,
		inflight:      make(map[int]bool),
	}
}

// Enqueue starts processing a video in the background. targetHeight caps the
// H.264 transcode resolution (ignored when the source is copied). It is a no-op
// if the video is already being processed. Returns false if already inflight.
func (p *Processor) Enqueue(videoID, targetHeight int) bool {
	p.mu.Lock()
	if p.inflight[videoID] {
		p.mu.Unlock()
		return false
	}
	p.inflight[videoID] = true
	p.mu.Unlock()

	go func() {
		defer func() {
			p.mu.Lock()
			delete(p.inflight, videoID)
			p.mu.Unlock()
		}()
		if err := p.run(videoID, targetHeight); err != nil {
			log.Printf("processing video %d failed: %v", videoID, err)
		}
	}()
	return true
}

func (p *Processor) run(videoID, targetHeight int) error {
	// Generous timeout: a full re-encode of a long film can take a while.
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	video, err := db.GetVideo(ctx, p.pool, videoID)
	if err != nil {
		return fmt.Errorf("load video: %w", err)
	}

	_ = db.SetStatus(ctx, p.pool, videoID, "processing", nil)
	_ = db.SetProgress(ctx, p.pool, videoID, 0)

	fail := func(err error) error {
		msg := err.Error()
		_ = db.SetStatus(ctx, p.pool, videoID, "error", &msg)
		return err
	}

	probe, err := media.Probe(ctx, video.FilePath)
	if err != nil {
		return fail(fmt.Errorf("probe: %w", err))
	}

	plan, err := media.BuildPlan(probe)
	if err != nil {
		return fail(fmt.Errorf("plan: %w", err))
	}

	duration := parseDuration(probe.Format.Duration)

	// Skip-if-ready: the source is already browser-playable (MP4 + H.264 +
	// browser-safe audio), so serve it directly — no remux, no extra disk.
	if probe.IsBrowserReady() {
		if err := p.saveAudioTracks(ctx, videoID, plan); err != nil {
			return fail(fmt.Errorf("save tracks: %w", err))
		}
		if err := db.SetProcessed(ctx, p.pool, videoID, video.FilePath, "mp4",
			plan.VideoCodec, plan.SourceWidth, plan.SourceHeight, plan.HDR, duration); err != nil {
			return fail(fmt.Errorf("mark ready: %w", err))
		}
		log.Printf("video %d: ready (direct play, no processing)", videoID)
		return nil
	}

	// Otherwise package to HLS: video copied, each audio track its own AAC
	// rendition, so every viewer can pick their own track/subs client-side.
	outDir := filepath.Join(p.processedRoot, strconv.Itoa(videoID))
	masterPath := filepath.Join(outDir, "master.m3u8")
	log.Printf("video %d: HLS codec=%s height=%d hdr=%v audio=%d dur=%.0fs -> %s",
		videoID, plan.VideoCodec, targetHeight, plan.HDR, len(plan.AudioTracks), duration, outDir)

	// Persist progress only when the whole-percent value changes, to avoid
	// hammering the DB with sub-percent updates (~1 write/sec at most).
	lastPct := -1
	onProgress := func(pct float64) {
		if whole := int(pct); whole != lastPct {
			lastPct = whole
			_ = db.SetProgress(ctx, p.pool, videoID, pct)
		}
	}

	subs, err := media.PackageHLS(ctx, video.FilePath, outDir, plan, targetHeight, duration, onProgress)
	if err != nil {
		return fail(fmt.Errorf("package hls: %w", err))
	}

	if err := p.saveAudioTracks(ctx, videoID, plan); err != nil {
		return fail(fmt.Errorf("save tracks: %w", err))
	}
	if err := p.saveSubtitleTracks(ctx, videoID, subs); err != nil {
		return fail(fmt.Errorf("save subtitles: %w", err))
	}

	if err := db.SetProcessed(ctx, p.pool, videoID, masterPath, "hls",
		plan.VideoCodec, plan.SourceWidth, plan.SourceHeight, plan.HDR, duration); err != nil {
		return fail(fmt.Errorf("mark ready: %w", err))
	}

	log.Printf("video %d: ready", videoID)
	return nil
}

// saveAudioTracks persists audio track metadata for the selection UI (Step 4).
func (p *Processor) saveAudioTracks(ctx context.Context, videoID int, plan *media.Plan) error {
	tracks := make([]db.AudioTrack, len(plan.AudioTracks))
	for i, a := range plan.AudioTracks {
		tracks[i] = db.AudioTrack{
			VideoID:    videoID,
			TrackIndex: i,
			SrcIndex:   a.SrcIndex,
			Codec:      strPtr(a.Codec),
			Language:   strPtr(a.Language),
			Title:      strPtr(a.Title),
			Channels:   intPtr(a.Channels),
		}
	}
	return db.ReplaceAudioTracks(ctx, p.pool, videoID, tracks)
}

// saveSubtitleTracks persists the WebVTT subtitle renditions for the player.
func (p *Processor) saveSubtitleTracks(ctx context.Context, videoID int, subs []media.SubtitleRendition) error {
	tracks := make([]db.SubtitleTrack, len(subs))
	for i, s := range subs {
		path := s.Path
		tracks[i] = db.SubtitleTrack{
			VideoID:    videoID,
			TrackIndex: s.TrackIndex,
			SrcIndex:   s.SrcIndex,
			Language:   strPtr(s.Language),
			Title:      strPtr(s.Title),
			Forced:     s.Forced,
			Path:       &path,
		}
	}
	return db.ReplaceSubtitleTracks(ctx, p.pool, videoID, tracks)
}

func parseDuration(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
