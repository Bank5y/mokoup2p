<script setup>
import { ref } from 'vue'
import { login, setServer } from '../backend.js'
import { t, errorText } from '../i18n/index.js'

const props = defineProps({ status: { type: Object, required: true } })
const emit = defineEmits(['logged-in'])

const serverUrl = ref(props.status.serverUrl || '')
const username = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await setServer(serverUrl.value)
    const st = await login(username.value, password.value)
    password.value = ''
    emit('logged-in', st)
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="login">
    <h1 class="login-title">{{ t('app.name') }}</h1>
    <form class="form" @submit.prevent="submit">
      <label class="field">
        <span class="field-label">{{ t('login.serverUrl') }}</span>
        <input v-model.trim="serverUrl" class="input" type="url" required
               :placeholder="t('login.serverUrlPlaceholder')" />
      </label>
      <label class="field">
        <span class="field-label">{{ t('login.username') }}</span>
        <input v-model.trim="username" class="input" type="text" required autocomplete="username" />
      </label>
      <label class="field">
        <span class="field-label">{{ t('login.password') }}</span>
        <input v-model="password" class="input" type="password" required autocomplete="current-password" />
      </label>
      <p v-if="error" class="notice notice-error">{{ error }}</p>
      <button class="button button-primary" type="submit" :disabled="busy">
        {{ busy ? t('login.submitting') : t('login.submit') }}
      </button>
      <p class="hint">{{ t('login.noAccountHint') }}</p>
    </form>
  </section>
</template>
