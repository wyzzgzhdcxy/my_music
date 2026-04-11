import { ref, computed } from 'vue'
import { getPlayModeDB, setPlayModeDB } from './useStorage'

export interface MediaFile {
  path: string
  name: string
  isVideo?: boolean
  lyricPath?: string
  backgroundPath?: string
  coverPath?: string
  thumbnailPath?: string
  artist?: string
  title?: string
  resolution?: string
  [key: string]: any
}

export function usePlaylist() {
  // Playlist state
  const playlist = ref<MediaFile[]>([])
  const currentIndex = ref(0)

  // 已发现有内嵌歌词的曲目路径集合
  const embeddedLyricPaths = ref(new Set<string>())

  // Play mode
  const playMode = ref<'sequence' | 'loop' | 'random'>('sequence')
  const playModeTitle = computed(() => {
    return playMode.value === 'sequence' ? '顺序播放' : playMode.value === 'loop' ? '单曲循环' : '随机播放'
  })

  // Video resolutions cache (path -> resolution)
  const videoResolutions = ref<Record<string, string>>({})

  // 根据模式过滤的播放列表
  const filteredPlaylist = computed(() => {
    return playlist.value.filter(item => item.isVideo === isVideoMode.value)
  })

  // 当前模式下的播放索引
  const currentFilteredIndex = computed(() => {
    if (!filteredPlaylist.value.length) return -1
    const currentPath = playlist.value[currentIndex.value]?.path
    return filteredPlaylist.value.findIndex(item => item.path === currentPath)
  })

  // 当前曲目（根据模式过滤后的）
  const currentTrack = computed(() => {
    const filtered = filteredPlaylist.value
    const idx = currentFilteredIndex.value
    return idx >= 0 ? filtered[idx] : { name: '未选择', path: '', size: '', isVideo: false }
  })

  // Video mode flag (injected from App.vue)
  let isVideoMode = ref(false)

  function setIsVideoMode(val: boolean) {
    isVideoMode.value = val
  }

  // 合并分辨率到播放列表项
  function mergeResolutionsIntoPlaylist(files: MediaFile[]): MediaFile[] {
    return files.map(file => {
      if (file.isVideo && videoResolutions.value[file.path]) {
        return { ...file, resolution: videoResolutions.value[file.path] }
      }
      return file
    })
  }

  function selectTrack(index: number) {
    currentIndex.value = index
    return index
  }

  function prevTrack(): number {
    const filtered = filteredPlaylist.value
    if (filtered.length === 0) return -1
    const idx = currentFilteredIndex.value
    const newIdx = (idx - 1 + filtered.length) % filtered.length
    const newTrack = filtered[newIdx]
    const actualIndex = playlist.value.findIndex(item => item.path === newTrack.path)
    if (actualIndex >= 0) {
      currentIndex.value = actualIndex
      return actualIndex
    }
    return -1
  }

  function nextTrack(currentTrackItem: MediaFile | undefined): number {
    const filtered = filteredPlaylist.value
    if (filtered.length === 0) return -1

    if (playMode.value === 'loop' && currentTrackItem) {
      // 单曲循环时返回当前索引（不切换曲目）
      return currentIndex.value
    }

    const idx = currentFilteredIndex.value
    let newIdx: number
    if (playMode.value === 'random') {
      newIdx = Math.floor(Math.random() * filtered.length)
    } else {
      newIdx = (idx + 1) % filtered.length
    }
    const newTrack = filtered[newIdx]
    const actualIndex = playlist.value.findIndex(item => item.path === newTrack.path)
    if (actualIndex >= 0) {
      currentIndex.value = actualIndex
      return actualIndex
    }
    return -1
  }

  function togglePlayMode() {
    const modes: Array<'sequence' | 'loop' | 'random'> = ['sequence', 'loop', 'random']
    const idx = modes.indexOf(playMode.value)
    playMode.value = modes[(idx + 1) % modes.length]
    setPlayModeDB(playMode.value)
  }

  async function initPlayMode() {
    const saved = await getPlayModeDB()
    playMode.value = saved
  }

  function setPlaylist(files: MediaFile[], isVideo: boolean) {
    playlist.value = mergeResolutionsIntoPlaylist(files)
    isVideoMode.value = isVideo
  }

  function clearPlaylist() {
    playlist.value = []
    currentIndex.value = 0
  }

  function getTrackByIndex(index: number): MediaFile | undefined {
    return playlist.value[index]
  }

  return {
    playlist,
    currentIndex,
    embeddedLyricPaths,
    playMode,
    playModeTitle,
    videoResolutions,
    filteredPlaylist,
    currentFilteredIndex,
    currentTrack,
    setIsVideoMode,
    mergeResolutionsIntoPlaylist,
    selectTrack,
    prevTrack,
    nextTrack,
    togglePlayMode,
    initPlayMode,
    setPlaylist,
    clearPlaylist,
    getTrackByIndex
  }
}
