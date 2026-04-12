// IndexedDB storage helper
const DB_NAME = 'my_music_db'
const STORE_NAME = 'settings'

const DB_KEY = 'my_music_home'
const DB_KEY_BG_COLOR = 'my_music_bg_color'
const DB_KEY_ALWAYS_ON_TOP = 'my_music_always_on_top'
const DB_KEY_BTN_COLOR = 'my_music_btn_color'
const DB_KEY_VIZ_COLOR = 'my_music_viz_color'
const DB_KEY_LYRIC_COLOR = 'my_music_lyric_color'
const DB_KEY_TITLE_COLOR = 'my_music_title_color'
const DB_KEY_TITLEBAR_COLOR = 'my_music_titlebar_color'
const DB_KEY_LYRICS_COLOR = 'my_music_lyrics_color'
const DB_KEY_PLAY_STATE = 'my_music_play_state'
const DB_KEY_VOLUME = 'my_music_volume'
const DB_KEY_PLAY_MODE = 'my_music_play_mode'
const DB_KEY_VIDEO_RESOLUTIONS = 'my_music_video_resolutions'
const DB_KEY_WINDOW_STATE = 'my_music_window_state'

let dbInstance: IDBDatabase | null = null

export function openDB(): Promise<IDBDatabase> {
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

export async function dbGet<T>(key: string, defaultValue: T): Promise<T> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const request = store.get(key)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve((request.result as T) ?? defaultValue)
  })
}

export async function dbSet<T>(key: string, value: T): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const request = store.put(value, key)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve()
  })
}

export function getCloseDB() {
  return () => {
    if (dbInstance) {
      dbInstance.close()
      dbInstance = null
    }
  }
}

// Music directory
export async function getMusicDir(): Promise<string> {
  return dbGet(DB_KEY, '')
}

export async function setMusicDir(path: string): Promise<void> {
  return dbSet(DB_KEY, path)
}

// Background color
export async function getBgColor(): Promise<string> {
  return dbGet(DB_KEY_BG_COLOR, '')
}

export async function setBgColor(color: string): Promise<void> {
  return dbSet(DB_KEY_BG_COLOR, color)
}

// Always on top
export async function getAlwaysOnTop(): Promise<boolean> {
  return dbGet(DB_KEY_ALWAYS_ON_TOP, false)
}

export async function setAlwaysOnTopState(onTop: boolean): Promise<void> {
  return dbSet(DB_KEY_ALWAYS_ON_TOP, onTop)
}

// Button color
export async function getBtnColor(): Promise<string> {
  return dbGet(DB_KEY_BTN_COLOR, '#ffffff')
}

export async function setBtnColor(color: string): Promise<void> {
  return dbSet(DB_KEY_BTN_COLOR, color)
}

// Visualizer color
export async function getVizColor(): Promise<string> {
  return dbGet(DB_KEY_VIZ_COLOR, '#74b9ff')
}

export async function setVizColor(color: string): Promise<void> {
  return dbSet(DB_KEY_VIZ_COLOR, color)
}

// Lyric color
export async function getLyricColor(): Promise<string> {
  return dbGet(DB_KEY_LYRIC_COLOR, '#ffffff')
}

export async function setLyricColor(color: string): Promise<void> {
  return dbSet(DB_KEY_LYRIC_COLOR, color)
}

// Title color
export async function getTitleColor(): Promise<string> {
  return dbGet(DB_KEY_TITLE_COLOR, '#ffffff')
}

export async function setTitleColor(color: string): Promise<void> {
  return dbSet(DB_KEY_TITLE_COLOR, color)
}

// Titlebar color
export async function getTitlebarColor(): Promise<string> {
  return dbGet(DB_KEY_TITLEBAR_COLOR, 'rgba(255, 255, 255, 0.3)')
}

export async function setTitlebarColor(color: string): Promise<void> {
  return dbSet(DB_KEY_TITLEBAR_COLOR, color)
}

// Lyrics color
export async function getLyricsColor(): Promise<string> {
  return dbGet(DB_KEY_LYRICS_COLOR, 'rgba(255, 255, 255, 0.3)')
}

export async function setLyricsColor(color: string): Promise<void> {
  return dbSet(DB_KEY_LYRICS_COLOR, color)
}

// Play state
interface PlayState {
  musicDir: string
  currentIndex: number
  currentTime: number
  isPlaying: boolean
}

export async function getPlayState(): Promise<PlayState | null> {
  return dbGet(DB_KEY_PLAY_STATE, null)
}

export async function setPlayState(state: PlayState): Promise<void> {
  return dbSet(DB_KEY_PLAY_STATE, state)
}

// Volume
export async function getVolume(): Promise<number> {
  return dbGet(DB_KEY_VOLUME, 100)
}

export async function setVolumeDB(vol: number): Promise<void> {
  return dbSet(DB_KEY_VOLUME, vol)
}

// Play mode
export async function getPlayModeDB(): Promise<'sequence' | 'loop' | 'random'> {
  return dbGet(DB_KEY_PLAY_MODE, 'sequence')
}

export async function setPlayModeDB(mode: 'sequence' | 'loop' | 'random'): Promise<void> {
  return dbSet(DB_KEY_PLAY_MODE, mode)
}

// Video resolutions
export async function getVideoResolutions(): Promise<Record<string, string>> {
  return dbGet(DB_KEY_VIDEO_RESOLUTIONS, {})
}

export async function setVideoResolution(path: string, resolution: string): Promise<void> {
  const resolutions = await getVideoResolutions()
  resolutions[path] = resolution
  return dbSet(DB_KEY_VIDEO_RESOLUTIONS, resolutions)
}

// Window state
interface WindowState {
  x: number
  y: number
  width: number
  height: number
}

export async function getWindowState(): Promise<WindowState | null> {
  return dbGet(DB_KEY_WINDOW_STATE, null)
}

export async function setWindowState(state: WindowState): Promise<void> {
  return dbSet(DB_KEY_WINDOW_STATE, state)
}
