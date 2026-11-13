<script setup>
import { onMounted, ref } from 'vue'
import { changePassword, logout, setDeviceName, setUpdateSettings, updateSettings } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import { checkNow } from '../update.js'

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

// About: version and update checks.
const autoCheck = ref(true)
const ignoredVersion = ref('')
const checkBusy = ref(false)
const checkMessage = ref('')
const checkError = ref('')
const checked = ref(null)

onMounted(async () => {
  try {
    const s = await updateSettings()
    autoCheck.value = s.autoCheck
    ignoredVersion.value = s.ignoredVersion ?? ''
  } catch (err) {
    checkError.value = errorText(err)
  }
})

async function saveAutoCheck() {
  checkError.value = ''
  try {
    const s = await setUpdateSettings(autoCheck.value, ignoredVersion.value)
    autoCheck.value = s.autoCheck
  } catch (err) {
    autoCheck.value = !autoCheck.value
    checkError.value = errorText(err)
  }
}

async function runCheck() {
  checkBusy.value = true
  checkMessage.value = ''
  checkError.value = ''
  checked.value = null
  try {
    const info = await checkNow()
    checked.value = info
    if (!info.available) checkMessage.value = t('settings.upToDate')
    else if (info.ignored) checkMessage.value = t('settings.newVersionIgnored', { version: info.latestVersion })
    else checkMessage.value = t('settings.newVersion', { version: info.latestVersion })
  } catch (err) {
    checkError.value = errorText(err)
  } finally {
    checkBusy.value = false
  }
}

async function unignore() {
  checkError.value = ''
  try {
    await setUpdateSettings(autoCheck.value, '')
    ignoredVersion.value = ''
    await runCheck()
  } catch (err) {
    checkError.value = errorText(err)
  }
}

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

    <h2 class="section-title">{{ t('settings.aboutTitle') }}</h2>
    <div class="form">
      <p class="field-label">{{ t('settings.version', { version: status.version }) }}</p>
      <label class="checkbox">
        <input v-model="autoCheck" type="checkbox" @change="saveAutoCheck" />
        <span>{{ t('settings.autoCheck') }}</span>
      </label>
      <p v-if="checkError" class="notice notice-error">{{ checkError }}</p>
      <p v-if="checkMessage" class="notice notice-ok">{{ checkMessage }}</p>
      <div class="button-row">
        <button class="button" type="button" :disabled="checkBusy" @click="runCheck">
          {{ checkBusy ? t('settings.checking') : t('settings.checkNow') }}
        </button>
        <button v-if="checked?.available && checked?.ignored" class="button" type="button" @click="unignore">
          {{ t('settings.unignore') }}
        </button>
      </div>
    </div>

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
