<script setup>
import { onMounted, ref } from 'vue'
import { setSettings, settings } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import { isAdminLost } from './adminErrors.js'

const emit = defineEmits(['status-lost'])

const roomsPerUser = ref(null)
const roomCapacity = ref(null)
const error = ref('')
const message = ref('')
const busy = ref(false)

function fail(err) {
  if (isAdminLost(err)) emit('status-lost')
  error.value = errorText(err)
}

function show(st) {
  roomsPerUser.value = st.roomsPerUser
  roomCapacity.value = st.roomCapacity
}

async function load() {
  try {
    show(await settings())
  } catch (err) {
    fail(err)
  }
}

async function save() {
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    show(await setSettings(Number(roomsPerUser.value), Number(roomCapacity.value)))
    message.value = t('adminSettings.saved')
  } catch (err) {
    fail(err)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="panel-body">
    <h2 class="section-title">{{ t('adminSettings.title') }}</h2>
    <p v-if="roomsPerUser === null && !error" class="field-label">{{ t('admin.loading') }}</p>
    <form v-else class="form" @submit.prevent="save">
      <label class="field">
        <span class="field-label">{{ t('adminSettings.roomsPerUser') }}</span>
        <input v-model="roomsPerUser" class="input" type="number" min="1" max="1000" required />
      </label>
      <label class="field">
        <span class="field-label">{{ t('adminSettings.roomCapacity') }}</span>
        <input v-model="roomCapacity" class="input" type="number" min="1" max="254" required />
      </label>
      <p class="hint hint-left">{{ t('adminSettings.hint') }}</p>
      <p v-if="error" class="notice notice-error">{{ error }}</p>
      <p v-if="message" class="notice notice-ok">{{ message }}</p>
      <div class="actions">
        <button class="button button-primary" type="submit" :disabled="busy">{{ t('adminSettings.save') }}</button>
      </div>
    </form>
  </div>
</template>
