package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	audioServer   *http.Server
	audioServerMu sync.Mutex
	httpClient    *http.Client
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectMusicFolder opens a native folder picker dialog
func (a *App) SelectMusicFolder() string {
	dirPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择音乐文件夹",
	})
	if err != nil {
		log.Printf("[SelectMusicFolder] 选择取消或失败: %v", err)
		return ""
	}
	log.Printf("[SelectMusicFolder] 选择的路径: %s", dirPath)
	return dirPath
}

// MusicFile represents a music file
type MusicFile struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Size           string `json:"size"`
	Artist         string `json:"artist"`
	Title          string `json:"title"`
	LyricPath      string `json:"lyricPath"`
	BackgroundPath string `json:"backgroundPath"`
	CoverPath      string `json:"coverPath"`
}

// normalize removes spaces and special chars for fuzzy matching
func normalize(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_u0026", "&")
	s = strings.ReplaceAll(s, "　", "") // fullwidth space
	return s
}

// parseMusicName extracts artist and title from music file name
// Common format: "Artist - Title" or "Title - Artist"
func parseMusicName(fileName string) (artist string, title string) {
	// Remove extension
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Try multiple separators
	separators := []string{" - ", " -", "- ", " — ", "—", " – ", " –", "-"}
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
	var files []MusicFile
	if musicDir == "" {
		return files
	}

	// Pre-scan directories once
	lyricDir := filepath.Join(filepath.Dir(os.Args[0]), "lyrics")
	lyricMap := buildLyricMap(lyricDir)
	coverDir := filepath.Join(musicDir, "cover")
	backgroundMap, coverMap := buildCoverMap(coverDir)

	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true,
		".m4a": true, ".ogg": true, ".aac": true,
		".wma": true, ".ape": true,
	}

	entries, err := os.ReadDir(musicDir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if audioExts[ext] {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			size := info.Size()
			sizeStr := formatSize(size)
			musicName := entry.Name()
			musicPath := filepath.Join(musicDir, musicName)
			musicBase := normalize(strings.TrimSuffix(musicName, filepath.Ext(musicName)))

			// Lookup from pre-built maps
			lyricPath := lyricMap[musicBase]
			backgroundPath := backgroundMap["_background_"]
			coverPath := coverMap[musicBase]

			// Read metadata from audio file
			var artist, title string
			if f, err := os.Open(musicPath); err == nil {
				meta, err := tag.ReadFrom(f)
				f.Close()
				if err == nil {
					artist = strings.TrimSpace(meta.Artist())
					title = strings.TrimSpace(meta.Title())
				}
			}
			// Fallback to parsed filename if metadata is empty
			if artist == "" || title == "" {
				aArtist, aTitle := parseMusicName(musicBase)
				if artist == "" {
					artist = aArtist
				}
				if title == "" {
					title = aTitle
				}
			}

			files = append(files, MusicFile{
				Name:           musicName,
				Path:           musicPath,
				Size:           sizeStr,
				Artist:         artist,
				Title:          title,
				LyricPath:      lyricPath,
				BackgroundPath: backgroundPath,
				CoverPath:      coverPath,
			})
		}
	}
	return files
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

// StartAudioServer starts a local HTTP server for audio streaming on an available port
func (a *App) StartAudioServer() string {
	a.audioServerMu.Lock()
	defer a.audioServerMu.Unlock()

	if a.audioServer != nil {
		a.audioServer.Shutdown(context.Background())
		a.audioServer = nil
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

	// Handler for streaming audio file
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
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
		contentType := "audio/mpeg"
		switch ext {
		case ".wav":
			contentType = "audio/wav"
		case ".flac":
			contentType = "audio/flac"
		case ".m4a":
			contentType = "audio/mp4"
		case ".ogg":
			contentType = "audio/ogg"
		case ".aac":
			contentType = "audio/aac"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		// HTTP缓存：使用 ETag 和 Cache-Control
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, stat.ModTime().UnixNano()))
		w.Header().Set("Cache-Control", "private, max-age=3600")

		// 检查是否有缓存的 ETag
		if cachedETag := r.Header.Get("If-None-Match"); cachedETag != "" {
			if cachedETag == fmt.Sprintf(`"%x"`, stat.ModTime().UnixNano()) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		io.Copy(w, file)
	})

	// Handler for audio data (returns raw bytes for Web Audio API)
	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
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
		contentType := "audio/mpeg"
		switch ext {
		case ".wav":
			contentType = "audio/wav"
		case ".flac":
			contentType = "audio/flac"
		case ".m4a":
			contentType = "audio/mp4"
		case ".ogg":
			contentType = "audio/ogg"
		case ".aac":
			contentType = "audio/aac"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

		io.Copy(w, file)
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

	go func() {
		ln, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			resultCh <- result{err: err}
			return
		}
		resultCh <- result{addr: ln.Addr().String()}
		server.Serve(ln)
	}()

	r := <-resultCh
	if r.err != nil {
		return ""
	}
	addr := r.addr // e.g. "127.0.0.1:51234"
	// Extract port from addr
	colonIdx := strings.LastIndex(addr, ":")
	portStr := addr[colonIdx+1:]

	a.audioServer = server
	return fmt.Sprintf("http://localhost:%s/stream", portStr)
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
