import { ref } from 'vue'

export function useVisualizer() {
  const visualizerBars = ref<number[]>(Array(32).fill(0))

  let audioContext: AudioContext | null = null
  let analyserNode: AnalyserNode | null = null
  let frequencyData: Uint8Array | null = null
  let visualizerRaf: number | null = null

  let lastVizUpdate = 0
  const VIZ_UPDATE_INTERVAL = 50 // 20fps

  function setupAnalyser(audio: HTMLAudioElement) {
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

  function runVisualizer(timestamp: number) {
    if (timestamp - lastVizUpdate >= VIZ_UPDATE_INTERVAL) {
      lastVizUpdate = timestamp
      if (analyserNode && frequencyData) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        analyserNode.getByteFrequencyData(frequencyData as any)
        visualizerBars.value = Array.from(frequencyData).slice(0, 32)
      }
    }
    visualizerRaf = requestAnimationFrame(runVisualizer)
  }

  function startVisualizer() {
    if (visualizerRaf !== null) return
    if (audioContext && audioContext.state === 'suspended') {
      audioContext.resume()
    }
    visualizerRaf = requestAnimationFrame(runVisualizer)
  }

  function stopVisualizer() {
    if (visualizerRaf !== null) {
      cancelAnimationFrame(visualizerRaf)
      visualizerRaf = null
    }
    visualizerBars.value = Array(32).fill(0)
  }

  function cleanup() {
    stopVisualizer()
    if (analyserNode) {
      analyserNode.disconnect()
      analyserNode = null
    }
    if (audioContext) {
      audioContext.close()
      audioContext = null
    }
    visualizerBars.value = Array(32).fill(0)
  }

  return {
    visualizerBars,
    setupAnalyser,
    startVisualizer,
    stopVisualizer,
    cleanup
  }
}
