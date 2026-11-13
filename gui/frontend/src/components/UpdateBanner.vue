<script setup>
import { ref } from 'vue'
import { openURL } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import { available, ignore } from '../update.js'

const error = ref('')

function update() {
  openURL(available.value.releaseUrl)
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
    <span class="update-banner-text">{{ t('update.available', { version: available.latestVersion }) }}</span>
    <a class="update-banner-link" href="#" @click.prevent="openURL(available.releaseUrl)">{{ t('update.notes') }}</a>
    <button class="button button-small button-primary" type="button" @click="update">{{ t('update.update') }}</button>
    <button class="button button-small" type="button" @click="skip">{{ t('update.ignore') }}</button>
    <span v-if="error" class="update-banner-error">{{ error }}</span>
  </div>
</template>
