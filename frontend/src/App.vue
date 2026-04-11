<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { GetMediaFiles, StopAudioServer, GetLyric, SelectMusicFolder, ClearThumbnails, GetCover, LogMessage } from '../wailsjs/go/main/App'
import { WindowMinimise, Quit, WindowSetSize, WindowSetPosition, WindowGetPosition, WindowSetAlwaysOnTop, WindowIsMinimised } from '../wailsjs/runtime/runtime'
import PlaylistPanel from './components/PlaylistPanel.vue'
import MenuPanel from './components/MenuPanel.vue'
import VideoPlayer from './components/VideoPlayer.vue'
import { useTheme } from './composables/useTheme'
import { useVisualizer } from './composables/useVisualizer'
import { usePlaylist, type MediaFile } from './composables/usePlaylist'
import { useAudio } from './composables/useAudio'
import {
  getAlwaysOnTop, setAlwaysOnTopState,
  getMusicDir, setMusicDir,
  getPlayState, setPlayState,
  getVideoResolutions, setVideoResolution,
  getCloseDB
} from './composables/useStorage'

// ============== Composables ==============
const theme = useTheme()
const visualizer = useVisualizer()
const playlistState = usePlaylist()

// ============== Audio Player ==============
const audioPlayer = useAudio({
  onEnded: () => {
    handleTrackEnded()
  },
  onPlay: () => {
    visualizer.startVisualizer()
    saveCurrentPlayState()
  },
  onPause: () => {
    visualizer.stopVisualizer()
    saveCurrentPlayState()
  },
  onError: (msg) => {
    showToastMsg('音频加载失败，请尝试播放其他歌曲', 'error')
  },
  onLoadedMetadata: () => {
    const track = playlistState.playlist.value[playlistState.currentIndex.value]
    if (track && !track.isVideo) {
      loadLyric(track.lyricPath || '', track.artist || '', track.title || '', track.path, musicDir.value, audioPlayer.duration.value)
    }
  }
})

// ============== UI State ==============
const showPlaylist = ref(false)
const showVolume = ref(false)
const showMenu = ref(false)
const showSetupPrompt = ref(false)

// Toast
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const showToast = ref(false)
let toastTimer: ReturnType<typeof setTimeout> | null = null

function showToastMsg(msg: string, type: 'success' | 'error' = 'success') {
  if (toastTimer !== null) {
    clearTimeout(toastTimer)
  }
  toastMessage.value = msg
  toastType.value = type
  showToast.value = true
  toastTimer = setTimeout(() => {
    showToast.value = false
  }, 3000)
}

// Lyric
const lyricContent = ref('')
const lyricLines = ref<{ time: number; text: string }[]>([])
const embeddedLyricPaths = ref(new Set<string>())

// Current lyric index (binary search)
const currentLyricIndex = computed(() => {
  if (lyricLines.value.length === 0) return -1
  const t = audioPlayer.currentTime.value
  let lo = 0, hi = lyricLines.value.length - 1, idx = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (lyricLines.value[mid].time <= t) {
      idx = mid
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  return idx
})

// Display 9 lyric lines centered at current
const lyricDisplayLines = computed(() => {
  const idx = currentLyricIndex.value
  const lines = lyricLines.value
  if (lines.length === 0) {
    return Array.from({ length: 9 }, () => ({ text: '', active: false }))
  }
  const result = []
  for (let i = -4; i <= 4; i++) {
    const lineIdx = idx + i
    if (lineIdx >= 0 && lineIdx < lines.length) {
      result.push({ text: lines[lineIdx].text, active: i === 0 })
    } else {
      result.push({ text: '', active: false })
    }
  }
  return result
})

// Cover and background
const coverUrl = ref('')
const backgroundUrl = ref('')

// Icon
const iconUrl = '/icon.png'

// Audio extension regex
const audioExtRegex = /\.[^.]+$/

// Music directory
const musicDir = ref('')

// Video
const isVideoMode = ref(false)
const isVideoFullscreen = ref(false)
const videoPlayerRef = ref<InstanceType<typeof VideoPlayer> | null>(null)

// Window state
const isSmallScreen = ref(false)
const isMiniMode = ref(false)
const alwaysOnTop = ref(false)
const isMinimized = ref(false)
const miniModeRestorePos = ref({ x: 0, y: 0 })
const miniModeRestoreAlwaysOnTop = ref(false)

// Computed
const trackTitle = computed(() => {
  const t = playlistState.currentTrack.value.name
  return t.replace(audioExtRegex, '')
})

const titlebarTitle = computed(() => {
  const mode = isVideoMode.value ? '视频' : '音乐'
  return trackTitle.value === '未选择' ? '我的音乐盒' : `${mode} - ${trackTitle.value}`
})

// 当前视频的缩略图URL
const videoThumbnailUrl = computed(() => {
  const track = playlistState.currentTrack.value
  if (!track?.thumbnailPath || !audioPlayer.streamUrl.value) return ''
  return `${audioPlayer.streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(track.thumbnailPath)}`
})

const volumeIcon = computed(() => {
  if (audioPlayer.volume.value === 0) return 'vol-mute'
  if (audioPlayer.volume.value < 50) return 'vol-low'
  return 'vol-high'
})

// ============== Window Controls ==============
function toggleMenu() {
  showMenu.value = !showMenu.value
}

async function toggleSmallScreen() {
  const FULL_WIDTH = 924
  const THIRD_WIDTH = 308
  const HEIGHT = 568

  isSmallScreen.value = !isSmallScreen.value

  const pos = await WindowGetPosition()
  const currentWidth = isSmallScreen.value ? FULL_WIDTH : THIRD_WIDTH
  const newWidth = isSmallScreen.value ? THIRD_WIDTH : FULL_WIDTH
  const newX = pos.x + currentWidth - newWidth

  WindowSetSize(newWidth, HEIGHT)
  WindowSetPosition(newX, pos.y)
}

async function toggleMiniMode() {
  isMiniMode.value = !isMiniMode.value
  if (isMiniMode.value) {
    const pos = await WindowGetPosition()
    const currentWidth = 924
    const miniWidth = 300
    const newX = pos.x + currentWidth - miniWidth
    miniModeRestorePos.value = { x: pos.x, y: pos.y }
    WindowSetPosition(newX, pos.y)
    WindowSetSize(miniWidth, 60)
    miniModeRestoreAlwaysOnTop.value = alwaysOnTop.value
    alwaysOnTop.value = true
    WindowSetAlwaysOnTop(true)
    setAlwaysOnTopState(true)
  } else {
    WindowSetSize(924, 568)
    WindowSetPosition(miniModeRestorePos.value.x, miniModeRestorePos.value.y)
    alwaysOnTop.value = miniModeRestoreAlwaysOnTop.value
    WindowSetAlwaysOnTop(alwaysOnTop.value)
    setAlwaysOnTopState(alwaysOnTop.value)
  }
}

function toggleAlwaysOnTop() {
  alwaysOnTop.value = !alwaysOnTop.value
  WindowSetAlwaysOnTop(alwaysOnTop.value)
  setAlwaysOnTopState(alwaysOnTop.value)
}

function minimize() {
  if (audioPlayer.isPlaying.value) {
    visualizer.stopVisualizer()
  }
  isMinimized.value = true
  WindowMinimise()
}

function closeWindow() {
  Quit()
}

function onWindowRestore() {
  if (isMinimized.value) {
    isMinimized.value = false
    if (audioPlayer.isPlaying.value) {
      visualizer.startVisualizer()
    }
  }
}

function handleWindowFocus() {
  onWindowRestore()
}

let minimizeCheckInterval: ReturnType<typeof setInterval> | null = null

function startMinimizePolling() {
  if (minimizeCheckInterval) return
  minimizeCheckInterval = setInterval(async () => {
    const isMin = await WindowIsMinimised()
    if (isMin && !isMinimized.value) {
      isMinimized.value = true
      if (audioPlayer.isPlaying.value) {
        visualizer.stopVisualizer()
      }
    } else if (!isMin && isMinimized.value) {
      isMinimized.value = false
      if (audioPlayer.isPlaying.value) {
        visualizer.startVisualizer()
      }
    }
  }, 200)
}

function stopMinimizePolling() {
  if (minimizeCheckInterval) {
    clearInterval(minimizeCheckInterval)
    minimizeCheckInterval = null
  }
}

// ============== Format Time ==============
function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

// ============== Parse LRC Lyric ==============
function parseLyric(lrc: string): { time: number; text: string }[] {
  const lines: { time: number; text: string }[] = []
  const regex = /\[(\d{2}):(\d{2})\.(\d{2,3})\](.*)/g
  let match
  while ((match = regex.exec(lrc)) !== null) {
    const min = parseInt(match[1])
    const sec = parseInt(match[2])
    const ms = match[3].length === 3 ? parseInt(match[3]) : parseInt(match[3]) * 10
    const time = min * 60 + sec + ms / 1000
    const text = match[4].trim()
    if (text) {
      lines.push({ time, text })
    }
  }
  return lines.sort((a, b) => a.time - b.time)
}

// ============== Load Lyric ==============
async function loadLyric(lyricPath: string, artist: string, title: string, musicPath: string, dir: string, durationVal?: number) {
  try {
    const result = await GetLyric(lyricPath, artist, title, musicPath, dir, durationVal || 0)
    lyricContent.value = result.content
    lyricLines.value = parseLyric(result.content)
    if (!result.lyricPath && result.content && playlistState.playlist.value[playlistState.currentIndex.value]) {
      embeddedLyricPaths.value.add(playlistState.playlist.value[playlistState.currentIndex.value].path)
    }
    if (result.lyricPath && !lyricPath && playlistState.playlist.value[playlistState.currentIndex.value]) {
      playlistState.playlist.value[playlistState.currentIndex.value].lyricPath = result.lyricPath
    }
  } catch (e) {
    lyricContent.value = ''
    lyricLines.value = []
  }
}

// ============== Load Track ==============
function loadTrack(index: number) {
  const track = playlistState.playlist.value[index]
  if (!track || !audioPlayer.streamUrl.value) return
  const src = `${audioPlayer.streamUrl.value}?path=${encodeURIComponent(track.path)}`

  if (!track.isVideo) {
    audioPlayer.isAudioLoading.value = true
  } else {
    isVideoLoading.value = true
  }

  // Stop other source
  if (track.isVideo) {
    audioPlayer.audio.pause()
    audioPlayer.audio.src = ''
    // 计算视频封面缩略图URL
    const thumbUrl = track.thumbnailPath
      ? `${audioPlayer.streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(track.thumbnailPath)}`
      : ''
    nextTick(() => {
      videoPlayerRef.value?.loadSrc(src, thumbUrl)
    })
  } else {
    if (videoPlayerRef.value) {
      videoPlayerRef.value.pause()
    }
    audioPlayer.loadTrack(track.path)
  }

  audioPlayer.currentTime.value = 0
  audioPlayer.duration.value = 0
  lyricContent.value = ''
  lyricLines.value = []

  // Background and cover
  if (track.backgroundPath) {
    backgroundUrl.value = `${audioPlayer.streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(track.backgroundPath)}`
  } else {
    backgroundUrl.value = ''
  }

  if (track.coverPath) {
    coverUrl.value = `${audioPlayer.streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(track.coverPath)}`
  } else {
    // 从网易云获取封面
    GetCover(track.artist || '', track.title || '').then(coverPath => {
      if (coverPath) {
        coverUrl.value = `${audioPlayer.streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(coverPath)}`
      } else {
        coverUrl.value = ''
      }
    })
  }

  if (audioPlayer.isPlaying.value) {
    if (track.isVideo) {
      nextTick(() => {
        videoPlayerRef.value?.play()
      })
    } else {
      audioPlayer.play()
    }
  }

  // 预加载下一首歌曲的封面
  preloadNextCover()
}

// 预加载下一首歌曲的封面
function preloadNextCover() {
  const filtered = playlistState.filteredPlaylist.value
  if (filtered.length === 0) return

  const currentIdx = playlistState.currentFilteredIndex.value
  let nextIdx: number

  if (playlistState.playMode.value === 'loop') {
    // 单曲循环时不预加载下一首
    return
  } else if (playlistState.playMode.value === 'random') {
    nextIdx = Math.floor(Math.random() * filtered.length)
  } else {
    nextIdx = (currentIdx + 1) % filtered.length
  }

  const nextTrack = filtered[nextIdx]
  if (!nextTrack) return

  // 如果已经有封面则不需要预加载
  if (nextTrack.coverPath) return

  // 异步预加载封面，不等待结果
  GetCover(nextTrack.artist || '', nextTrack.title || '').then(coverPath => {
    if (coverPath) {
      nextTrack.coverPath = coverPath
    }
  })
}

// ============== Playback Controls ==============
function togglePlay() {
  if (!playlistState.currentTrack.value.path) return
  const track = playlistState.playlist.value[playlistState.currentIndex.value]
  if (track?.isVideo) {
    videoPlayerRef.value?.togglePlay()
  } else {
    audioPlayer.togglePlay()
  }
}

function prevTrack() {
  const newIndex = playlistState.prevTrack()
  if (newIndex >= 0) {
    loadTrack(newIndex)
  }
}

function handleTrackEnded() {
  const newIndex = playlistState.nextTrack(playlistState.playlist.value[playlistState.currentIndex.value])
  if (newIndex >= 0) {
    loadTrack(newIndex)
    audioPlayer.play()
  }
}

function nextTrack() {
  const newIndex = playlistState.nextTrack(playlistState.playlist.value[playlistState.currentIndex.value])
  if (newIndex >= 0) {
    if (playlistState.playMode.value === 'loop') {
      const track = playlistState.playlist.value[playlistState.currentIndex.value]
      if (track?.isVideo) {
        videoPlayerRef.value?.seek(0)
        videoPlayerRef.value?.play()
      } else {
        audioPlayer.seek(0)
        audioPlayer.play()
      }
    } else {
      loadTrack(newIndex)
    }
  }
}

function togglePlayMode() {
  playlistState.togglePlayMode()
}

function selectTrack(index: number) {
  playlistState.selectTrack(index)
  loadTrack(index)
  audioPlayer.isPlaying.value = true
  showPlaylist.value = false
  const track = playlistState.playlist.value[index]
  if (track?.isVideo) {
    videoPlayerRef.value?.play()
  } else {
    audioPlayer.play()
  }
  saveCurrentPlayState()
}

function seekProgress(e: MouseEvent) {
  if (!audioPlayer.duration.value) return
  const bar = (e.currentTarget as HTMLElement)
  const rect = bar.getBoundingClientRect()
  const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  const targetTime = ratio * audioPlayer.duration.value
  audioPlayer.currentTime.value = targetTime
  const currentTrackItem = playlistState.playlist.value[playlistState.currentIndex.value]
  if (currentTrackItem?.isVideo) {
    videoPlayerRef.value?.seek(targetTime)
  } else {
    audioPlayer.seek(targetTime)
  }
}

function toggleVideoFullscreen() {
  isVideoFullscreen.value = !isVideoFullscreen.value
  if (isVideoFullscreen.value) {
    document.documentElement.requestFullscreen?.()
  } else {
    document.exitFullscreen?.()
  }
}

function changeVolume(e: Event) {
  const input = e.target as HTMLInputElement
  const vol = parseInt(input.value)
  audioPlayer.setVolume(vol)
  videoPlayerRef.value?.setVolume(vol)
}

function togglePlaylist() {
  showPlaylist.value = !showPlaylist.value
}

// Video loading ref
const isVideoLoading = ref(false)

// Video event handlers
function onVideoLoadedmetadata(durationVal: number) {
  isVideoLoading.value = false
  audioPlayer.duration.value = durationVal
  // Save video resolution
  if (videoPlayerRef.value?.video) {
    const v = videoPlayerRef.value.video
    if (v.videoWidth > 0 && v.videoHeight > 0) {
      const resolution = `${v.videoWidth}x${v.videoHeight}`
      const currentPath = playlistState.playlist.value[playlistState.currentIndex.value]?.path
      if (currentPath && !playlistState.videoResolutions.value[currentPath]) {
        playlistState.videoResolutions.value[currentPath] = resolution
        setVideoResolution(currentPath, resolution)
      }
    }
  }
}

function onVideoTimeupdate(time: number) {
  audioPlayer.currentTime.value = time
}

function onVideoEnded() {
  handleTrackEnded()
}

function onVideoError(msg: string) {
  isVideoLoading.value = false
  showToastMsg('视频加载失败，请尝试播放其他文件', 'error')
}

function onVideoPlay() {
  audioPlayer.isPlaying.value = true
  saveCurrentPlayState()
}

function onVideoPause() {
  audioPlayer.isPlaying.value = false
  saveCurrentPlayState()
}

// ============== Click Outside ==============
function handleClickOutside(e: MouseEvent) {
  const target = e.target as Node
  const volumeWrap = document.querySelector('.volume-wrap')
  if (volumeWrap && !volumeWrap.contains(target)) {
    showVolume.value = false
  }
  const menuBtn = (e.target as Element).closest('.menu-btn')
  const menuPanel = document.querySelector('.menu-panel')
  if (menuPanel && !menuPanel.contains(target) && !menuBtn) {
    showMenu.value = false
  }
}

function handleClickCapture(e: MouseEvent) {
  const target = e.target as Node
  const menuPanel = document.querySelector('.menu-panel')
  const menuBtn = (e.target as Element).closest('.menu-btn')
  const volumeWrap = document.querySelector('.volume-wrap')
  if (menuPanel?.contains(target) || menuBtn || volumeWrap?.contains(target)) {
    return
  }
  if (showMenu.value || showVolume.value) {
    showMenu.value = false
    showVolume.value = false
  }
}

// ============== Keyboard ==============
function handleKeydown(e: KeyboardEvent) {
  if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
    return
  }
  switch (e.code) {
    case 'Space':
      e.preventDefault()
      togglePlay()
      break
    case 'ArrowLeft':
      e.preventDefault()
      if (isVideoMode.value) {
        const v = videoPlayerRef.value?.video
        if (v) v.currentTime = Math.max(0, v.currentTime - 10)
      } else {
        audioPlayer.seek(Math.max(0, audioPlayer.currentTime.value - 10))
      }
      break
    case 'ArrowRight':
      e.preventDefault()
      if (isVideoMode.value) {
        const v = videoPlayerRef.value?.video
        if (v) v.currentTime = Math.min(audioPlayer.duration.value, v.currentTime + 10)
      } else {
        audioPlayer.seek(Math.min(audioPlayer.duration.value, audioPlayer.currentTime.value + 10))
      }
      break
    case 'KeyM':
      e.preventDefault()
      audioPlayer.toggleMute()
      break
    case 'F11':
      e.preventDefault()
      if (isVideoMode.value) {
        toggleVideoFullscreen()
      }
      break
  }
}

// ============== Save/Restore Play State ==============
async function saveCurrentPlayState() {
  if (musicDir.value && playlistState.playlist.value.length > 0) {
    await setPlayState({
      musicDir: musicDir.value,
      currentIndex: playlistState.currentIndex.value,
      currentTime: audioPlayer.audio.currentTime,
      isPlaying: audioPlayer.isPlaying.value
    })
  }
}

async function restorePlayState() {
  const state = await getPlayState()
  if (state && state.musicDir && state.currentIndex >= 0) {
    const savedMusicDir = state.musicDir
    const savedIndex = state.currentIndex
    const savedTime = state.currentTime
    const savedIsPlaying = state.isPlaying
    ;(window as any).__restorePlayState = () => {
      if (musicDir.value === savedMusicDir && playlistState.playlist.value.length > savedIndex) {
        playlistState.currentIndex.value = savedIndex
        loadTrack(savedIndex)
        if (savedIsPlaying) {
          audioPlayer.seek(savedTime)
          audioPlayer.play()
        } else {
          audioPlayer.seek(savedTime)
        }
      }
    }
  }
}

function handleBeforeUnload() {
  saveCurrentPlayState()
}

// ============== Load Playlist ==============
async function loadPlaylist() {
  const loadStart = Date.now()
  LogMessage(`[前端] loadPlaylist 开始`)
  audioPlayer.pauseAndClear()
  audioPlayer.isPlaying.value = false
  audioPlayer.currentTime.value = 0
  audioPlayer.duration.value = 0
  lyricContent.value = ''
  lyricLines.value = []

  const dir = await getMusicDir()

  if (!dir) {
    showSetupPrompt.value = true
    playlistState.clearPlaylist()
    LogMessage(`[前端] loadPlaylist 完成 (无音乐目录) 耗时 ${Date.now() - loadStart}ms`)
    return
  }

  if (dir === musicDir.value && playlistState.playlist.value.length > 0) {
    showSetupPrompt.value = false
    LogMessage(`[前端] loadPlaylist 完成 (使用缓存) 耗时 ${Date.now() - loadStart}ms`)
    return
  }

  musicDir.value = dir
  showSetupPrompt.value = false

  // 使用固定 URL
  const url = 'http://localhost:19890/stream'
  audioPlayer.setStreamUrl(url)

  // Set background image URL directly
  theme.bgImage.value = url.replace('/stream', '/theme/bg') + '?t=' + Date.now()

  const result = await GetMediaFiles(dir)
  playlistState.setPlaylist(result.files, result.isVideoMode)
  isVideoMode.value = result.isVideoMode
  const scanElapsed = Date.now() - loadStart
  LogMessage(`[前端] GetMediaFiles 扫描完成，共 ${result.files.length} 个文件 (耗时 ${scanElapsed}ms)`)

  if (result.files.length > 0) {
    playlistState.currentIndex.value = 0
    await nextTick()
    loadTrack(0)
    if ((window as any).__restorePlayState) {
      ;(window as any).__restorePlayState()
      delete (window as any).__restorePlayState
    }
  } else {
    showSetupPrompt.value = true
  }
  LogMessage(`[前端] loadPlaylist 完成 总耗时 ${Date.now() - loadStart}ms`)
}

// ============== Select Music Folder ==============
async function selectMusicFolder() {
  try {
    const path = await SelectMusicFolder()
    if (!path) return

    audioPlayer.pauseAndClear()
    if (videoPlayerRef.value) {
      videoPlayerRef.value.stopAndClear()
    }
    audioPlayer.isPlaying.value = false
    audioPlayer.currentTime.value = 0
    audioPlayer.duration.value = 0
    lyricContent.value = ''
    lyricLines.value = []
    playlistState.clearPlaylist()

    await setMusicDir(path)

    musicDir.value = path
    showSetupPrompt.value = false

    // 使用固定 URL
    audioPlayer.setStreamUrl('http://localhost:19890/stream')

    const result = await GetMediaFiles(path)
    playlistState.setPlaylist(result.files, result.isVideoMode)
    isVideoMode.value = result.isVideoMode

    if (result.files.length > 0) {
      playlistState.currentIndex.value = 0
      await nextTick()
      if (result.isVideoMode) {
        isVideoLoading.value = true
      }
      loadTrack(0)
    } else {
      showSetupPrompt.value = true
    }
  } catch (e) {
    console.error('选择文件夹失败:', e)
  }
}

// ============== Clear Cache ==============
async function clearCache() {
  const { openDB } = await import('./composables/useStorage')
  const db = await openDB()
  return new Promise<void>((resolve, reject) => {
    const tx = db.transaction('settings', 'readwrite')
    const store = tx.objectStore('settings')
    const request = store.clear()
    request.onerror = () => reject(request.error)
    request.onsuccess = async () => {
      theme.resetTheme()
      musicDir.value = ''
      playlistState.clearPlaylist()
      showMenu.value = false
      showSetupPrompt.value = true
      audioPlayer.pauseAndClear()
      audioPlayer.isPlaying.value = false
      isVideoMode.value = false
      alwaysOnTop.value = false
      WindowSetAlwaysOnTop(false)
      setAlwaysOnTopState(false)
      audioPlayer.setVolume(100)
      playlistState.playMode.value = 'sequence'
      coverUrl.value = ''
      // 清理缩略图缓存
      await ClearThumbnails()
      resolve()
    }
  })
}

// ============== Watch video mode ==============
watch(isVideoMode, (newVal) => {
  if (newVal) {
    nextTick(() => {
      // Video listeners are setup in VideoPlayer component
    })
  }
})

// ============== Watch audio current time for lyric ==============
watch(() => audioPlayer.currentTime.value, () => {
  // Lyric update is handled by computed currentLyricIndex
})

// ============== onMounted ==============
onMounted(async () => {
  const mountStart = Date.now()
  LogMessage(`[前端] onMounted 开始`)
  let stepStart = Date.now()
  visualizer.setupAnalyser(audioPlayer.audio)
  LogMessage(`[前端]   - setupAnalyser 耗时: ${Date.now() - stepStart}ms`)
  stepStart = Date.now()
  await nextTick()
  LogMessage(`[前端]   - nextTick 耗时: ${Date.now() - stepStart}ms`)
  stepStart = Date.now()
  await theme.initTheme()
  LogMessage(`[前端]   - initTheme 耗时: ${Date.now() - stepStart}ms`)
  stepStart = Date.now()
  const savedOnTop = await getAlwaysOnTop()
  alwaysOnTop.value = savedOnTop
  WindowSetAlwaysOnTop(savedOnTop)
  LogMessage(`[前端]   - alwaysOnTop 耗时: ${Date.now() - stepStart}ms`)
  stepStart = Date.now()
  audioPlayer.initVolume()
  playlistState.initPlayMode()
  playlistState.videoResolutions.value = await getVideoResolutions()
  await restorePlayState()
  LogMessage(`[前端]   - 播放状态恢复 耗时: ${Date.now() - stepStart}ms`)
  stepStart = Date.now()
  loadPlaylist()
  startMinimizePolling()
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('click', handleClickOutside)
  window.addEventListener('focus', handleWindowFocus)
  window.addEventListener('beforeunload', handleBeforeUnload)
  const mountElapsed = Date.now() - mountStart
  LogMessage(`[前端] onMounted 完成，界面已渲染 (耗时 ${mountElapsed}ms)`)
})

// ============== onUnmounted ==============
onUnmounted(() => {
  visualizer.cleanup()
  audioPlayer.cleanup()
  getCloseDB()()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('click', handleClickOutside)
  window.removeEventListener('focus', handleWindowFocus)
  window.removeEventListener('beforeunload', handleBeforeUnload)
  saveCurrentPlayState()
  stopMinimizePolling()
  StopAudioServer().catch(() => {})
})
</script>

<template>
  <div class="player-container" @mousedown.capture="handleClickCapture">
    <!-- 自定义标题栏 -->
    <div class="custom-titlebar" style="--wails-draggable: drag;">
      <!-- 迷你模式：只显示图标 + 播放/暂停 + 退出 -->
      <div v-if="isMiniMode" class="mini-mode-content">
        <img :src="iconUrl" alt="" class="titlebar-icon" @error="($event.target as HTMLImageElement).style.display = 'none'" />
        <div class="mini-controls">
          <button class="mini-btn" @click.stop="prevTrack" style="--wails-draggable: no-drag;">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>
          </button>
          <button class="mini-btn" @click.stop="togglePlay" style="--wails-draggable: no-drag;">
            <svg v-if="!audioPlayer.isPlaying.value" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
          </button>
          <button class="mini-btn" @click.stop="nextTrack" style="--wails-draggable: no-drag;">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
          </button>
          <button class="mini-btn mini-exit" @click.stop="toggleMiniMode" style="--wails-draggable: no-drag;" title="退出迷你模式">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z"/></svg>
          </button>
        </div>
      </div>
      <!-- 普通/小屏模式 -->
      <template v-else>
        <div class="titlebar-drag">
          <img :src="iconUrl" alt="" class="titlebar-icon" @error="($event.target as HTMLImageElement).style.display = 'none'" />
          <span>{{ isSmallScreen ? trackTitle : titlebarTitle }}</span>
        </div>
        <div class="titlebar-controls" style="--wails-draggable: no-drag;">
          <button class="titlebar-btn menu-btn" @click.stop="toggleMenu" style="--wails-draggable: no-drag;" title="菜单">☰</button>
          <MenuPanel
            v-if="showMenu"
            :bgColor="theme.bgColor.value"
            :btnColor="theme.btnColor.value"
            :vizColor="theme.vizColor.value"
            :lyricColor="theme.lyricColor.value"
            :titleColor="theme.titleColor.value"
            :titlebarColor="theme.titlebarColor.value"
            :lyricsColor="theme.lyricsColor.value"
            :isSmallScreen="isSmallScreen"
            :isMiniMode="isMiniMode"
            :alwaysOnTop="alwaysOnTop"
            :isVideoMode="isVideoMode"
            @selectFolder="selectMusicFolder(); showMenu = false"
            @selectBgImage="theme.saveBgImage(audioPlayer.streamUrl.value); showMenu = false"
            @clearCache="clearCache(); showMenu = false"
            @toggleSmallScreen="toggleSmallScreen(); showMenu = false"
            @toggleMiniMode="toggleMiniMode(); showMenu = false"
            @toggleAlwaysOnTop="toggleAlwaysOnTop(); showMenu = false"
            @update:bgColor="theme.updateBgColor"
            @update:btnColor="theme.updateBtnColor"
            @update:vizColor="theme.updateVizColor"
            @update:lyricColor="theme.updateLyricColor"
            @update:titleColor="theme.updateTitleColor"
            @update:titlebarColor="theme.updateTitlebarColor"
            @update:lyricsColor="theme.updateLyricsColor"
          />
          <button class="titlebar-btn minimize" @click="minimize" style="--wails-draggable: no-drag;">─</button>
          <button class="titlebar-btn close" @click="closeWindow" style="--wails-draggable: no-drag;">✕</button>
        </div>
      </template>
    </div>

    <!-- 迷你模式主内容区 -->
    <div v-if="isMiniMode" class="mini-main-content">
      <span class="mini-lyric-text">{{ lyricLines[currentLyricIndex]?.text || '暂无歌词' }}</span>
    </div>

    <!-- 背景装饰 -->
    <div class="bg-glow bg-glow-1" v-show="!isMiniMode && !isMinimized"></div>
    <div class="bg-glow bg-glow-2" v-show="!isMiniMode && !isMinimized"></div>
    <div class="bg-glow bg-glow-3" v-show="!isMiniMode && !isMinimized"></div>
    <img v-show="backgroundUrl && !isMiniMode && !isMinimized" :src="backgroundUrl" class="background-image" alt="" @error="($event.target as HTMLImageElement).style.display = 'none'" />
    <img v-show="theme.bgImage.value && !isMiniMode && !isMinimized" :src="theme.bgImage.value" class="custom-bg-image" alt="" @error="theme.bgImage.value = ''" />

    <!-- 设置提示 -->
    <div v-if="showSetupPrompt && !isMiniMode && !isMinimized" class="setup-prompt" @click="selectMusicFolder">
      <div class="setup-content">
        <div class="setup-icon">&#127925;</div>
        <h2>单击添加你的音乐文件夹</h2>
      </div>
    </div>

    <!-- Toast -->
    <div v-if="showToast && !isMiniMode && !isMinimized" class="toast" :class="toastType">
      {{ toastMessage }}
    </div>

    <!-- 视频播放区域 -->
    <VideoPlayer
      v-if="isVideoMode && !isMiniMode && !isMinimized"
      ref="videoPlayerRef"
      :thumbnail="videoThumbnailUrl"
      :isFullscreen="isVideoFullscreen"
      @dblclick-fullscreen="toggleVideoFullscreen"
      @loadedmetadata="onVideoLoadedmetadata"
      @timeupdate="onVideoTimeupdate"
      @ended="onVideoEnded"
      @error="onVideoError"
      @play="onVideoPlay"
      @pause="onVideoPause"
    />

    <!-- 主内容区 -->
    <div class="main-content" :class="{ 'small-screen': isSmallScreen }" v-show="!showSetupPrompt && !isMiniMode && !isMinimized && !isVideoMode">
      <div class="album-wrapper" v-if="!isSmallScreen">
        <div class="album-glow"></div>
        <div class="album-cover">
          <img
            :src="coverUrl || '/default_cover.jpeg'"
            :alt="trackTitle"
            @error="($event.target as HTMLImageElement).src = '/default_cover.jpeg'"
          />
        </div>
      </div>

      <div class="right-content" :class="{ 'small-screen': isSmallScreen, 'centered': isSmallScreen }">
        <div class="track-info">
          <div class="track-title">{{ trackTitle }}</div>
          <div class="lyric-display">
            <div
              v-for="(line, i) in lyricDisplayLines"
              :key="i"
              class="lyric-line"
              :class="{ active: line.active }"
            >{{ line.text }}</div>
          </div>
        </div>

        <div class="visualizer" v-show="!isSmallScreen">
          <div
            v-for="(h, i) in visualizer.visualizerBars.value"
            :key="i"
            class="viz-bar"
            :style="{ height: audioPlayer.isPlaying.value ? Math.min(80, Math.max(4, h * 0.4)) + 'px' : '4px' }"
          ></div>
        </div>
      </div>
    </div>

    <!-- 底部控制栏 -->
    <div class="controls-bar" :class="{ 'small-screen': isSmallScreen }" v-show="!isMiniMode && !isMinimized">
      <template v-if="!isSmallScreen">
        <div class="ctrl-group">
          <button class="ctrl-btn" @click="prevTrack">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>
          </button>
          <button class="ctrl-btn play-btn" @click="togglePlay">
            <svg v-if="!audioPlayer.isPlaying.value" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
          </button>
          <button class="ctrl-btn" @click="nextTrack">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
          </button>
        </div>

        <div class="progress-section">
          <div class="progress-bar" @click="seekProgress">
            <div class="progress-fill" :style="{ width: (audioPlayer.currentTime.value / audioPlayer.duration.value * 100) + '%' }"></div>
            <div class="progress-thumb" :style="{ left: (audioPlayer.currentTime.value / audioPlayer.duration.value * 100) + '%' }"></div>
            <div
              v-for="i in Math.floor((audioPlayer.duration.value || 0) / 600)"
              :key="i"
              class="progress-tick"
              :style="{ left: (i * 600 / (audioPlayer.duration.value || 1) * 100) + '%' }"
            ></div>
          </div>
          <span class="time-label">{{ formatTime(audioPlayer.currentTime.value) }} / {{ formatTime(audioPlayer.duration.value || 0) }}</span>
        </div>

        <div class="ctrl-group-right">
          <div class="volume-wrap" @click.stop>
            <button class="icon-btn" @click="showVolume = !showVolume">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path v-if="volumeIcon === 'vol-mute'" d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
                <path v-else-if="volumeIcon === 'vol-low'" d="M7 9v6h4l5 5V4l-5 5H7z"/>
                <path v-else d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
              </svg>
            </button>
            <div class="volume-popup" v-if="showVolume">
              <div class="vol-track" @click="changeVolume">
                <div class="vol-fill" :style="{ height: audioPlayer.volume.value + '%' }"></div>
              </div>
              <input type="range" class="vol-slider" min="0" max="100" :value="audioPlayer.volume.value" @input="changeVolume" orient="vertical" />
            </div>
          </div>

          <button class="icon-btn mode-btn" @click="togglePlayMode" :title="playlistState.playModeTitle.value">
            <svg v-if="playlistState.playMode.value === 'sequence'" viewBox="0 0 24 24" fill="currentColor"><path d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z"/></svg>
            <svg v-else-if="playlistState.playMode.value === 'loop'" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M10.59 9.17L5.41 4 4 5.41l5.17 5.17 1.42-1.41zM14.5 4l2.04 2.04L4 18.59 5.41 20 17.96 7.46 20 9.5V4h-5.5zm.33 9.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04-3.13-3.13z"/></svg>
          </button>
          <button class="icon-btn" :class="{ active: showPlaylist }" @click="togglePlaylist" title="播放列表">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M15 6H3v2h12V6zm0 4H3v2h12v-2zM3 16h8v-2H3v2zM17 6v6.18c-.31-.11-.65-.18-1-.18-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3V8h3V6h-5z"/></svg>
          </button>
        </div>
      </template>

      <template v-else>
        <div class="progress-section small-screen-progress">
          <div class="progress-bar" @click="seekProgress">
            <div class="progress-fill" :style="{ width: (audioPlayer.currentTime.value / audioPlayer.duration.value * 100) + '%' }"></div>
            <div class="progress-thumb" :style="{ left: (audioPlayer.currentTime.value / audioPlayer.duration.value * 100) + '%' }"></div>
            <div
              v-for="i in Math.floor((audioPlayer.duration.value || 0) / 600)"
              :key="i"
              class="progress-tick"
              :style="{ left: (i * 600 / (audioPlayer.duration.value || 1) * 100) + '%' }"
            ></div>
          </div>
          <span class="time-label">{{ formatTime(audioPlayer.currentTime.value) }} / {{ formatTime(audioPlayer.duration.value || 0) }}</span>
        </div>

        <div class="ctrl-row">
          <div class="ctrl-group">
            <button class="ctrl-btn" @click="prevTrack">
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>
            </button>
            <button class="ctrl-btn play-btn" @click="togglePlay">
              <svg v-if="!audioPlayer.isPlaying.value" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
              <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
            </button>
            <button class="ctrl-btn" @click="nextTrack">
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
            </button>
          </div>

          <div class="ctrl-group-right">
            <div class="volume-wrap" @click.stop>
              <button class="icon-btn" @click="showVolume = !showVolume">
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path v-if="volumeIcon === 'vol-mute'" d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
                  <path v-else-if="volumeIcon === 'vol-low'" d="M7 9v6h4l5 5V4l-5 5H7z"/>
                  <path v-else d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
                </svg>
              </button>
            </div>

            <button class="icon-btn mode-btn" @click="togglePlayMode" :title="playlistState.playModeTitle.value">
              <svg v-if="playlistState.playMode.value === 'sequence'" viewBox="0 0 24 24" fill="currentColor"><path d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z"/></svg>
              <svg v-else-if="playlistState.playMode.value === 'loop'" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/></svg>
              <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M10.59 9.17L5.41 4 4 5.41l5.17 5.17 1.42-1.41zM14.5 4l2.04 2.04L4 18.59 5.41 20 17.96 7.46 20 9.5V4h-5.5zm.33 9.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04-3.13-3.13z"/></svg>
            </button>
            <button class="icon-btn" :class="{ active: showPlaylist }" @click="togglePlaylist" title="播放列表">
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M15 6H3v2h12V6zm0 4H3v2h12v-2zM3 16h8v-2H3v2zM17 6v6.18c-.31-.11-.65-.18-1-.18-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3V8h3V6h-5z"/></svg>
            </button>
          </div>
        </div>
      </template>
    </div>

    <!-- 播放列表面板 -->
    <Transition name="slide">
      <PlaylistPanel
        v-show="showPlaylist && !isMiniMode && !isMinimized"
        :playlist="(playlistState.playlist.value as any[])"
        :currentIndex="playlistState.currentIndex.value"
        :isPlaying="audioPlayer.isPlaying.value"
        :embeddedLyricPaths="embeddedLyricPaths"
        :isSmallScreen="isSmallScreen"
        :videoResolutions="(playlistState.videoResolutions.value as Record<string, string>)"
        @select="selectTrack"
      />
    </Transition>
  </div>
</template>

<style scoped>
/* 自定义标题栏 */
.custom-titlebar {
  position: relative;
  z-index: 999;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 30px;
  background: rgba(5, 8, 18, 0.5);
  flex-shrink: 0;
  user-select: none;
  cursor: move;
}

.titlebar-drag {
  flex: 1;
  height: 100%;
  display: flex;
  align-items: center;
  padding-left: 8px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.3);
  letter-spacing: 1px;
  gap: 8px;
  overflow: hidden;
}

.titlebar-drag img {
  flex-shrink: 0;
}

.titlebar-drag span {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
  color: var(--titlebar-color);
}

.titlebar-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.titlebar-controls {
  display: flex;
  gap: 0;
}

.titlebar-btn {
  width: 46px;
  height: 30px;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.35);
  cursor: pointer;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s, color 0.15s;
  flex-shrink: 0;
}

.titlebar-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.7);
  cursor: pointer;
}

.titlebar-btn.close:hover {
  background: rgba(220, 60, 60, 0.85);
  color: #ffffff;
  cursor: pointer;
}

/* 迷你模式内容 */
.mini-mode-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 100%;
  padding: 0 8px;
  gap: 12px;
}

/* 迷你模式主内容区 */
.mini-main-content {
  position: absolute;
  top: 30px;
  left: 0;
  right: 0;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(10, 15, 30, 0.6);
  backdrop-filter: blur(5px);
}

.mini-lyric-text {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.7);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: 1px;
  padding: 0 16px;
  text-align: center;
}

.mini-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.mini-btn {
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 4px;
  transition: all 0.15s;
}

.mini-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.8);
}

.mini-btn svg {
  width: 16px;
  height: 16px;
}

.mini-exit:hover {
  background: rgba(220, 60, 60, 0.7);
  color: #ffffff;
}

.player-container {
  position: relative;
  width: 100%;
  height: 100vh;
  margin: 0;
  padding: 0;
  background: v-bind('theme.bgColor.value');
  --btn-color: v-bind('theme.btnColor.value');
  --viz-color: v-bind('theme.vizColor.value');
  --lyric-color: v-bind('theme.lyricColor.value');
  --title-color: v-bind('theme.titleColor.value');
  --titlebar-color: v-bind('theme.titlebarColor.value');
  --lyrics-color: v-bind('theme.lyricsColor.value');
  display: flex;
  flex-direction: column;
  overflow: hidden;
  color: #e0e6f0;
  font-family: 'Microsoft YaHei', 'PingFang SC', -apple-system, sans-serif;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

/* 设置音乐文件夹提示 */
.setup-prompt {
  position: absolute;
  top: 36px;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
}

.setup-content {
  text-align: center;
  padding: 40px;
}

.setup-icon {
  font-size: 64px;
  margin-bottom: 20px;
}

.setup-content h2 {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 12px 0;
  color: rgba(255, 255, 255, 0.9);
}

/* Toast 提示 */
.toast {
  position: fixed;
  bottom: 100px;
  left: 50%;
  transform: translateX(-50%);
  padding: 12px 24px;
  border-radius: 8px;
  font-size: 14px;
  z-index: 1000;
  animation: toastFadeIn 0.3s ease;
}

.toast.success {
  background: rgba(46, 204, 113, 0.9);
  color: white;
}

.toast.error {
  background: rgba(231, 76, 60, 0.9);
  color: white;
}

@keyframes toastFadeIn {
  from {
    opacity: 0;
    transform: translateX(-50%) translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateX(-50%) translateY(0);
  }
}

.setup-content p {
  font-size: 14px;
  margin: 0 0 24px 0;
  color: rgba(255, 255, 255, 0.5);
}

.setup-btn {
  background: linear-gradient(135deg, #4a90d9 0%, #74b9ff 100%);
  color: white;
  border: none;
  padding: 12px 32px;
  border-radius: 24px;
  font-size: 15px;
  cursor: pointer;
  transition: all 0.2s;
}

.setup-btn:hover {
  transform: scale(1.05);
  box-shadow: 0 4px 20px rgba(74, 144, 217, 0.4);
}

/* 背景光晕 */
.bg-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(20px);
  opacity: 0.4;
  pointer-events: none;
  will-change: opacity, transform;
  contain: layout style;
}

.bg-glow-1 {
  width: 400px;
  height: 400px;
  background: radial-gradient(circle, #1e3a5f 0%, transparent 70%);
  top: -100px;
  left: 50%;
  transform: translateX(-50%);
}

.bg-glow-2 {
  width: 300px;
  height: 300px;
  background: radial-gradient(circle, #2d4a6f 0%, transparent 70%);
  top: 20%;
  left: 10%;
  opacity: 0.2;
}

.bg-glow-3 {
  width: 250px;
  height: 250px;
  background: radial-gradient(circle, #1a3050 0%, transparent 70%);
  bottom: 25%;
  right: 10%;
  opacity: 0.25;
}

/* 背景图片 */
.background-image {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.3;
  pointer-events: none;
  z-index: 0;
}

/* 自定义背景图片 */
.custom-bg-image {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.6;
  pointer-events: none;
  z-index: 0;
}

/* 主内容区 */
.main-content {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 80px;
  min-height: 0;
  width: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

.main-content > * {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

.main-content.small-screen {
  justify-content: center;
}

/* 左侧专辑封面 */
.album-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-shrink: 0;
  margin-top: -50px;
  margin-left: 20px;
}

.album-glow {
  position: absolute;
  width: 350px;
  height: 350px;
  background: radial-gradient(circle, rgba(200, 120, 180, 0.4) 0%, rgba(120, 80, 160, 0.2) 40%, transparent 70%);
  border-radius: 50%;
  filter: blur(15px);
  will-change: opacity, transform;
}

@keyframes pulse-glow {
  0%, 100% { opacity: 0.6; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.05); }
}

.album-cover {
  width: 350px;
  height: 350px;
  border-radius: 50%;
  overflow: hidden;
  box-shadow: 0 0 60px rgba(180, 100, 200, 0.4), 0 20px 40px rgba(0, 0, 0, 0.4);
  position: relative;
  z-index: 1;
  opacity: 0.6;
  margin-left: 0;
  contain: layout style paint;
}

.album-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.album-cover.spinning {
  will-change: transform;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 右侧信息区 */
.right-content {
  width: 500px;
  height: 400px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 20px;
  flex-shrink: 0;
  overflow: hidden;
  margin-top: -50px;
  margin-right: 0;
}

.right-content > .track-info {
  flex: 7;
}

.right-content > .visualizer {
  flex: 3;
}

.right-content.small-screen {
  width: 90%;
  margin: 0 auto;
  margin-top: 0;
}

.right-content.small-screen .visualizer {
  width: 100%;
}

/* 歌曲信息 */
.track-info {
  text-align: left;
  width: 100%;
}

.track-title {
  font-size: 28px;
  font-weight: 600;
  color: var(--title-color);
  letter-spacing: 2px;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.5);
}

.lyric-display {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 5px;
  flex: 1;
  overflow: hidden;
}

.lyric-line {
  font-size: 18px;
  color: var(--lyrics-color);
  letter-spacing: 1px;
  transition: all 0.3s ease;
  white-space: nowrap;
  opacity: 0.4;
  line-height: 1.2;
}

.lyric-line.active {
  color: var(--lyric-color);
  font-size: 24px;
  opacity: 1;
  text-shadow: 0 0 20px color-mix(in srgb, var(--lyric-color) 60%, white);
}

/* 可视化 */
.visualizer {
  display: flex;
  align-items: flex-end;
  justify-content: flex-start;
  gap: 3px;
  width: 500px;
  contain: layout style;
}

.viz-bar {
  width: 6px;
  background: linear-gradient(to top, var(--viz-color), color-mix(in srgb, var(--viz-color) 70%, white));
  border-radius: 2px;
  will-change: height;
  flex-shrink: 0;
}

/* 小屏模式 */
.progress-row {
  position: absolute;
  bottom: 80px;
  left: 30px;
  right: 30px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.progress-row .progress-bar {
  flex: 1;
  height: 8px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  position: relative;
  cursor: pointer;
}

.progress-row .progress-fill {
  height: 100%;
  background: var(--btn-color);
  border-radius: 4px;
}

.progress-row .progress-thumb {
  position: absolute;
  top: 50%;
  width: 16px;
  height: 16px;
  background: #ffffff;
  border-radius: 50%;
  transform: translate(-50%, -50%);
  box-shadow: 0 0 8px rgba(100, 160, 255, 1);
}

.progress-row .progress-tick {
  position: absolute;
  top: 0;
  width: 1px;
  height: 100%;
  background: rgba(255, 255, 255, 0.3);
  transform: translateX(-50%);
}

.progress-row .time-label {
  flex-shrink: 0;
}

/* 控制栏 */
.controls-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 0 30px 30px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  z-index: 10;
}

.controls-bar.small-screen {
  flex-direction: column;
  padding: 0 20px 20px;
  gap: 10px;
  align-items: stretch;
  justify-content: flex-end;
}

.controls-bar.small-screen .small-screen-progress {
  width: 100%;
}

.controls-bar.small-screen .ctrl-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.controls-bar.small-screen .ctrl-group {
  display: flex;
  justify-content: flex-start;
}

.controls-bar.small-screen .ctrl-group-right {
  display: flex;
  justify-content: flex-end;
}

/* 进度条 */
.progress-section {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

/* 播放控制组 */
.ctrl-group {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.ctrl-group-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.ctrl-btn {
  background: none;
  border: none;
  color: var(--btn-color);
  cursor: pointer;
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s;
}

.ctrl-btn:hover {
  color: var(--btn-color);
  filter: brightness(1.2);
  background: color-mix(in srgb, var(--btn-color) 15%, transparent);
}

.ctrl-btn svg {
  width: 20px;
  height: 20px;
}

.ctrl-btn.play-btn {
  background: transparent;
}

.ctrl-btn.play-btn svg {
  width: 22px;
  height: 22px;
}

.time-label {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.35);
  min-width: 34px;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.progress-bar {
  flex: 1;
  height: 8px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  position: relative;
  cursor: pointer;
}

.progress-fill {
  height: 100%;
  background: var(--btn-color);
  border-radius: 4px;
}

.progress-thumb {
  position: absolute;
  top: 50%;
  width: 16px;
  height: 16px;
  background: #ffffff;
  border-radius: 50%;
  transform: translate(-50%, -50%);
  box-shadow: 0 0 8px rgba(100, 160, 255, 1);
  opacity: 1;
  z-index: 2;
}

.progress-tick {
  position: absolute;
  top: 0;
  width: 1px;
  height: 100%;
  background: rgba(255, 255, 255, 0.3);
  transform: translateX(-50%);
}

/* 音量包装 */
.volume-wrap {
  position: relative;
  flex-shrink: 0;
}

.volume-popup {
  position: absolute;
  bottom: 40px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(15, 20, 40, 0.95);
  backdrop-filter: blur(5px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 14px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
  z-index: 200;
}

.vol-track {
  width: 6px;
  height: 80px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 3px;
  position: relative;
  cursor: pointer;
}

.vol-fill {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  background: linear-gradient(to top, #4a90d9, #74b9ff);
  border-radius: 3px;
}

.vol-slider {
  -webkit-appearance: none;
  writing-mode: vertical-lr;
  direction: rtl;
  width: 6px;
  height: 80px;
  background: transparent;
  cursor: pointer;
  position: absolute;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  opacity: 0;
}

.vol-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 14px;
  height: 14px;
  background: #ffffff;
  border-radius: 50%;
  box-shadow: 0 0 6px rgba(100, 160, 255, 0.8);
}

/* 图标按钮 */
.icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--btn-color);
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  transition: all 0.2s;
  flex-shrink: 0;
}

.icon-btn:hover {
  color: var(--btn-color);
  filter: brightness(1.2);
  background: color-mix(in srgb, var(--btn-color) 15%, transparent);
}

.icon-btn.active {
  color: var(--btn-color);
  filter: brightness(1.1);
}

.icon-btn svg {
  width: 20px;
  height: 20px;
}

/* 播放模式按钮 */
.mode-btn {
  flex-shrink: 0;
}

.mode-btn svg {
  width: 20px;
  height: 20px;
}
</style>
