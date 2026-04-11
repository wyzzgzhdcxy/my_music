import { ref } from 'vue'
import { getVolume, setVolumeDB } from './useStorage'

export interface AudioCallbacks {
  onLoadedMetadata?: (duration: number) => void
  onTimeUpdate?: (time: number) => void
  onEnded?: () => void
  onPlay?: () => void
  onPause?: () => void
  onError?: (msg: string) => void
  onDurationChange?: (duration: number) => void
}

export function useAudio(callbacks: AudioCallbacks = {}) {
  // Playback state
  const isPlaying = ref(false)
  const currentTime = ref(0)
  const duration = ref(0)
  const volume = ref(100)
  const isAudioLoading = ref(false)

  // Audio element
  const audio = new Audio()
  audio.crossOrigin = 'anonymous'

  // Stream URL
  const streamUrl = ref('')

  // Last save play state time
  let lastSavePlayState = 0
  const SAVE_PLAY_STATE_INTERVAL = 5000

  // Audio events
  audio.addEventListener('loadedmetadata', () => {
    isAudioLoading.value = false
    duration.value = audio.duration
    callbacks.onLoadedMetadata?.(audio.duration)
  })

  audio.addEventListener('timeupdate', () => {
    currentTime.value = audio.currentTime
    callbacks.onTimeUpdate?.(audio.currentTime)
  })

  audio.addEventListener('ended', () => {
    callbacks.onEnded?.()
  })

  audio.addEventListener('play', () => {
    isPlaying.value = true
    callbacks.onPlay?.()
  })

  audio.addEventListener('pause', () => {
    isPlaying.value = false
    callbacks.onPause?.()
  })

  audio.addEventListener('error', (e) => {
    // 忽略所有错误，loading状态的错误不应该弹出toast
    // 真正的加载失败会在loadedmetadata超时或其他机制处理
    isAudioLoading.value = false
  })

  // Set stream URL
  function setStreamUrl(url: string) {
    streamUrl.value = url
  }

  // Load track
  function loadTrack(trackPath: string) {
    if (!streamUrl.value) return
    isAudioLoading.value = true
    const src = `${streamUrl.value}?path=${encodeURIComponent(trackPath)}`
    audio.src = src
    audio.load()
    currentTime.value = 0
    duration.value = 0
  }

  // Playback controls
  function play() {
    audio.play().catch((e: Error) => console.error('play error:', e))
  }

  function pause() {
    audio.pause()
  }

  function togglePlay() {
    if (isPlaying.value) {
      pause()
    } else {
      play()
    }
  }

  function seek(time: number) {
    audio.currentTime = time
    currentTime.value = time
  }

  function setVolume(vol: number) {
    volume.value = vol
    audio.volume = vol / 100
    setVolumeDB(vol)
  }

  function toggleMute() {
    volume.value = volume.value > 0 ? 0 : 100
    audio.volume = volume.value / 100
    setVolumeDB(volume.value)
  }

  // Initialize volume from storage
  async function initVolume() {
    const saved = await getVolume()
    volume.value = saved
    audio.volume = saved / 100
  }

  // Pause and clear
  function pauseAndClear() {
    audio.pause()
    audio.src = ''
    isPlaying.value = false
    currentTime.value = 0
    duration.value = 0
  }

  function cleanup() {
    pauseAndClear()
  }

  return {
    audio,
    isPlaying,
    currentTime,
    duration,
    volume,
    isAudioLoading,
    streamUrl,
    setStreamUrl,
    loadTrack,
    play,
    pause,
    togglePlay,
    seek,
    setVolume,
    toggleMute,
    initVolume,
    pauseAndClear,
    cleanup
  }
}
