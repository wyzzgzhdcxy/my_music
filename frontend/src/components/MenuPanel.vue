<script lang="ts" setup>
const props = defineProps<{
  bgColor: string
  btnColor: string
  vizColor: string
  lyricColor: string
  titleColor: string
  titlebarColor: string
  lyricsColor: string
  bgImageEnabled: boolean
  isSmallScreen: boolean
  isMiniMode: boolean
  alwaysOnTop: boolean
  isVideoMode: boolean
}>()

const emit = defineEmits<{
  selectFolder: []
  selectBgImage: []
  toggleBgImageEnabled: []
  clearCache: []
  openCacheFolder: []
  toggleSmallScreen: []
  toggleMiniMode: []
  toggleAlwaysOnTop: []
  'update:bgColor': [color: string]
  'update:btnColor': [color: string]
  'update:vizColor': [color: string]
  'update:lyricColor': [color: string]
  'update:titleColor': [color: string]
  'update:titlebarColor': [color: string]
  'update:lyricsColor': [color: string]
}>()

function handleBgColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:bgColor', input.value)
}

function handleBtnColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:btnColor', input.value)
}

function handleVizColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:vizColor', input.value)
}

function handleLyricColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:lyricColor', input.value)
}

function handleTitleColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:titleColor', input.value)
}

function handleTitlebarColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:titlebarColor', input.value)
}

function handleLyricsColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:lyricsColor', input.value)
}
</script>

<template>
  <div class="menu-panel" @click.stop>
    <!-- 视频模式：显示部分菜单 -->
    <template v-if="isVideoMode">
      <div class="menu-item" @click="emit('selectFolder')">
        <span class="menu-icon">&#128193;</span>
        <span>视频音乐</span>
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#10022;</span>
        <span>按钮颜色</span>
        <span class="color-preview" :style="{ backgroundColor: btnColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="btnColor"
          @input="handleBtnColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#128336;</span>
        <span>标题栏歌名颜色</span>
        <span class="color-preview" :style="{ backgroundColor: titlebarColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="titlebarColor"
          @input="handleTitlebarColorChange"
        />
      </div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="emit('toggleAlwaysOnTop')">
        <span class="menu-icon">&#128204;</span>
        <span>{{ alwaysOnTop ? '取消置顶' : '窗口置顶' }}</span>
      </div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="emit('clearCache')">
        <span class="menu-icon">&#128465;</span>
        <span>清空缓存</span>
      </div>
      <div class="menu-item" @click="emit('openCacheFolder')">
        <span class="menu-icon">&#128193;</span>
        <span>打开缓存文件夹</span>
      </div>
    </template>
    <!-- 音乐模式：显示所有菜单 -->
    <template v-else>
      <div class="menu-item" @click="emit('selectFolder')">
        <span class="menu-icon">&#128193;</span>
        <span>视频音乐</span>
      </div>
      <div class="menu-item" @click="emit('selectBgImage')">
        <span class="menu-icon">&#128444;</span>
        <span>背景图片</span>
        <span class="bg-toggle" :class="{ enabled: bgImageEnabled }" @click.stop="emit('toggleBgImageEnabled')"></span>
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#127912;</span>
        <span>背景颜色</span>
        <span class="color-preview" :style="{ backgroundColor: bgColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="bgColor"
          @input="handleBgColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#10022;</span>
        <span>按钮颜色</span>
        <span class="color-preview" :style="{ backgroundColor: btnColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="btnColor"
          @input="handleBtnColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#10697;</span>
        <span>音浪条颜色</span>
        <span class="color-preview" :style="{ backgroundColor: vizColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="vizColor"
          @input="handleVizColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#9835;</span>
        <span>歌词高亮颜色</span>
        <span class="color-preview" :style="{ backgroundColor: lyricColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="lyricColor"
          @input="handleLyricColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#127926;</span>
        <span>歌名颜色</span>
        <span class="color-preview" :style="{ backgroundColor: titleColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="titleColor"
          @input="handleTitleColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#128336;</span>
        <span>标题栏歌名颜色</span>
        <span class="color-preview" :style="{ backgroundColor: titlebarColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="titlebarColor"
          @input="handleTitlebarColorChange"
        />
      </div>
      <div class="menu-item color-picker-item">
        <span class="menu-icon">&#9834;</span>
        <span>歌词颜色</span>
        <span class="color-preview" :style="{ backgroundColor: lyricsColor }"></span>
        <input
          type="color"
          class="hidden-color-input"
          :value="lyricsColor"
          @input="handleLyricsColorChange"
        />
      </div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="emit('toggleSmallScreen')">
        <span class="menu-icon">&#128343;</span>
        <span>{{ isSmallScreen ? '退出小屏' : '小屏模式' }}</span>
      </div>
      <div class="menu-item" v-if="!isSmallScreen" @click="emit('toggleMiniMode')">
        <span class="menu-icon">&#127925;</span>
        <span>{{ isMiniMode ? '退出迷你模式' : '迷你模式' }}</span>
      </div>
      <div class="menu-item" @click="emit('toggleAlwaysOnTop')">
        <span class="menu-icon">&#128204;</span>
        <span>{{ alwaysOnTop ? '取消置顶' : '窗口置顶' }}</span>
      </div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="emit('clearCache')">
        <span class="menu-icon">&#128465;</span>
        <span>清空缓存</span>
      </div>
      <div class="menu-item" @click="emit('openCacheFolder')">
        <span class="menu-icon">&#128193;</span>
        <span>打开缓存文件夹</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.menu-panel {
  position: absolute;
  top: 36px;
  right: 0;
  background: rgba(20, 25, 40, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 6px 0;
  z-index: 1001;
  min-width: 140px;
  max-width: 180px;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 5px 16px;
  cursor: pointer;
  transition: background 0.15s;
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
}

.color-picker-item {
  position: relative;
}

.color-preview {
  width: 14px;
  height: 14px;
  border-radius: 3px;
  margin-left: auto;
  border: 1px solid rgba(255, 255, 255, 0.3);
  flex-shrink: 0;
}

.bg-toggle {
  margin-left: auto;
  width: 36px;
  height: 20px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.2);
  position: relative;
  flex-shrink: 0;
  cursor: pointer;
  transition: background 0.2s;
}

.bg-toggle::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.5);
  transition: transform 0.2s, background 0.2s;
}

.bg-toggle.enabled {
  background: rgba(74, 185, 120, 0.7);
}

.bg-toggle.enabled::after {
  transform: translateX(16px);
  background: #ffffff;
}

.menu-item:hover {
  background: rgba(255, 255, 255, 0.1);
}

.menu-divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.1);
  margin: 4px 0;
}

.menu-icon {
  font-size: 16px;
  width: 20px;
  text-align: center;
}

.hidden-color-input {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}
</style>
