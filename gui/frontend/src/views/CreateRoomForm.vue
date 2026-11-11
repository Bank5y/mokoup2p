<script setup>
import { ref } from 'vue'
import { createRoom } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import RoomCode from '../components/RoomCode.vue'

const emit = defineEmits(['cancel', 'done'])

const name = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')
const created = ref(null)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    created.value = await createRoom(name.value, password.value)
    password.value = ''
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel">
    <h2 class="panel-title">{{ t('createRoom.title') }}</h2>
    <div v-if="created" class="form">
      <p class="notice">{{ t('createRoom.created') }}</p>
      <div class="field">
        <span class="field-label">{{ t('room.code') }}</span>
        <RoomCode :code="created.code" />
      </div>
      <div class="actions">
        <button class="button button-primary" type="button" @click="emit('done', created)">
          {{ t('createRoom.done') }}
        </button>
      </div>
    </div>
    <form v-else class="form" @submit.prevent="submit">
      <label class="field">
        <span class="field-label">{{ t('createRoom.name') }}</span>
        <input v-model.trim="name" class="input" type="text" required maxlength="64" />
      </label>
      <label class="field">
        <span class="field-label">{{ t('createRoom.password') }}</span>
        <input v-model="password" class="input" type="password" autocomplete="new-password"
               :placeholder="t('createRoom.passwordHint')" />
      </label>
      <p v-if="error" class="notice notice-error">{{ error }}</p>
      <div class="actions">
        <button class="button button-primary" type="submit" :disabled="busy">
          {{ busy ? t('createRoom.submitting') : t('createRoom.submit') }}
        </button>
        <button class="button" type="button" @click="emit('cancel')">{{ t('createRoom.cancel') }}</button>
      </div>
    </form>
  </section>
</template>
