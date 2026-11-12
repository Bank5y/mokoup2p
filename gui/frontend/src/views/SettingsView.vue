<script setup>
import { ref } from 'vue'
import { changePassword, logout, setDeviceName } from '../backend.js'
import { t, errorText } from '../i18n/index.js'

const props = defineProps({ status: { type: Object, required: true } })
// status: the daemon status changed (renamed, logged out, session lost).
const emit = defineEmits(['status'])

const deviceName = ref(props.status.deviceName)
const deviceBusy = ref(false)
const deviceMessage = ref('')
const deviceError = ref('')

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordBusy = ref(false)
const passwordMessage = ref('')
const passwordError = ref('')

const logoutBusy = ref(false)
const logoutError = ref('')

async function saveDeviceName() {
  deviceBusy.value = true
  deviceMessage.value = ''
  deviceError.value = ''
  try {
    const st = await setDeviceName(deviceName.value)
    deviceName.value = st.deviceName
    deviceMessage.value = t('settings.deviceNameSaved')
    emit('status', st)
  } catch (err) {
    deviceError.value = errorText(err)
  } finally {
    deviceBusy.value = false
  }
}

async function submitPassword() {
  passwordMessage.value = ''
  passwordError.value = ''
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = t('settings.passwordMismatch')
    return
  }
  passwordBusy.value = true
  try {
    await changePassword(oldPassword.value, newPassword.value)
    oldPassword.value = newPassword.value = confirmPassword.value = ''
    passwordMessage.value = t('settings.passwordChanged')
  } catch (err) {
    passwordError.value = String(err?.message ?? err) === 'invalid_credentials'
      ? t('settings.wrongOldPassword')
      : errorText(err)
  } finally {
    passwordBusy.value = false
  }
}

async function submitLogout() {
  logoutBusy.value = true
  logoutError.value = ''
  try {
    emit('status', await logout())
  } catch (err) {
    logoutError.value = errorText(err)
    logoutBusy.value = false
  }
}
</script>

<template>
  <section class="page">
    <h2 class="section-title">{{ t('settings.deviceTitle') }}</h2>
    <form class="form" @submit.prevent="saveDeviceName">
      <label class="field">
        <span class="field-label">{{ t('settings.deviceName') }}</span>
        <input v-model.trim="deviceName" class="input" type="text" required maxlength="64" />
        <span class="field-hint">{{ t('settings.deviceHint') }}</span>
      </label>
      <p v-if="deviceError" class="notice notice-error">{{ deviceError }}</p>
      <p v-if="deviceMessage" class="notice notice-ok">{{ deviceMessage }}</p>
      <div>
        <button class="button button-primary" type="submit" :disabled="deviceBusy">
          {{ deviceBusy ? t('settings.saving') : t('settings.saveDeviceName') }}
        </button>
      </div>
    </form>

    <h2 class="section-title">{{ t('settings.passwordTitle') }}</h2>
    <form class="form" @submit.prevent="submitPassword">
      <label class="field">
        <span class="field-label">{{ t('settings.oldPassword') }}</span>
        <input v-model="oldPassword" class="input" type="password" required autocomplete="current-password" />
      </label>
      <label class="field">
        <span class="field-label">{{ t('settings.newPassword') }}</span>
        <input v-model="newPassword" class="input" type="password" required autocomplete="new-password" />
      </label>
      <label class="field">
        <span class="field-label">{{ t('settings.confirmPassword') }}</span>
        <input v-model="confirmPassword" class="input" type="password" required autocomplete="new-password" />
      </label>
      <p v-if="passwordError" class="notice notice-error">{{ passwordError }}</p>
      <p v-if="passwordMessage" class="notice notice-ok">{{ passwordMessage }}</p>
      <div>
        <button class="button button-primary" type="submit" :disabled="passwordBusy">
          {{ passwordBusy ? t('settings.saving') : t('settings.changePassword') }}
        </button>
      </div>
    </form>

    <h2 class="section-title">{{ t('settings.accountTitle') }}</h2>
    <div class="form">
      <p class="field-label">{{ t('settings.loggedInAs', { username: status.username }) }}</p>
      <p v-if="logoutError" class="notice notice-error">{{ logoutError }}</p>
      <div>
        <button class="button button-danger" type="button" :disabled="logoutBusy" @click="submitLogout">
          {{ t('settings.logout') }}
        </button>
      </div>
    </div>
  </section>
</template>
