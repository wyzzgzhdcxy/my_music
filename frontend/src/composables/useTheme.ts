import { ref } from 'vue'
import { SaveBgImage, SaveSettings, GetSettings } from '../../wailsjs/go/main/App'

export function useTheme() {
  // Background
  const bgColor = ref('#000000')
  const bgImage = ref('')

  // Build background image URL from audio stream URL
  function buildBgImageUrl(streamUrl: string): string {
    if (!streamUrl) return ''
    return streamUrl.replace('/stream', '/theme/bg') + '?t=' + Date.now()
  }

  // Theme colors
  const btnColor = ref('#ffffff')
  const vizColor = ref('#74b9ff')
  const lyricColor = ref('#ffffff')
  const titleColor = ref('#ffffff')
  const titlebarColor = ref('rgba(255, 255, 255, 0.3)')
  const lyricsColor = ref('rgba(255, 255, 255, 0.3)')

  // Save all settings to file
  async function saveSettings() {
    const settings = {
      bgColor: bgColor.value,
      btnColor: btnColor.value,
      vizColor: vizColor.value,
      lyricColor: lyricColor.value,
      titleColor: titleColor.value,
      titlebarColor: titlebarColor.value,
      lyricsColor: lyricsColor.value
    }
    await SaveSettings(JSON.stringify(settings))
  }

  // Update methods
  function updateBgColor(color: string) {
    bgColor.value = color
    saveSettings()
  }

  function updateBtnColor(color: string) {
    btnColor.value = color
    saveSettings()
  }

  function updateVizColor(color: string) {
    vizColor.value = color
    saveSettings()
  }

  function updateLyricColor(color: string) {
    lyricColor.value = color
    saveSettings()
  }

  function updateTitleColor(color: string) {
    titleColor.value = color
    saveSettings()
  }

  function updateTitlebarColor(color: string) {
    titlebarColor.value = color
    saveSettings()
  }

  function updateLyricsColor(color: string) {
    lyricsColor.value = color
    saveSettings()
  }

  async function saveBgImage(streamUrl: string): Promise<boolean> {
    return new Promise((resolve) => {
      const input = document.createElement('input')
      input.type = 'file'
      input.accept = 'image/*'
      input.onchange = async (e) => {
        const file = (e.target as HTMLInputElement).files?.[0]
        if (!file) {
          resolve(false)
          return
        }
        const reader = new FileReader()
        reader.onload = async (ev) => {
          const dataUrl = ev.target?.result as string
          const hasBg = await SaveBgImage(dataUrl)
          if (hasBg && streamUrl) {
            bgImage.value = buildBgImageUrl(streamUrl)
          }
          resolve(hasBg)
        }
        reader.readAsDataURL(file)
      }
      input.click()
    })
  }

  async function initTheme(): Promise<void> {
    // Load settings from file
    const savedSettings = await GetSettings()
    if (savedSettings) {
      const settings = JSON.parse(savedSettings)
      if (settings.bgColor) bgColor.value = settings.bgColor
      if (settings.btnColor) btnColor.value = settings.btnColor
      if (settings.vizColor) vizColor.value = settings.vizColor
      if (settings.lyricColor) lyricColor.value = settings.lyricColor
      if (settings.titleColor) titleColor.value = settings.titleColor
      if (settings.titlebarColor) titlebarColor.value = settings.titlebarColor
      if (settings.lyricsColor) lyricsColor.value = settings.lyricsColor
    }
    // Background image will be loaded after audio server starts
  }

  function resetTheme() {
    bgColor.value = '#000000'
    bgImage.value = ''
    btnColor.value = '#ffffff'
    vizColor.value = '#74b9ff'
    lyricColor.value = '#ffffff'
    titleColor.value = '#ffffff'
    titlebarColor.value = 'rgba(255, 255, 255, 0.3)'
    lyricsColor.value = 'rgba(255, 255, 255, 0.3)'
    saveSettings()
  }

  return {
    bgColor,
    bgImage,
    btnColor,
    vizColor,
    lyricColor,
    titleColor,
    titlebarColor,
    lyricsColor,
    updateBgColor,
    updateBtnColor,
    updateVizColor,
    updateLyricColor,
    updateTitleColor,
    updateTitlebarColor,
    updateLyricsColor,
    saveBgImage,
    initTheme,
    resetTheme
  }
}