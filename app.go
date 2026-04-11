package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dhowden/tag"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	audioServer   *http.Server
	audioServerMu sync.Mutex
	httpClient    *http.Client
	audioPort     string
	startTime     time.Time
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		startTime:  time.Now(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	elapsed := time.Since(a.startTime)
	log.Printf("[启动] Wails startup 完成 耗时: %s", elapsed)

	// 启动音频服务器
	a.StartAudioServer()
}

// activateWindow 激活并置顶窗口
func (a *App) activateWindow() {
	if a.ctx == nil {
		log.Printf("[单例] context 为空，无法激活窗口")
		return
	}

	// 使用 Wails runtime 激活窗口
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	log.Printf("[单例] 窗口已激活并置顶")

	// 延迟取消置顶，确保用户能看到窗口
	time.AfterFunc(800*time.Millisecond, func() {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
		log.Printf("[单例] 取消置顶")
	})
}

// LogMessage logs a message with timestamp from frontend
func (a *App) LogMessage(msg string) {
	elapsed := time.Since(a.startTime)
	log.Printf("[前端] %s (启动后 %s)", msg, elapsed)
}

// GetStartTime returns the program start time for calculating elapsed time
func (a *App) GetStartTime() int64 {
	return a.startTime.UnixMilli()
}

// SelectMusicFolder opens a native folder picker dialog
func (a *App) SelectMusicFolder() string {
	dirPath, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择音乐文件夹",
	})
	if err != nil {
		log.Printf("[SelectMusicFolder] 选择取消或失败: %v", err)
		return ""
	}
	log.Printf("[SelectMusicFolder] 选择的路径: %s", dirPath)
	return dirPath
}

// MusicFile represents a music or video file
type MusicFile struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Size           string `json:"size"`
	Quality        string `json:"quality"`
	Artist         string `json:"artist"`
	Title          string `json:"title"`
	LyricPath      string `json:"lyricPath"`
	BackgroundPath string `json:"backgroundPath"`
	CoverPath      string `json:"coverPath"`
	ThumbnailPath  string `json:"thumbnailPath"`
	IsVideo        bool   `json:"isVideo"`
}

// MediaFilesResult holds the file list and the detected mode
type MediaFilesResult struct {
	Files       []MusicFile `json:"files"`
	IsVideoMode bool        `json:"isVideoMode"`
}

// normalize removes spaces and special chars for fuzzy matching
func normalize(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "") // fullwidth space
	// Handle URL-encoded HTML entities commonly found in filenames
	s = strings.ReplaceAll(s, "_u0026", "&")
	s = strings.ReplaceAll(s, "_u003D", "=")
	s = strings.ReplaceAll(s, "_u003C", "<")
	s = strings.ReplaceAll(s, "_u003E", ">")
	s = strings.ReplaceAll(s, "_u0027", "'")
	s = strings.ReplaceAll(s, "_u0022", `"`)
	return s
}

// inferQuality infers audio quality from file extension and size
// 优先按格式判断，无损格式直接返回，其他根据大小估算（存在误差）
func inferQuality(ext string, size int64) string {
	// 无损格式直接判定（扩展名可靠）
	losslessExts := map[string]bool{".flac": true, ".ape": true, ".wav": true}
	if losslessExts[ext] {
		return "无损"
	}

	// 有损格式根据文件大小估算（存在误差，因为音质还取决于码率和时长）
	// 估算依据：假设平均歌曲时长4分钟，320kbps≈10MB，128kbps≈4MB
	sizeMB := size / (1024 * 1024)
	if sizeMB < 6 {
		return "流畅" // 可能是128kbps或更低
	}
	if sizeMB < 25 {
		return "高清" // 可能是192-320kbps
	}
	return "超清" // 可能是320kbps+或较长的高码率文件
}

// parseMusicName extracts artist and title from music file name
// Common format: "Artist - Title" or "Title - Artist"
func parseMusicName(fileName string) (artist string, title string) {
	// Remove extension
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Try longer separators first to avoid partial matches
	separators := []string{" - ", " — ", " – ", " -", "- ", "—", "–", "-"}
	for _, sep := range separators {
		parts := strings.SplitN(name, sep, 2)
		if len(parts) >= 2 {
			artist = strings.TrimSpace(parts[0])
			title = strings.TrimSpace(parts[1])
			if artist != "" && title != "" {
				return artist, title
			}
		}
	}

	return "", ""
}

// CheckMusicDir checks if the music directory exists
func (a *App) CheckMusicDir(musicDir string) bool {
	if musicDir == "" {
		return false
	}
	info, err := os.Stat(musicDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// buildLyricMap pre-scans lyricDir and returns a map of normalized music name -> lyric path
func buildLyricMap(lyricDir string) map[string]string {
	lyricMap := make(map[string]string)
	entries, err := os.ReadDir(lyricDir)
	if err != nil {
		return lyricMap
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".lrc") {
			continue
		}
		lrcBase := normalize(strings.TrimSuffix(entry.Name(), ".lrc"))
		lyricMap[lrcBase] = filepath.Join(lyricDir, entry.Name())
	}
	return lyricMap
}

// buildCoverMap pre-scans coverDir and returns maps for background and cover images
func buildCoverMap(coverDir string) (backgroundMap map[string]string, coverMap map[string]string) {
	backgroundMap = make(map[string]string)
	coverMap = make(map[string]string)
	// Create cover directory if not exists
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		return
	}
	entries, err := os.ReadDir(coverDir)
	if err != nil {
		return
	}
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !imageExts[ext] {
			continue
		}
		base := normalize(strings.TrimSuffix(entry.Name(), ext))
		if base == "background" {
			backgroundMap["_background_"] = filepath.Join(coverDir, entry.Name())
		} else {
			if _, ok := coverMap[base]; !ok {
				coverMap[base] = filepath.Join(coverDir, entry.Name())
			}
		}
	}
	return
}

// GetMusicFiles returns music files from the specified music directory
func (a *App) GetMusicFiles(musicDir string) []MusicFile {
	result := a.GetMediaFiles(musicDir)
	return result.Files
}

// GetMediaFiles returns all media files and determines the playback mode
// If video files > audio files -> Video Mode, otherwise -> Music Mode
func (a *App) GetMediaFiles(musicDir string) MediaFilesResult {
	var files []MusicFile
	var audioCount, videoCount int

	if musicDir == "" {
		return MediaFilesResult{Files: files, IsVideoMode: false}
	}

	// Pre-scan directories once
	lyricDir := filepath.Join(filepath.Dir(os.Args[0]), "lyrics")
	lyricMap := buildLyricMap(lyricDir)
	coverDir := filepath.Join(filepath.Dir(os.Args[0]), "cover")
	backgroundMap, coverMap := buildCoverMap(coverDir)

	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true,
		".m4a": true, ".ogg": true, ".aac": true,
		".wma": true, ".ape": true,
	}

	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true,
		".mov": true, ".wmv": true, ".webm": true,
	}

	entries, err := os.ReadDir(musicDir)
	if err != nil {
		return MediaFilesResult{Files: files, IsVideoMode: false}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		isVideo := videoExts[ext]
		isAudio := audioExts[ext]

		if !isAudio && !isVideo {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		sizeStr := formatSize(size)
		fileName := entry.Name()
		filePath := filepath.Join(musicDir, fileName)
		fileBase := normalize(strings.TrimSuffix(fileName, filepath.Ext(fileName)))

		// Lookup from pre-built maps
		lyricPath := ""
		backgroundPath := backgroundMap["_background_"]
		coverPath := coverMap[fileBase]

		if !isVideo {
			lyricPath = lyricMap[fileBase]
		}

		// Read metadata from audio/video file
		var artist, title string
		if f, err := os.Open(filePath); err == nil {
			meta, err := tag.ReadFrom(f)
			f.Close()
			if err == nil {
				artist = strings.TrimSpace(meta.Artist())
				title = strings.TrimSpace(meta.Title())
			}
		}
		// Fallback to parsed filename if metadata is empty
		if artist == "" || title == "" {
			aArtist, aTitle := parseMusicName(fileBase)
			if artist == "" {
				artist = aArtist
			}
			if title == "" {
				title = aTitle
			}
		}

		// 如果没匹配到封面，用歌名(title)模糊匹配
		if coverPath == "" && title != "" {
			coverPath = coverMap[normalize(title)]
		}

		quality := ""
		if isVideo {
			videoCount++
			// Video quality could be inferred from resolution but we'll leave it empty
		} else {
			audioCount++
			quality = inferQuality(ext, size)
		}

		// Get thumbnail for video files
		thumbnailPath := ""
		if isVideo {
			thumbnailPath = GetVideoThumbnail(filePath)
		}

		files = append(files, MusicFile{
			Name:           fileName,
			Path:           filePath,
			Size:           sizeStr,
			Quality:        quality,
			Artist:         artist,
			Title:          title,
			LyricPath:      lyricPath,
			BackgroundPath: backgroundPath,
			CoverPath:      coverPath,
			ThumbnailPath:  thumbnailPath,
			IsVideo:        isVideo,
		})
	}

	// Determine mode: Video Mode if video count > audio count
	isVideoMode := videoCount > audioCount

	return MediaFilesResult{Files: files, IsVideoMode: isVideoMode}
}

// LyricResult holds the lyric content and the path (if newly downloaded)
type LyricResult struct {
	Content   string `json:"content"`
	LyricPath string `json:"lyricPath"`
}

// GetLyric reads and returns the content of a lyric file, downloads from LRCIB if not found
func (a *App) GetLyric(lyricPath string, artist string, title string, musicPath string, musicDir string, duration float64) LyricResult {
	log.Printf("[GetLyric] lyricPath=%s, artist=%s, title=%s, musicPath=%s, duration=%.0f", lyricPath, artist, title, musicPath, duration)

	// If lyric file exists, read and return (local LRC file)
	if lyricPath != "" {
		data, err := os.ReadFile(lyricPath)
		if err == nil {
			log.Printf("[GetLyric] 使用本地歌词文件: %s", lyricPath)
			return LyricResult{Content: string(data), LyricPath: lyricPath}
		}
	}

	// Try embedded lyrics from audio file
	if musicPath != "" {
		if f, err := os.Open(musicPath); err == nil {
			meta, err := tag.ReadFrom(f)
			f.Close()
			if err == nil {
				lyrics := meta.Lyrics()
				// Check if embedded lyrics have timestamps (synced)
				if lyrics != "" && strings.Contains(lyrics, "[") {
					log.Printf("[GetLyric] 使用音频内嵌歌词，长度: %d 字符", len(lyrics))
					return LyricResult{Content: lyrics}
				}
			}
		}
	}

	// If no artist/title provided, try parsing from filename
	if artist == "" || title == "" {
		fileArtist, fileTitle := parseMusicName(filepath.Base(musicPath))
		if artist == "" {
			artist = fileArtist
		}
		if title == "" {
			title = fileTitle
		}
	}

	// Still no artist/title, skip download
	if artist == "" || title == "" {
		log.Printf("[GetLyric] 无法获取歌手/歌名，跳过下载")
		return LyricResult{}
	}

	log.Printf("[GetLyric] 下载歌词: artist=%s, title=%s", artist, title)

	// Try to download (first attempt: artist - title)
	downloadedPath := a.DownloadLyric(musicPath, artist, title, duration)
	if downloadedPath == "" {
		// Try swapping: title - artist (common in Chinese music)
		log.Printf("[GetLyric] 第一次尝试失败，尝试交换: title=%s, artist=%s", title, artist)
		downloadedPath = a.DownloadLyric(musicPath, title, artist, duration)
	}
	if downloadedPath == "" {
		return LyricResult{}
	}

	// Read and return the downloaded lyric
	data, err := os.ReadFile(downloadedPath)
	if err != nil {
		return LyricResult{}
	}
	log.Printf("[GetLyric] 下载歌词成功: %s", downloadedPath)
	return LyricResult{Content: string(data), LyricPath: downloadedPath}
}

// LRCIB API response
type LrcLibResponse struct {
	ID         int     `json:"id"`
	TrackName  string  `json:"trackName"`
	ArtistName string  `json:"artistName"`
	AlbumName  string  `json:"albumName"`
	Duration   float64 `json:"duration"`
	Lyrics     string  `json:"syncedLyrics"`
}

// fetchLyricFromLRCIB downloads synced lyrics from LRCIB by artist and title
func fetchLyricFromLRCIB(client *http.Client, artist, title string, duration float64) string {
	url := fmt.Sprintf("https://lrclib.net/api/get?artist_name=%s&track_name=%s&duration=%.0f",
		url.QueryEscape(artist),
		url.QueryEscape(title),
		duration)

	log.Printf("[歌词下载] 正在查询 LRCIB: %s - %s", artist, title)
	log.Printf("[歌词下载] URL: %s", url)

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[歌词下载] 请求失败: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[歌词下载] LRCIB 返回状态码: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[歌词下载] 读取响应失败: %v", err)
		return ""
	}

	var result LrcLibResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[歌词下载] 解析 JSON 失败: %v, body: %s", err, string(body))
		return ""
	}

	// 必须有同步歌词（时间戳），普通歌词不要
	if result.Lyrics == "" {
		log.Printf("[歌词下载] LRCIB 未找到有时间戳的歌词: %s - %s, body: %s", artist, title, string(body))
		return ""
	}

	log.Printf("[歌词下载] 成功获取歌词: %s - %s", artist, title)
	return result.Lyrics
}

// DownloadLyric downloads lyric from LRCIB and saves to lyricsDir
func (a *App) DownloadLyric(musicPath string, artist string, title string, duration float64) string {
	log.Printf("[歌词下载] 开始下载歌词: %s - %s, 时长: %.0f秒", artist, title, duration)

	lyricsDir := filepath.Join(filepath.Dir(os.Args[0]), "lyrics")
	log.Printf("[歌词下载] 歌词目录: %s", lyricsDir)
	if err := os.MkdirAll(lyricsDir, 0755); err != nil {
		log.Printf("[歌词下载] 创建目录失败: %v", err)
		return ""
	}

	lyricContent := fetchLyricFromLRCIB(a.httpClient, artist, title, duration)
	log.Printf("[歌词下载] 获取到的歌词长度: %d 字符", len(lyricContent))
	if lyricContent == "" {
		log.Printf("[歌词下载] 获取歌词内容为空: %s - %s", artist, title)
		return ""
	}

	// Use the same filename as the music file (just change extension to .lrc)
	musicName := strings.TrimSuffix(filepath.Base(musicPath), filepath.Ext(musicPath))
	lyricPath := filepath.Join(lyricsDir, musicName+".lrc")
	log.Printf("[歌词下载] 保存路径: %s", lyricPath)

	if err := os.WriteFile(lyricPath, []byte(lyricContent), 0644); err != nil {
		log.Printf("[歌词下载] 保存文件失败: %v", err)
		return ""
	}

	log.Printf("[歌词下载] 歌词已保存: %s", lyricPath)
	return lyricPath
}

func formatSize(bytes int64) string {
	const KB = 1024
	const MB = KB * 1024
	const GB = MB * 1024
	if bytes >= GB {
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	}
	if bytes >= MB {
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	}
	return fmt.Sprintf("%dKB", bytes/KB)
}

// StartAudioServer starts a local HTTP server for audio streaming on port 19890
func (a *App) StartAudioServer() string {
	a.audioServerMu.Lock()
	defer a.audioServerMu.Unlock()

	// 如果服务器已启动，不再重复启动
	if a.audioServer != nil {
		return fmt.Sprintf("http://localhost:%s/stream", a.audioPort)
	}

	mux := http.NewServeMux()

	// Middleware: add CORS headers to all responses and handle preflight
	corsMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})

	// Use port 0 to let OS assign an available port
	server := &http.Server{Handler: corsMux}

	// Get the actual port after server starts
	type result struct {
		addr string
		err  error
	}
	resultCh := make(chan result, 1)

	// serveMediaFile serves a file with proper headers; enableCache adds ETag/Cache-Control
	serveMediaFile := func(w http.ResponseWriter, r *http.Request, contentType string, enableCache bool) bool {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "Missing path", 400)
			return false
		}
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "File not found: "+err.Error(), 404)
			return false
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			http.Error(w, "File stat error: "+err.Error(), 500)
			return false
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		if enableCache {
			w.Header().Set("Accept-Ranges", "bytes")
			etag := fmt.Sprintf(`"%x"`, stat.ModTime().UnixNano())
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "private, max-age=3600")
			if cachedETag := r.Header.Get("If-None-Match"); cachedETag != "" && cachedETag == etag {
				w.WriteHeader(http.StatusNotModified)
				return false
			}
		}
		io.Copy(w, file)
		return true
	}

	audioContentType := func(ext string) string {
		switch ext {
		case ".wav":
			return "audio/wav"
		case ".flac":
			return "audio/flac"
		case ".m4a":
			return "audio/mp4"
		case ".ogg":
			return "audio/ogg"
		case ".aac":
			return "audio/aac"
		}
		return "audio/mpeg"
	}

	// Handler for streaming audio file
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(filepath.Ext(r.URL.Query().Get("path")))
		serveMediaFile(w, r, audioContentType(ext), true)
	})

	// Handler for audio data (returns raw bytes for Web Audio API)
	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(filepath.Ext(r.URL.Query().Get("path")))
		serveMediaFile(w, r, audioContentType(ext), false)
	})

	// Handler for serving image files
	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "Missing path", 400)
			return
		}

		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "File not found: "+err.Error(), 404)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			http.Error(w, "File stat error: "+err.Error(), 500)
			return
		}
		ext := strings.ToLower(filepath.Ext(filePath))
		contentType := "image/jpeg"
		switch ext {
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		case ".bmp":
			contentType = "image/bmp"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// HTTP缓存
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, stat.ModTime().UnixNano()))
		w.Header().Set("Cache-Control", "private, max-age=86400") // 1 day for images

		// 检查是否有缓存的 ETag
		if cachedETag := r.Header.Get("If-None-Match"); cachedETag != "" {
			if cachedETag == fmt.Sprintf(`"%x"`, stat.ModTime().UnixNano()) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		io.Copy(w, file)
	})

	// Handler for single-instance activation
	mux.HandleFunc("/activate_1964e24cbef57e7ca32f80238fd3320c", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[单例] 收到激活信号")
		a.activateWindow()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Handler for serving background image from theme directory
	mux.HandleFunc("/theme/bg", func(w http.ResponseWriter, r *http.Request) {
		themeDir := getThemeDir()
		bgImagePath := filepath.Join(themeDir, "background.png")
		_ = r.URL.Query().Get("t") // consume the timestamp query param to avoid cache

		file, err := os.Open(bgImagePath)
		if err != nil {
			http.Error(w, "Background image not found", 404)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			http.Error(w, "File stat error: "+err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "private, max-age=86400")

		io.Copy(w, file)
	})

	go func() {
		ln, err := net.Listen("tcp", "localhost:19890")
		if err != nil {
			resultCh <- result{err: err}
			return
		}
		resultCh <- result{addr: ln.Addr().String()}
		server.Serve(ln)
	}()

	r := <-resultCh
	if r.err != nil {
		log.Printf("[音频服务器] 启动失败: %v", r.err)
		return ""
	}

	a.audioServer = server
	a.audioPort = "19890"
	log.Printf("[音频服务器] 已启动 http://localhost:19890")
	return "http://localhost:19890/stream"
}

// StopAudioServer stops the audio HTTP server
func (a *App) StopAudioServer() {
	a.audioServerMu.Lock()
	defer a.audioServerMu.Unlock()
	if a.audioServer != nil {
		a.audioServer.Shutdown(context.Background())
		a.audioServer = nil
	}
}

// getThumbnailDir returns the thumbnail cache directory
func getThumbnailDir() string {
	return filepath.Join(filepath.Dir(os.Args[0]), "thumbnails")
}

// GetVideoThumbnail generates or retrieves a cached thumbnail for a video file
// Returns the thumbnail file path, or empty string if generation failed
func GetVideoThumbnail(videoPath string) string {
	if videoPath == "" {
		return ""
	}

	// Get thumbnail directory
	thumbDir := getThumbnailDir()

	// Create thumbnails directory if not exists
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		log.Printf("[Thumbnail] 创建缩略图目录失败: %v", err)
		return ""
	}

	// Generate thumbnail filename based on video file name (not full path to avoid exposing paths)
	videoBase := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	thumbnailName := videoBase + "_thumb.jpg"
	thumbnailPath := filepath.Join(thumbDir, thumbnailName)

	// Check if thumbnail already exists
	if _, err := os.Stat(thumbnailPath); err == nil {
		log.Printf("[Thumbnail] 使用缓存封面: %s", thumbnailPath)
		return thumbnailPath
	}

	// Generate thumbnail using ffmpeg
	log.Printf("[Thumbnail] 正在生成封面: %s", videoPath)
	err := generateVideoThumbnail(videoPath, thumbnailPath)
	if err != nil {
		log.Printf("[Thumbnail] 生成封面失败: %v", err)
		return ""
	}

	log.Printf("[Thumbnail] 封面已生成: %s", thumbnailPath)
	return thumbnailPath
}

// generateVideoThumbnail uses ffmpeg to extract a frame from the video
func generateVideoThumbnail(videoPath string, outputPath string) error {
	// Try to extract a frame at 1 second (or at 5% of duration if video is short)
	// -ss before -i is faster (seek before input)
	// -vframes 1: extract single frame
	// -q:v 2: quality (lower is better, 2-5 is good range)
	// -y: overwrite output file
	// -2 ensures dimensions divisible by 2 (required by some codecs)
	// scale to 1920 width max, keeping aspect ratio
	// 提取首帧
	cmd := []string{
		"ffmpeg",
		"-y",
		"-ss", "00:00:00",
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale='min(1920,iw)':-2",
		outputPath,
	}

	// Try the command
	err := runCommand(cmd)
	if err == nil {
		return nil
	}

	// If first attempt failed, try without timestamp (for very short videos)
	log.Printf("[Thumbnail] 第一次尝试失败，尝试0秒位置: %v", err)
	cmd[3] = "00:00:00" // change -ss position
	err = runCommand(cmd)
	return err
}

// runCommand executes a shell command and returns the error
func runCommand(cmd []string) error {
	// Simple command execution using exec.LookPath and exec.Command
	ffmpegPath := cmd[0]
	if ffmpegPath == "ffmpeg" {
		// Find ffmpeg in PATH
		if path, ok := os.LookupEnv("PATH"); ok {
			for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
				p := filepath.Join(dir, "ffmpeg")
				if _, err := os.Stat(p); err == nil {
					ffmpegPath = p
					break
				}
				// Try .exe suffix on Windows
				p += ".exe"
				if _, err := os.Stat(p); err == nil {
					ffmpegPath = p
					break
				}
			}
		}
	}
	cmd[0] = ffmpegPath

	// Use exec.Command to run
	var execCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// On Windows, run through cmd.exe without showing window
		execCmd = exec.Command("cmd", "/c", strings.Join(cmd, " "))
		execCmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	} else {
		execCmd = exec.Command(cmd[0], cmd[1:]...)
	}
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.Printf("[Thumbnail] ffmpeg 执行失败: %v, output: %s", err, string(output))
		return err
	}
	return nil
}

// ClearThumbnails removes all cached thumbnail images
func (a *App) ClearThumbnails() error {
	thumbDir := getThumbnailDir()
	entries, err := os.ReadDir(thumbDir)
	if err != nil {
		// Directory doesn't exist, nothing to clear
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(thumbDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			log.Printf("[ClearThumbnails] 删除缩略图失败: %v", err)
			return err
		}
	}
	log.Printf("[ClearThumbnails] 缩略图缓存已清理")
	return nil
}

// getThemeDir returns the theme directory path (exe所在目录/theme)
func getThemeDir() string {
	return filepath.Join(filepath.Dir(os.Args[0]), "theme")
}

// SaveBgImage saves a background image (base64 data URL) to the theme directory
// Returns true if background image exists after saving
func (a *App) SaveBgImage(base64Data string) (bool, error) {
	if base64Data == "" {
		return false, nil
	}

	// Decode base64 data URL
	dataStr := base64Data
	if strings.HasPrefix(base64Data, "data:") {
		commaIdx := strings.Index(base64Data, ",")
		if commaIdx > 0 {
			dataStr = base64Data[commaIdx+1:]
		}
	}

	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		log.Printf("[SaveBgImage] 解码 base64 失败: %v", err)
		return false, fmt.Errorf("解码图片数据失败: %v", err)
	}

	// Create theme directory if not exists
	themeDir := getThemeDir()
	if err := os.MkdirAll(themeDir, 0755); err != nil {
		log.Printf("[SaveBgImage] 创建 theme 目录失败: %v", err)
		return false, fmt.Errorf("创建目录失败: %v", err)
	}

	// Save to background.png in theme directory
	bgImagePath := filepath.Join(themeDir, "background.png")
	if err := os.WriteFile(bgImagePath, data, 0644); err != nil {
		log.Printf("[SaveBgImage] 保存图片失败: %v", err)
		return false, fmt.Errorf("保存图片失败: %v", err)
	}

	log.Printf("[SaveBgImage] 背景图片已保存: %s", bgImagePath)
	return true, nil
}

// GetBgImage returns true if background image exists in theme directory
func (a *App) GetBgImage() (bool, error) {
	themeDir := getThemeDir()
	bgImagePath := filepath.Join(themeDir, "background.png")

	if _, err := os.Stat(bgImagePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// SettingsResult holds all color settings
type SettingsResult struct {
	BgColor       string `json:"bgColor"`
	BtnColor      string `json:"btnColor"`
	VizColor      string `json:"vizColor"`
	LyricColor    string `json:"lyricColor"`
	TitleColor    string `json:"titleColor"`
	TitlebarColor string `json:"titlebarColor"`
	LyricsColor   string `json:"lyricsColor"`
}

// SaveSettings saves all color settings to a JSON file in theme directory
func (a *App) SaveSettings(settings string) error {
	themeDir := getThemeDir()
	if err := os.MkdirAll(themeDir, 0755); err != nil {
		log.Printf("[SaveSettings] 创建 theme 目录失败: %v", err)
		return fmt.Errorf("创建目录失败: %v", err)
	}

	settingsPath := filepath.Join(themeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(settings), 0644); err != nil {
		log.Printf("[SaveSettings] 保存设置失败: %v", err)
		return fmt.Errorf("保存设置失败: %v", err)
	}

	log.Printf("[SaveSettings] 设置已保存: %s", settingsPath)
	return nil
}

// GetSettings reads all color settings from the JSON file in theme directory
func (a *App) GetSettings() (string, error) {
	themeDir := getThemeDir()
	settingsPath := filepath.Join(themeDir, "settings.json")

	if _, err := os.Stat(settingsPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", err
	}

	// Validate it's valid JSON
	var settings SettingsResult
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("[GetSettings] 解析设置文件失败: %v", err)
		return "", nil
	}

	return string(data), nil
}

// ClearThemeFiles removes background image and settings file from theme directory
func (a *App) ClearThemeFiles() error {
	themeDir := getThemeDir()

	// Remove background.png
	bgImagePath := filepath.Join(themeDir, "background.png")
	if _, err := os.Stat(bgImagePath); err == nil {
		if err := os.Remove(bgImagePath); err != nil {
			log.Printf("[ClearThemeFiles] 删除背景图片失败: %v", err)
		}
	}

	// Remove settings.json
	settingsPath := filepath.Join(themeDir, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		if err := os.Remove(settingsPath); err != nil {
			log.Printf("[ClearThemeFiles] 删除设置文件失败: %v", err)
		}
	}

	log.Printf("[ClearThemeFiles] 主题文件已清理")
	return nil
}

// NetEaseSongDetail 网易云歌曲详情响应
type NetEaseSongDetail struct {
	Songs []struct {
		Album struct {
			PicURL string `json:"picUrl"`
		} `json:"album"`
	} `json:"songs"`
}

// NetEaseSearchResult 网易云搜索响应
type NetEaseSearchResult struct {
	Result struct {
		Songs []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Artist string `json:"artist"`
		} `json:"songs"`
	} `json:"result"`
}

// FetchCoverFromNetEase 从网易云获取歌曲封面并保存到 cover 目录
// 返回保存后的封面路径，如果失败返回空字符串
func (a *App) FetchCoverFromNetEase(artist string, title string, fileBase string) string {
	if artist == "" && title == "" {
		log.Printf("[FetchCover] 歌手和歌名都为空，跳过")
		return ""
	}

	coverDir := filepath.Join(filepath.Dir(os.Args[0]), "cover")
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		log.Printf("[FetchCover] 创建 cover 目录失败: %v", err)
		return ""
	}

	// 构造搜索关键词：优先用 "歌手 歌名" 格式
	searchKeyword := title
	if artist != "" {
		searchKeyword = artist + " " + title
	}

	// 搜索歌曲 ID
	searchURL := fmt.Sprintf("http://music.163.com/api/search/get?s=%s&type=1&limit=100", url.QueryEscape(searchKeyword))
	log.Printf("[FetchCover] 搜索URL: %s", searchURL)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		log.Printf("[FetchCover] 创建搜索请求失败: %v", err)
		return ""
	}
	req.Header.Set("Referer", "http://music.163.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Printf("[FetchCover] 搜索请求失败: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[FetchCover] 搜索返回状态码: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[FetchCover] 读取搜索响应失败: %v", err)
		return ""
	}

	var searchResult NetEaseSearchResult
	if err := json.Unmarshal(body, &searchResult); err != nil {
		log.Printf("[FetchCover] 解析搜索响应失败: %v", err)
		return ""
	}

	if len(searchResult.Result.Songs) == 0 {
		log.Printf("[FetchCover] 未找到歌曲: %s", searchKeyword)
		return ""
	}

	// 找到匹配的歌曲 ID
	songID := searchResult.Result.Songs[0].ID
	log.Printf("[FetchCover] 找到歌曲 ID: %d, 名称: %s", songID, searchResult.Result.Songs[0].Name)

	// 获取歌曲详情（包含封面 URL）
	detailURL := fmt.Sprintf("https://music.163.com/api/song/detail/?id=%d&ids=[%d]", songID, songID)
	log.Printf("[FetchCover] 详情URL: %s", detailURL)

	req2, err := http.NewRequest("GET", detailURL, nil)
	if err != nil {
		log.Printf("[FetchCover] 创建详情请求失败: %v", err)
		return ""
	}
	req2.Header.Set("Referer", "http://music.163.com/")
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp2, err := a.httpClient.Do(req2)
	if err != nil {
		log.Printf("[FetchCover] 详情请求失败: %v", err)
		return ""
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		log.Printf("[FetchCover] 详情返回状态码: %d", resp2.StatusCode)
		return ""
	}

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		log.Printf("[FetchCover] 读取详情响应失败: %v", err)
		return ""
	}

	var detailResult NetEaseSongDetail
	if err := json.Unmarshal(body2, &detailResult); err != nil {
		log.Printf("[FetchCover] 解析详情响应失败: %v", err)
		return ""
	}

	if len(detailResult.Songs) == 0 {
		log.Printf("[FetchCover] 未找到歌曲详情")
		return ""
	}

	picURL := detailResult.Songs[0].Album.PicURL
	if picURL == "" {
		log.Printf("[FetchCover] 歌曲无封面图")
		return ""
	}
	log.Printf("[FetchCover] 封面URL: %s", picURL)

	// 下载封面图片
	req3, err := http.NewRequest("GET", picURL, nil)
	if err != nil {
		log.Printf("[FetchCover] 创建图片请求失败: %v", err)
		return ""
	}
	req3.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp3, err := a.httpClient.Do(req3)
	if err != nil {
		log.Printf("[FetchCover] 下载封面失败: %v", err)
		return ""
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != 200 {
		log.Printf("[FetchCover] 下载封面返回状态码: %d", resp3.StatusCode)
		return ""
	}

	imageData, err := io.ReadAll(resp3.Body)
	if err != nil {
		log.Printf("[FetchCover] 读取封面数据失败: %v", err)
		return ""
	}

	// 保存封面图片，文件名使用 fileBase（原始文件名规范化后的名称）
	saveFileName := fileBase + ".jpg"
	savePath := filepath.Join(coverDir, saveFileName)

	if err := os.WriteFile(savePath, imageData, 0644); err != nil {
		log.Printf("[FetchCover] 保存封面失败: %v", err)
		return ""
	}

	log.Printf("[FetchCover] 封面已保存: %s", savePath)
	return savePath
}

// GetCover returns the path to the cover image for a given artist and title
// If no local cover is found, it downloads from NetEase and saves to cover directory
func (a *App) GetCover(artist string, title string) string {
	if artist == "" && title == "" {
		return ""
	}

	coverDir := filepath.Join(filepath.Dir(os.Args[0]), "cover")

	// Try to find existing cover file
	keyword := title
	if artist != "" {
		keyword = artist + " " + title
	}

	entries, err := os.ReadDir(coverDir)
	if err == nil {
		normalizedKeyword := normalize(keyword)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" && ext != ".bmp" {
				continue
			}
			base := normalize(strings.TrimSuffix(entry.Name(), ext))
			if base == normalizedKeyword || normalize(base) == normalizedKeyword {
				return filepath.Join(coverDir, entry.Name())
			}
		}
	}

	// No local cover found, download from NetEase
	return a.FetchCoverFromNetEase(artist, title, normalize(keyword))
}
