<script lang="ts" setup>
import { ref, computed, watch } from 'vue'

interface Track {
  name: string
  path: string
  size: string
  quality: string
  artist: string
  title: string
  lyricPath: string
  backgroundPath: string
  coverPath: string
  isVideo: boolean
  duration?: number
  resolution?: string
}

const props = defineProps<{
  playlist: Track[]
  currentIndex: number
  isPlaying: boolean
  embeddedLyricPaths?: Set<string>
  isSmallScreen?: boolean
  videoResolutions?: Record<string, string>
}>()

const emit = defineEmits<{
  select: [index: number]
  refresh: []
}>()

// 搜索（带防抖）
const searchQuery = ref('')
const debouncedQuery = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null
const audioExtRegex = /\.[^.]+$/

watch(searchQuery, (val) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    debouncedQuery.value = val
  }, 150)
})

// 过滤后的播放列表
const filteredPlaylist = computed(() => {
  if (!debouncedQuery.value.trim()) return props.playlist
  const keyword = debouncedQuery.value.toLowerCase()
  return props.playlist.filter(file => file.name.toLowerCase().includes(keyword))
})

function selectTrack(index: number) {
  emit('select', index)
}
</script>

<template>
  <div class="playlist-panel" :class="{ 'small-screen': isSmallScreen }">
    <div class="playlist-header">
      <span>播放列表 ({{ playlist.length }})</span>
      <div class="header-actions">
        <input
          type="text"
          class="playlist-search"
          v-model="searchQuery"
          placeholder="搜索歌曲..."
        />
        <button class="refresh-btn" @click="emit('refresh')" title="刷新列表">
          &#128260;
        </button>
      </div>
    </div>
    <div class="playlist-content">
      <div
        v-for="(file, index) in filteredPlaylist"
        :key="file.path"
        class="playlist-item"
        :class="{ playing: index === currentIndex && isPlaying }"
        @click="selectTrack(index)"
      >
        <div class="item-info">
          <span class="item-name">{{ file.name.replace(audioExtRegex, '') }}</span>
          <div class="item-meta" v-if="!isSmallScreen">
            <span class="item-size">{{ file.size }}</span>
            <span class="quality-badge" :class="'quality-' + file.quality">{{ file.quality }}</span>
            <span class="video-badge" v-if="file.isVideo">影</span>
            <span class="resolution-badge" v-if="file.isVideo && (videoResolutions?.[file.path] || file.resolution)">{{ videoResolutions?.[file.path] || file.resolution }}</span>
            <span class="lyric-badge lyric-badge-embed" v-else-if="embeddedLyricPaths?.has(file.path)">嵌</span>
            <span class="lyric-badge" v-else-if="file.lyricPath && !file.isVideo">词</span>
            <span class="cover-badge" v-if="file.coverPath && !file.isVideo">图</span>
          </div>
        </div>
        <div class="playing-indicator" v-if="index === currentIndex && isPlaying">
          <span></span><span></span><span></span>
        </div>
      </div>
      <div v-if="playlist.length === 0" class="empty-tip">
        未找到音视频文件
      </div>
      <div v-else-if="filteredPlaylist.length === 0 && searchQuery" class="empty-tip">
        未找到匹配的歌曲
      </div>
    </div>
  </div>
</template>

<style scoped>
.playlist-panel {
  position: absolute;
  top: 30px;
  left: 0;
  right: 0;
  bottom: 80px;
  background: rgba(10, 15, 30, 0.8);
  backdrop-filter: blur(8px);
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
  z-index: 100;
}

.playlist-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 15px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
  gap: 12px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.refresh-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 5px 8px;
  color: rgba(255, 255, 255, 0.7);
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
}

.refresh-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: rgba(255, 255, 255, 0.9);
}

.playlist-search {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 6px 12px;
  color: rgba(255, 255, 255, 0.9);
  font-size: 13px;
  width: 160px;
  outline: none;
  transition: border-color 0.2s;
}

.playlist-search:focus {
  border-color: rgba(74, 144, 217, 0.6);
}

.playlist-search::placeholder {
  color: rgba(255, 255, 255, 0.4);
}

.playlist-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: grid;
  grid-template-columns: repeat(v-bind(isSmallScreen ? 1 : 2), 1fr);
  gap: 8px;
}

.playlist-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
}

.playlist-item:hover {
  background: rgba(255, 255, 255, 0.08);
}

.playlist-item.playing {
  background: rgba(74, 144, 217, 0.2);
}

.item-info {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}

.item-name {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.8);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 280px;
}

/* 小屏模式：歌曲名最大250px */
.playlist-panel.small-screen .item-name {
  max-width: 250px;
}

.item-size {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.3);
}

.item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.lyric-badge {
  font-size: 10px;
  color: #74b9ff;
  background: rgba(116, 185, 255, 0.15);
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.lyric-badge-embed {
  color: #a8e6cf;
  background: rgba(168, 230, 207, 0.15);
}

.cover-badge {
  font-size: 10px;
  color: #ff9f43;
  background: rgba(255, 159, 67, 0.15);
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.video-badge {
  font-size: 10px;
  color: #fd79a8;
  background: rgba(253, 121, 168, 0.15);
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.resolution-badge {
  font-size: 10px;
  color: #a29bfe;
  background: rgba(162, 155, 254, 0.15);
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.quality-badge {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.quality-badge.quality-流畅 {
  color: #a0a0a0;
  background: rgba(160, 160, 160, 0.15);
}

.quality-badge.quality-高清 {
  color: #74b9ff;
  background: rgba(116, 185, 255, 0.15);
}

.quality-badge.quality-超清 {
  color: #a8e6cf;
  background: rgba(168, 230, 207, 0.15);
}

.quality-badge.quality-无损 {
  color: #ff9f43;
  background: rgba(255, 159, 67, 0.15);
}

.playing-indicator {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 16px;
}

.playing-indicator span {
  width: 3px;
  background: #74b9ff;
  border-radius: 1px;
}

.playing-indicator span:nth-child(1) { height: 6px; }
.playing-indicator span:nth-child(2) { height: 12px; }
.playing-indicator span:nth-child(3) { height: 8px; }

@keyframes blink {
  0%, 100% { opacity: 0.3; }
  50% { opacity: 1; }
}

.empty-tip {
  text-align: center;
  padding: 40px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.3);
}
</style>
