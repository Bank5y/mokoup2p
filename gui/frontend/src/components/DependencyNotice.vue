<script setup>
import { onMounted, ref } from 'vue'
import { dependencyMissing, openURL } from '../backend.js'
import { t } from '../i18n/index.js'

// Npcap cannot ship with mokoup2p (ADR-0004); say so before joining fails.
const missing = ref(false)

onMounted(async () => {
  try {
    missing.value = await dependencyMissing()
  } catch {
    missing.value = false
  }
})
</script>

<template>
  <div v-if="missing" class="update-banner dependency-notice" role="alert">
    <span class="update-banner-grow">{{ t('dependency.npcapMissing') }}</span>
    <button class="button button-small button-primary" type="button" @click="openURL('https://npcap.com/#download')">
      {{ t('dependency.download') }}
    </button>
  </div>
</template>
