<script lang="ts" setup>
import { ref } from 'vue'

const props = defineProps<{
  bgColor: string
  isSmallScreen: boolean
  isMiniMode: boolean
}>()

const emit = defineEmits<{
  selectFolder: []
  selectBgImage: []
  clearCache: []
  toggleSmallScreen: []
  toggleMiniMode: []
  'update:bgColor': [color: string]
}>()

const colorInputRef = ref<HTMLInputElement | null>(null)

function handleBgColorChange(e: Event) {
  const input = e.target as HTMLInputElement
  emit('update:bgColor', input.value)
}
</script>

<template>
  <div class="menu-panel" @click.stop>
    <div class="menu-item" @click="emit('selectFolder')">
      <span class="menu-icon">&#128193;</span>
      <span>音乐文件夹</span>
    </div>
    <div class="menu-item" @click="emit('selectBgImage')">
      <span class="menu-icon">&#128444;</span>
      <span>背景图片</span>
    </div>
    <div class="menu-item color-picker-item">
      <span class="menu-icon">&#127912;</span>
      <span>背景颜色</span>
      <input
        ref="colorInputRef"
        type="color"
        class="hidden-color-input"
        :value="bgColor"
        @input="handleBgColorChange"
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
    <div class="menu-divider"></div>
    <div class="menu-item" @click="emit('clearCache')">
      <span class="menu-icon">&#128465;</span>
      <span>清空缓存</span>
    </div>
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
  backdrop-filter: blur(10px);
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
