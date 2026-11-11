<script setup>
import { ref } from 'vue'
import { join } from '../backend.js'
import { t, errorText } from '../i18n/index.js'

const emit = defineEmits(['cancel', 'joined'])

const code = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  busy.value = true
  error.value = ''
  try {
    const st = await join(code.value, password.value)
    password.value = ''
    emit('joined', st)
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel">
    <h2 class="panel-title">{{ t('joinRoom.title') }}</h2>
    <form class="form" @submit.prevent="submit">
      <label class="field">
        <span class="field-label">{{ t('room.code') }}</span>
        <input v-model.trim="code" class="input" type="text" required autocomplete="off"
               :placeholder="t('joinRoom.codePlaceholder')" />
      </label>
      <label class="field">
        <span class="field-label">{{ t('joinRoom.password') }}</span>
        <input v-model="password" class="input" type="password" autocomplete="off"
               :placeholder="t('joinRoom.passwordHint')" />
      </label>
      <p v-if="error" class="notice notice-error">{{ error }}</p>
      <div class="actions">
        <button class="button button-primary" type="submit" :disabled="busy">
          {{ busy ? t('joinRoom.submitting') : t('joinRoom.submit') }}
        </button>
        <button class="button" type="button" @click="emit('cancel')">{{ t('joinRoom.cancel') }}</button>
      </div>
    </form>
  </section>
</template>
