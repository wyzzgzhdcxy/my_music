<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { GetMusicFiles, StartAudioServer, StopAudioServer, GetLyric, SelectMusicFolder } from '../wailsjs/go/main/App'
import { WindowMinimise, Quit, WindowSetSize, WindowSetPosition, WindowGetPosition } from '../wailsjs/runtime/runtime'
import PlaylistPanel from './components/PlaylistPanel.vue'
import MenuPanel from './components/MenuPanel.vue'

// 播放状态
const isPlaying = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const volume = ref(70)
const currentIndex = ref(0)

// 播放模式: sequence=顺序播放, loop=单曲循环, random=随机播放
const playMode = ref<'sequence' | 'loop' | 'random'>('sequence')
const playModeTitle = computed(() => {
  return playMode.value === 'sequence' ? '顺序播放' : playMode.value === 'loop' ? '单曲循环' : '随机播放'
})

// 播放列表
const showPlaylist = ref(false)
const showVolume = ref(false)
const playlist = ref<Awaited<ReturnType<typeof GetMusicFiles>>>([])

// 歌词
const lyricContent = ref('')
const lyricLines = ref<{ time: number; text: string }[]>([])

// 已发现有内嵌歌词的曲目路径集合（按需检查，不预扫描）
const embeddedLyricPaths = ref(new Set<string>())

// 音频对象
const audio = new Audio()
audio.crossOrigin = 'anonymous'  // 允许 Web Audio API 访问跨域音频
audio.volume = volume.value / 100

// 音频服务器地址
const streamUrl = ref('')

// 背景图片
const backgroundUrl = ref('')

// 封面图片
const coverUrl = ref('')

// 图标
const iconUrl = '/icon.png'

// 正则预编译
const audioExtRegex = /\.[^.]+$/

// 音乐文件夹
const musicDir = ref('')
const showSetupPrompt = ref(false)

// Toast 提示
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const showToast = ref(false)
let toastTimer: ReturnType<typeof setTimeout> | null = null

function showToastMsg(msg: string, type: 'success' | 'error' = 'success') {
  if (toastTimer !== null) {
    clearTimeout(toastTimer)
    toastTimer = null
  }
  toastMessage.value = msg
  toastType.value = type
  showToast.value = true
  toastTimer = setTimeout(() => {
    showToast.value = false
    toastTimer = null
  }, 3000)
}

// IndexedDB helper
const DB_NAME = 'my_music_db'
const STORE_NAME = 'settings'
const DB_KEY = 'my_music_home'
const DB_KEY_BG_COLOR = 'my_music_bg_color'
const DB_KEY_BG_IMAGE = 'my_music_bg_image'
let dbInstance: IDBDatabase | null = null

function openDB(): Promise<IDBDatabase> {
  if (dbInstance) return Promise.resolve(dbInstance)
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, 1)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => { dbInstance = request.result; resolve(request.result) }
    request.onupgradeneeded = (event) => {
      const db = (event.target as IDBOpenDBRequest).result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME)
      }
    }
  })
}

async function getMusicDir(): Promise<string> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const request = store.get(DB_KEY)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve(request.result || '')
  })
}

async function setMusicDir(path: string): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const request = store.put(path, DB_KEY)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve()
  })
}

async function getBgColor(): Promise<string> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const request = store.get(DB_KEY_BG_COLOR)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve(request.result || '')
  })
}

async function setBgColor(color: string): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const request = store.put(color, DB_KEY_BG_COLOR)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve()
  })
}

async function getBgImage(): Promise<string> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const request = store.get(DB_KEY_BG_IMAGE)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve(request.result || '')
  })
}

async function setBgImage(imageData: string): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const request = store.put(imageData, DB_KEY_BG_IMAGE)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve()
  })
}

async function saveBgImage() {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.onchange = async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = async (ev) => {
      const dataUrl = ev.target?.result as string
      await setBgImage(dataUrl)
      bgImage.value = dataUrl
      showMenu.value = false
    }
    reader.readAsDataURL(file)
  }
  input.click()
}

async function clearCache() {
  const db = await openDB()
  return new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const request = store.clear()
    request.onerror = () => reject(request.error)
    request.onsuccess = () => {
      bgColor.value = 'linear-gradient(180deg, #0a0f1e 0%, #0d1a2d 30%, #0a1628 60%, #0f1c33 100%)'
      bgImage.value = ''
      musicDir.value = ''
      playlist.value = []
      showMenu.value = false
      showSetupPrompt.value = true
      audio.pause()
      audio.src = ''
      isPlaying.value = false
      resolve()
    }
  })
}

// 窗口控制

// 背景颜色
const bgColor = ref('linear-gradient(180deg, #0a0f1e 0%, #0d1a2d 30%, #0a1628 60%, #0f1c33 100%)')
const bgImage = ref('')
const showMenu = ref(false)

// 小屏模式
const isSmallScreen = ref(false)

// 迷你模式
const isMiniMode = ref(false)
const miniModeRestorePos = ref({ x: 0, y: 0 })

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
  } else {
    WindowSetSize(924, 568)
    WindowSetPosition(miniModeRestorePos.value.x, miniModeRestorePos.value.y)
  }
}

function updateBgColor(color: string) {
  bgColor.value = color
  saveBgColor()
}

async function saveBgColor() {
  await setBgColor(bgColor.value)
}
function minimize() {
  WindowMinimise()
}

function closeWindow() {
  Quit()
}

// 可视化
const visualizerBars = ref<number[]>(Array(40).fill(0))
let visualizerRaf: number | null = null

// Web Audio API
let audioContext: AudioContext | null = null
let analyserNode: AnalyserNode | null = null
let frequencyData: Uint8Array | null = null
let audioBuffer: AudioBuffer | null = null
let bufferSource: AudioBufferSourceNode | null = null
let audioStartTime = 0
let audioPauseTime = 0

const volumeIcon = computed(() => {
  if (volume.value === 0) return 'vol-mute'
  if (volume.value < 50) return 'vol-low'
  return 'vol-high'
})

const currentTrack = computed(() => playlist.value?.[currentIndex.value] || { name: '未选择', path: '', size: '' })

const trackTitle = computed(() => {
  const t = currentTrack.value.name
  return t.replace(audioExtRegex, '')
})

const titlebarTitle = computed(() => {
  return trackTitle.value === '未选择' ? '我的音乐盒' : `我的音乐盒 - ${trackTitle.value}`
})

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

// 解析LRC歌词
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

// 加载歌词
async function loadLyric(lyricPath: string, artist: string, title: string, musicPath: string, musicDir: string, duration?: number) {
  try {
    const result = await GetLyric(lyricPath, artist, title, musicPath, musicDir, duration || 0)
    lyricContent.value = result.content
    lyricLines.value = parseLyric(result.content)
    if (!result.lyricPath && result.content && playlist.value[currentIndex.value]) {
      // lyricPath 为空说明是内嵌歌词
      embeddedLyricPaths.value.add(playlist.value[currentIndex.value].path)
      // lyricPath 为空说明是内嵌歌词
      embeddedLyricPaths.value.add(playlist.value[currentIndex.value].path)
    }
    // 如果下载了新歌词，更新播放列表
    if (result.lyricPath && !lyricPath && playlist.value[currentIndex.value]) {
      playlist.value[currentIndex.value].lyricPath = result.lyricPath
      console.log('歌词已更新:', result.lyricPath)
    }
  } catch (e) {
    console.error('加载歌词失败:', e)
    lyricContent.value = ''
    lyricLines.value = []
  }
}

// 当前歌词行索引（二分查找）
const currentLyricIndex = computed(() => {
  if (lyricLines.value.length === 0) return -1
  const t = currentTime.value
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

// 显示的9行歌词
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

function togglePlay() {
  if (!currentTrack.value.path) return
  if (isPlaying.value) {
    audio.pause()
  } else {
    audio.play()
  }
  isPlaying.value = !isPlaying.value
}

function prevTrack() {
  if (playlist.value.length === 0) return
  currentIndex.value = (currentIndex.value - 1 + playlist.value.length) % playlist.value.length
  loadTrack(currentIndex.value)
}

function nextTrack() {
  if (playlist.value.length === 0) return
  if (playMode.value === 'loop') {
    audio.currentTime = 0
    audio.play()
    return
  }
  if (playMode.value === 'random') {
    const rand = Math.floor(Math.random() * playlist.value.length)
    currentIndex.value = rand
  } else {
    currentIndex.value = (currentIndex.value + 1) % playlist.value.length
  }
  loadTrack(currentIndex.value)
}

function togglePlayMode() {
  const modes: Array<'sequence' | 'loop' | 'random'> = ['sequence', 'loop', 'random']
  const idx = modes.indexOf(playMode.value)
  playMode.value = modes[(idx + 1) % modes.length]
}

function loadTrack(index: number) {
  const track = playlist.value[index]
  if (!track || !streamUrl.value) return
  const src = `${streamUrl.value}?path=${encodeURIComponent(track.path)}`
  console.log('Loading audio:', src)
  audio.src = src
  audio.load()
  currentTime.value = 0
  duration.value = 0
  // 清空歌词，等待音频加载完成后再下载
  lyricContent.value = ''
  lyricLines.value = []
  // 背景图片
  if (track.backgroundPath) {
    backgroundUrl.value = `${streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(track.backgroundPath)}`
  } else {
    backgroundUrl.value = ''
  }
  // 封面图片
  if (track.coverPath) {
    coverUrl.value = `${streamUrl.value.replace('/stream', '/image')}?path=${encodeURIComponent(track.coverPath)}`
  } else {
    coverUrl.value = ''
  }
  if (isPlaying.value) {
    audio.play().catch((e: Error) => console.error('play error:', e))
  }
}

function selectTrack(index: number) {
  currentIndex.value = index
  loadTrack(index)
  isPlaying.value = true
  showPlaylist.value = false
  audio.play().catch((e: Error) => console.error('play error:', e))
}


function seekProgress(e: MouseEvent) {
  if (!duration.value) return
  const bar = (e.currentTarget as HTMLElement)
  const rect = bar.getBoundingClientRect()
  const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  const targetTime = ratio * duration.value
  // 立即更新 UI（视觉欺骗）
  currentTime.value = targetTime
  // 设置音频时间
  audio.currentTime = targetTime
}

function changeVolume(e: Event) {
  const input = e.target as HTMLInputElement
  volume.value = parseInt(input.value)
  audio.volume = volume.value / 100
}

function setupAnalyser() {
  if (analyserNode) return
  try {
    audioContext = new AudioContext()
    analyserNode = audioContext.createAnalyser()
    analyserNode.fftSize = 128
    analyserNode.smoothingTimeConstant = 0.3
    frequencyData = new Uint8Array(analyserNode.frequencyBinCount)
    const source = audioContext.createMediaElementSource(audio)
    source.connect(analyserNode)
    analyserNode.connect(audioContext.destination)
  } catch (e) {
    console.error('AnalyserNode setup failed:', e)
  }
}

function runVisualizer() {
  if (analyserNode && frequencyData) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    analyserNode.getByteFrequencyData(frequencyData as any)
    // 取前40个频段
    visualizerBars.value = Array.from(frequencyData).slice(0, 40)
  }
  visualizerRaf = requestAnimationFrame(runVisualizer)
}

function startVisualizer() {
  if (visualizerRaf !== null) return
  setupAnalyser()
  if (audioContext && audioContext.state === 'suspended') {
    audioContext.resume()
  }
  runVisualizer()
}

function stopVisualizer() {
  if (visualizerRaf !== null) {
    cancelAnimationFrame(visualizerRaf)
    visualizerRaf = null
  }
  visualizerBars.value = Array(40).fill(0)
}

function togglePlaylist() {
  showPlaylist.value = !showPlaylist.value
}

async function loadPlaylist() {
  try {
    // 停止当前播放
    audio.pause()
    audio.src = ''
    isPlaying.value = false
    currentTime.value = 0
    duration.value = 0
    lyricContent.value = ''
    lyricLines.value = []

    // 并行：停止旧服务器 + 读取目录
    const [, dir] = await Promise.all([
      StopAudioServer(),
      getMusicDir()
    ])

    if (!dir) {
      showSetupPrompt.value = true
      playlist.value = []
      currentIndex.value = 0
      return
    }

    // 文件夹没变且已有播放列表，跳过重载
    if (dir === musicDir.value && playlist.value.length > 0) {
      showSetupPrompt.value = false
      return
    }

    musicDir.value = dir
    showSetupPrompt.value = false

    // 启动音频HTTP服务器
    const url = await StartAudioServer()
    streamUrl.value = url
    console.log('Audio server:', url)

    // 加载音乐列表
    const files = await GetMusicFiles(dir)
    playlist.value = files
    if (files.length > 0) {
      currentIndex.value = 0
      loadTrack(0)
    } else {
      showSetupPrompt.value = true
    }
  } catch (e) {
    console.error('初始化失败:', e)
  }
}

async function selectMusicFolder() {
  try {
    const path = await SelectMusicFolder()
    if (!path) return

    // 停止当前播放
    audio.pause()
    audio.src = ''
    isPlaying.value = false
    currentTime.value = 0
    duration.value = 0
    lyricContent.value = ''
    lyricLines.value = []

    // 保存到数据库并停止旧服务器
    await Promise.all([
      setMusicDir(path),
      StopAudioServer()
    ])

    musicDir.value = path
    showSetupPrompt.value = false

    // 启动音频HTTP服务器
    const url = await StartAudioServer()
    streamUrl.value = url
    console.log('Audio server:', url)

    // 加载音乐列表
    const files = await GetMusicFiles(path)
    playlist.value = files
    if (files.length > 0) {
      currentIndex.value = 0
      loadTrack(0)
    } else {
      showSetupPrompt.value = true
    }
  } catch (e) {
    console.error('选择文件夹失败:', e)
  }
}

// 音频事件
audio.addEventListener('loadedmetadata', () => {
  duration.value = audio.duration
  console.log('Audio loaded, duration:', audio.duration)
  // 更新当前歌曲的缓存时长
  if (playlist.value[currentIndex.value]) {
    ;(playlist.value[currentIndex.value] as any).duration = audio.duration
    // 下载歌词（使用真实时长）
    const track = playlist.value[currentIndex.value]
    loadLyric(track.lyricPath, track.artist, track.title, track.path, musicDir.value, audio.duration)
  }
})

audio.addEventListener('timeupdate', () => {
  currentTime.value = audio.currentTime
})

audio.addEventListener('ended', () => {
  nextTrack()
})

audio.addEventListener('play', () => {
  console.log('Audio playing')
  startVisualizer()
})

audio.addEventListener('pause', () => {
  stopVisualizer()
})

audio.addEventListener('error', (e) => {
  // 忽略 src 为空的初始状态错误
  if (!audio.src || audio.src === window.location.href) {
    return
  }
  console.error('Audio error:', e, 'src:', audio.src, 'error code:', audio.error?.code, audio.error?.message)
  showToastMsg('音频加载失败，请尝试播放其他歌曲', 'error')
})

// 点击外部关闭弹窗
function handleClickOutside(e: MouseEvent) {
  const volumeWrap = document.querySelector('.volume-wrap')
  if (volumeWrap && !volumeWrap.contains(e.target as Node)) {
    showVolume.value = false
  }
  const menuPanel = document.querySelector('.menu-panel')
  if (menuPanel && !menuPanel.contains(e.target as Node) && !(e.target as Element).closest('.menu-btn')) {
    showMenu.value = false
  }
}

// 键盘控制
function handleKeydown(e: KeyboardEvent) {
  // 如果焦点在输入框中，不触发快捷键
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
      audio.currentTime = Math.max(0, audio.currentTime - 10)
      break
    case 'ArrowRight':
      e.preventDefault()
      audio.currentTime = Math.min(duration.value, audio.currentTime + 10)
      break
    case 'KeyM':
      e.preventDefault()
      volume.value = volume.value > 0 ? 0 : 70
      audio.volume = volume.value / 100
      break
  }
}

onMounted(async () => {
  // 提前初始化音频分析图（必须在音频播放前完成）
  setupAnalyser()
  // 加载背景颜色
  const savedColor = await getBgColor()
  if (savedColor) {
    bgColor.value = savedColor
  }
  // 加载背景图片
  const savedImage = await getBgImage()
  if (savedImage) {
    bgImage.value = savedImage
  }
  loadPlaylist()
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  stopVisualizer()
  audio.pause()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('click', handleClickOutside)
  StopAudioServer().catch(() => {})
})
</script>

<template>
  <div class="player-container">
    <!-- 自定义标题栏 -->
    <div class="custom-titlebar" style="--wails-draggable: drag;">
      <!-- 迷你模式：只显示图标 + 播放/暂停 + 退出 -->
      <div v-if="isMiniMode" class="mini-mode-content">
        <img :src="iconUrl" alt="" class="titlebar-icon" />
        <div class="mini-controls">
          <button class="mini-btn" @click.stop="prevTrack" style="--wails-draggable: no-drag;">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>
          </button>
          <button class="mini-btn" @click.stop="togglePlay" style="--wails-draggable: no-drag;">
            <svg v-if="!isPlaying" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
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
          <img :src="iconUrl" alt="" class="titlebar-icon" />
          <span>{{ isSmallScreen ? trackTitle : titlebarTitle }}</span>
        </div>
        <div class="titlebar-controls" style="--wails-draggable: no-drag;">
          <button class="titlebar-btn menu-btn" @click.stop="toggleMenu" style="--wails-draggable: no-drag;" title="菜单">☰</button>
          <!-- 菜单面板 -->
          <MenuPanel
            v-if="showMenu"
            :bgColor="bgColor"
            :isSmallScreen="isSmallScreen"
            :isMiniMode="isMiniMode"
            @selectFolder="selectMusicFolder(); showMenu = false"
            @selectBgImage="saveBgImage(); showMenu = false"
            @clearCache="clearCache(); showMenu = false"
            @toggleSmallScreen="toggleSmallScreen(); showMenu = false"
            @toggleMiniMode="toggleMiniMode(); showMenu = false"
            @update:bgColor="updateBgColor"
          />
          <button class="titlebar-btn minimize" @click="minimize" style="--wails-draggable: no-drag;">─</button>
          <button class="titlebar-btn close" @click="closeWindow" style="--wails-draggable: no-drag;">✕</button>
        </div>
      </template>
    </div>

    <!-- 迷你模式主内容区：只显示当前歌词 -->
    <div v-if="isMiniMode" class="mini-main-content">
      <span class="mini-lyric-text">{{ lyricLines[currentLyricIndex]?.text || '暂无歌词' }}</span>
    </div>

    <!-- 背景装饰 -->
    <div class="bg-glow bg-glow-1" v-if="!isMiniMode"></div>
    <div class="bg-glow bg-glow-2" v-if="!isMiniMode"></div>
    <div class="bg-glow bg-glow-3" v-if="!isMiniMode"></div>
    <!-- 歌曲背景图片 -->
    <img v-if="backgroundUrl && !isMiniMode" :src="backgroundUrl" class="background-image" alt="" />
    <img v-if="bgImage && !isMiniMode" :src="bgImage" class="custom-bg-image" alt="" />

    <!-- 设置音乐文件夹提示 -->
    <div v-if="showSetupPrompt && !isMiniMode" class="setup-prompt" @click="selectMusicFolder">
      <div class="setup-content">
        <div class="setup-icon">&#127925;</div>
        <h2>单击添加你的音乐文件夹</h2>
      </div>
    </div>

    <!-- Toast 提示 -->
    <div v-if="showToast && !isMiniMode" class="toast" :class="toastType">
      {{ toastMessage }}
    </div>

    <!-- 主内容区：左侧封面 + 右侧信息 -->
    <div class="main-content" :class="{ 'small-screen': isSmallScreen }" v-if="!showSetupPrompt && !isMiniMode">
      <!-- 左侧专辑封面 -->
      <div class="album-wrapper" v-if="!isSmallScreen">
        <div class="album-glow"></div>
        <div class="album-cover" :class="{ spinning: isPlaying }">
          <img
            :src="coverUrl || '/default_cover.jpeg'"
            :alt="trackTitle"
            @error="($event.target as HTMLImageElement).src = '/default_cover.jpeg'"
          />
        </div>
      </div>

      <!-- 右侧信息 -->
      <div class="right-content" :class="{ 'small-screen': isSmallScreen, 'centered': isSmallScreen }">
        <!-- 歌曲信息 -->
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

        <!-- 可视化 -->
        <div class="visualizer">
          <div
            v-for="(h, i) in visualizerBars"
            :key="i"
            class="viz-bar"
            :style="{ height: isPlaying ? Math.min(80, Math.max(4, h * 0.4)) + 'px' : '4px' }"
          ></div>
        </div>
      </div>
    </div>

    <!-- 底部控制栏 -->
    <div class="controls-bar" :class="{ 'small-screen': isSmallScreen }" v-if="!isMiniMode">
      <!-- 大屏模式：进度条在中间 -->
      <template v-if="!isSmallScreen">
        <!-- 左侧播放控制 -->
        <div class="ctrl-group">
          <button class="ctrl-btn" @click="prevTrack">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>
          </button>
          <button class="ctrl-btn play-btn" @click="togglePlay">
            <svg v-if="!isPlaying" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
          </button>
          <button class="ctrl-btn" @click="nextTrack">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
          </button>
        </div>

        <!-- 进度条 -->
        <div class="progress-section">
          <div class="progress-bar" @click="seekProgress">
            <div class="progress-fill" :style="{ width: (currentTime / duration * 100) + '%' }"></div>
            <div class="progress-thumb" :style="{ left: (currentTime / duration * 100) + '%' }"></div>
            <div
              v-for="i in Math.floor((duration || 0) / 60)"
              :key="i"
              class="progress-tick"
              :style="{ left: (i * 60 / (duration || 1) * 100) + '%' }"
            ></div>
          </div>
          <span class="time-label">{{ formatTime(currentTime) }} / {{ formatTime(duration || 0) }}</span>
        </div>

        <!-- 右侧按钮组 -->
        <div class="ctrl-group-right">
          <!-- 音量图标 -->
          <div class="volume-wrap" @click.stop>
            <button class="icon-btn" @click="showVolume = !showVolume">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path v-if="volumeIcon === 'vol-mute'" d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
                <path v-else-if="volumeIcon === 'vol-low'" d="M7 9v6h4l5 5V4l-5 5H7z"/>
                <path v-else d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
              </svg>
            </button>
            <!-- 垂直音量滑块 -->
            <div class="volume-popup" v-if="showVolume">
              <div class="vol-track" @click="changeVolume">
                <div class="vol-fill" :style="{ height: volume + '%' }"></div>
              </div>
              <input type="range" class="vol-slider" min="0" max="100" :value="volume" @input="changeVolume" orient="vertical" />
            </div>
          </div>

          <!-- 循环模式按钮 -->
          <button class="icon-btn mode-btn" @click="togglePlayMode" :title="playModeTitle">
            <svg v-if="playMode === 'sequence'" viewBox="0 0 24 24" fill="currentColor"><path d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z"/></svg>
            <svg v-else-if="playMode === 'loop'" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M10.59 9.17L5.41 4 4 5.41l5.17 5.17 1.42-1.41zM14.5 4l2.04 2.04L4 18.59 5.41 20 17.96 7.46 20 9.5V4h-5.5zm.33 9.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04-3.13-3.13z"/></svg>
          </button>
          <button class="icon-btn" :class="{ active: showPlaylist }" @click="togglePlaylist" title="播放列表">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M15 6H3v2h12V6zm0 4H3v2h12v-2zM3 16h8v-2H3v2zM17 6v6.18c-.31-.11-.65-.18-1-.18-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3V8h3V6h-5z"/></svg>
          </button>
        </div>
      </template>

      <!-- 小屏模式：进度条在上，按钮在下一行 -->
      <template v-else>
        <!-- 进度条 -->
        <div class="progress-section small-screen-progress">
          <div class="progress-bar" @click="seekProgress">
            <div class="progress-fill" :style="{ width: (currentTime / duration * 100) + '%' }"></div>
            <div class="progress-thumb" :style="{ left: (currentTime / duration * 100) + '%' }"></div>
            <div
              v-for="i in Math.floor((duration || 0) / 60)"
              :key="i"
              class="progress-tick"
              :style="{ left: (i * 60 / (duration || 1) * 100) + '%' }"
            ></div>
          </div>
          <span class="time-label">{{ formatTime(currentTime) }} / {{ formatTime(duration || 0) }}</span>
        </div>

        <!-- 控制按钮行（全部6个按钮一行） -->
        <div class="ctrl-row">
          <!-- 左侧播放控制 -->
          <div class="ctrl-group">
            <button class="ctrl-btn" @click="prevTrack">
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>
            </button>
            <button class="ctrl-btn play-btn" @click="togglePlay">
              <svg v-if="!isPlaying" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
              <svg v-else viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
            </button>
            <button class="ctrl-btn" @click="nextTrack">
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
            </button>
          </div>

          <!-- 右侧按钮组 -->
          <div class="ctrl-group-right">
            <!-- 音量图标 -->
            <div class="volume-wrap" @click.stop>
              <button class="icon-btn" @click="showVolume = !showVolume">
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path v-if="volumeIcon === 'vol-mute'" d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
                  <path v-else-if="volumeIcon === 'vol-low'" d="M7 9v6h4l5 5V4l-5 5H7z"/>
                  <path v-else d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
                </svg>
              </button>
            </div>

            <!-- 循环模式按钮 -->
            <button class="icon-btn mode-btn" @click="togglePlayMode" :title="playModeTitle">
              <svg v-if="playMode === 'sequence'" viewBox="0 0 24 24" fill="currentColor"><path d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z"/></svg>
              <svg v-else-if="playMode === 'loop'" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/></svg>
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
        v-if="showPlaylist && !isMiniMode"
        :playlist="playlist"
        :currentIndex="currentIndex"
        :isPlaying="isPlaying"
        :embeddedLyricPaths="Array.from(embeddedLyricPaths)"
        :isSmallScreen="isSmallScreen"
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
  backdrop-filter: blur(10px);
  flex-shrink: 0;
  user-select: none;
  cursor: move;
}

.titlebar-drag {
  flex: 1;
  height: 100%;
  display: flex;
  align-items: center;
  padding-left: 16px;
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
}

.titlebar-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.titlebar-controls {
  display: flex;
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
  transition: all 0.15s;
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
  padding: 0 16px;
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
  backdrop-filter: blur(10px);
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
  background: v-bind(bgColor);
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
  color: #e0e6f0;
  font-family: 'Microsoft YaHei', 'PingFang SC', -apple-system, sans-serif;
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
  filter: blur(80px);
  opacity: 0.4;
  pointer-events: none;
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

/* 主内容区：左右布局，填充剩余空间 */
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
  filter: blur(30px);
  animation: pulse-glow 3s ease-in-out infinite;
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
  box-shadow:
    0 0 0 4px rgba(255, 255, 255, 0.1),
    0 0 60px rgba(180, 100, 200, 0.4),
    0 30px 80px rgba(0, 0, 0, 0.5);
  position: relative;
  z-index: 1;
  opacity: 0.6;
  margin-left: 0;
}

.album-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.album-cover.spinning {
  animation: spin 30s linear infinite;
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
  margin-top: -100px;
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
  color: #ffffff;
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
  color: rgba(255, 255, 255, 0.3);
  letter-spacing: 1px;
  transition: all 0.3s ease;
  white-space: nowrap;
  opacity: 0.4;
  line-height: 1.2;
}

.lyric-line.active {
  color: rgba(255, 255, 255, 0.95);
  font-size: 24px;
  opacity: 1;
  text-shadow: 0 0 20px rgba(100, 180, 255, 0.6);
}

/* 可视化 */
.visualizer {
  display: flex;
  align-items: flex-end;
  justify-content: flex-start;
  gap: 3px;
  width: 500px;
}

.viz-bar {
  width: 4px;
  background: linear-gradient(to top, rgba(100, 160, 255, 0.8), rgba(150, 200, 255, 0.6));
  border-radius: 2px;
  transition: height 0.1s ease;
  box-shadow: 0 0 6px rgba(100, 160, 255, 0.3);
  flex-shrink: 0;
}

/* 小屏模式：进度条单独一行 */
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
  background: linear-gradient(90deg, #4a90d9 0%, #74b9ff 100%);
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
}

/* 小屏模式：上下结构 */
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
  color: rgba(255, 255, 255, 0.6);
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
  color: #ffffff;
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

/* 进度条 */
.progress-section {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
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
  background: linear-gradient(90deg, #4a90d9 0%, #74b9ff 100%);
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
  backdrop-filter: blur(10px);
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
  color: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  transition: all 0.2s;
  flex-shrink: 0;
}

.icon-btn:hover {
  color: rgba(255, 255, 255, 0.8);
}

.icon-btn.active {
  color: #74b9ff;
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
