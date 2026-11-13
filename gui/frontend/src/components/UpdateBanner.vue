<script setup>
import { computed, ref } from 'vue'
import { installUpdate, openURL } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import { available, ignore } from '../update.js'

// edition is the daemon status's edition: only the Windows installer
// edition updates itself; the others are pointed at the release page.
const props = defineProps({ edition: { type: String, default: '' } })

const confirming = ref(false)
const busy = ref(false)
const error = ref('')

const oneClick = computed(() => props.edition === 'windows-installer' && !!available.value?.assetUrl)

function update() {
  error.value = ''
  if (oneClick.value) confirming.value = true
  else openURL(available.value.releaseUrl)
}

async function install() {
  busy.value = true
  error.value = ''
  try {
    await installUpdate()
    // The GUI quits now; the installer reopens it when done.
  } catch (err) {
    error.value = errorText(err)
    busy.value = false
    confirming.value = false
  }
}

async function skip() {
  error.value = ''
  try {
    await ignore(available.value.latestVersion)
  } catch (err) {
    error.value = errorText(err)
  }
}
</script>

<template>
  <div v-if="available" class="update-banner" role="status">
    <template v-if="!confirming">
      <span class="update-banner-text">{{ t('update.available', { version: available.latestVersion }) }}</span>
      <a class="update-banner-link" href="#" @click.prevent="openURL(available.releaseUrl)">{{ t('update.notes') }}</a>
      <button class="button button-small button-primary" type="button" @click="update">{{ t('update.update') }}</button>
      <button class="button button-small" type="button" @click="skip">{{ t('update.ignore') }}</button>
    </template>
    <template v-else>
      <span class="update-banner-text update-banner-grow">
        {{ busy ? t('update.installing') : t('update.confirm', { version: available.latestVersion }) }}
      </span>
      <button class="button button-small button-primary" type="button" :disabled="busy" @click="install">{{ t('update.confirmYes') }}</button>
      <button class="button button-small" type="button" :disabled="busy" @click="confirming = false">{{ t('update.confirmNo') }}</button>
    </template>
    <span v-if="error" class="update-banner-error">{{ error }}</span>
  </div>
</template>
