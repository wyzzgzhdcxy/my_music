<script lang="ts" setup>
import { ref, onMounted, nextTick } from 'vue'

const props = defineProps<{
  thumbnail?: string
  isFullscreen?: boolean
}>()

const emit = defineEmits<{
  'loadedmetadata': [duration: number]
  'timeupdate': [time: number]
  'ended': []
  'play': []
  'pause': []
  'error': [msg: string]
  'dblclick-fullscreen': []
}>()

const video = ref<HTMLVideoElement | null>(null)
const isVideoLoading = ref(false)
const showOverlay = ref(false)
let isIntentionalClear = false

function setupVideoListeners() {
  if (!video.value) return
  const v = video.value

  v.addEventListener('loadedmetadata', () => {
    emit('loadedmetadata', v.duration)
  })

  v.addEventListener('playing', () => {
    // 视频真正开始播放后才隐藏封面
    showOverlay.value = false
  })

  v.addEventListener('timeupdate', () => {
    emit('timeupdate', v.currentTime)
  })

  v.addEventListener('ended', () => {
    emit('ended')
  })

  v.addEventListener('play', () => {
    emit('play')
  })

  v.addEventListener('pause', () => {
    emit('pause')
  })

  v.addEventListener('error', (e) => {
    if (!v.src || isVideoLoading.value || isIntentionalClear) {
      isVideoLoading.value = false
      isIntentionalClear = false
      return
    }
    isVideoLoading.value = false
    emit('error', `Video error: ${e}`)
  })
}

function loadSrc(src: string, thumbnail?: string) {
  if (!video.value) return
  isVideoLoading.value = true
  showOverlay.value = !!thumbnail
  video.value.src = src
  video.value.load()
}

function play() {
  video.value?.play().catch((e: Error) => console.error('play error:', e))
}

function pause() {
  video.value?.pause()
}

function stopAndClear() {
  if (video.value) {
    isIntentionalClear = true
    video.value.pause()
    video.value.src = ''
  }
  isVideoLoading.value = false
  showOverlay.value = false
}

function togglePlay() {
  if (video.value?.paused) {
    play()
  } else {
    pause()
  }
}

function seek(time: number) {
  if (video.value) {
    video.value.currentTime = time
  }
}

function setVolume(vol: number) {
  if (video.value) {
    video.value.volume = vol / 100
  }
}

function toggleFullscreen() {
  emit('dblclick-fullscreen')
}

defineExpose({
  video,
  loadSrc,
  play,
  pause,
  stopAndClear,
  togglePlay,
  seek,
  setVolume,
  isVideoLoading
})

onMounted(async () => {
  await nextTick()
  setupVideoListeners()
})
</script>

<template>
  <div class="video-container" @dblclick="toggleFullscreen" @click.stop>
    <!-- 封面图（叠加在视频上，提供视觉过渡） -->
    <img
      v-if="showOverlay && thumbnail"
      :src="thumbnail"
      class="video-poster"
      alt=""
    />
    <video
      ref="video"
      class="video-player"
      :class="{ 'fullscreen': isFullscreen }"
      @click="togglePlay"
    ></video>
  </div>
</template>

<style scoped>
.video-container {
  position: absolute;
  top: 30px;
  left: 0;
  right: 0;
  bottom: 0;
  background: #000;
  overflow: hidden;
  z-index: 1;
}

.video-player {
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
  background: #000;
}

.video-player.fullscreen {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 9999;
}

.video-poster {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
  z-index: 1;
  pointer-events: none;
  transition: opacity 0.2s ease-out;
}
</style>
