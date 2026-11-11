<script setup>
import { ref } from 'vue'
import { copyText } from '../backend.js'
import { t } from '../i18n/index.js'

const props = defineProps({ code: { type: String, required: true } })

// state is '', 'copied' or 'failed'; it resets after a moment.
const state = ref('')
let timer

async function copy() {
  try {
    await copyText(props.code)
    state.value = 'copied'
  } catch {
    state.value = 'failed'
  }
  clearTimeout(timer)
  timer = setTimeout(() => (state.value = ''), 2000)
}
</script>

<template>
  <span class="room-code">
    <span class="room-code-value">{{ code }}</span>
    <button class="button button-small" type="button" @click="copy">
      {{ state === 'copied' ? t('room.copied') : t('room.copy') }}
    </button>
    <span v-if="state === 'failed'" class="room-code-error">{{ t('errors.copy_failed') }}</span>
  </span>
</template>
